// internal/cli/poll_single.go
package cli

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"time"

	"github.com/chaz8081/proof/internal/config"
	proofgh "github.com/chaz8081/proof/internal/github"
	"github.com/chaz8081/proof/internal/review"
	proofstore "github.com/chaz8081/proof/internal/store"
	"github.com/spf13/cobra"
)

// pollSinglePR handles the single-PR review flow when the user provides a PR ref argument.
func pollSinglePR(cmd *cobra.Command, prRef string, opts pollOptions) error {
	ctx := cmd.Context()

	owner, repo, number, err := parsePRRef(prRef)
	if err != nil {
		return err
	}

	// Load config for instructions/model (but don't need repos list)
	cfg, _ := config.Load() // OK if no config for single-PR mode

	token, err := resolveGitHubToken(cfg)
	if err != nil {
		return err
	}
	ghClient := proofgh.NewClient(token)

	if opts.Output != "json" {
		cmd.Printf("Reviewing %s/%s#%d...\n", owner, repo, number)
	}

	// Check for existing pending review
	existing, err := ghClient.ListPendingReviews(ctx, owner, repo, number)
	if err == nil && len(existing) > 0 && !opts.ReReview {
		if opts.Output != "json" {
			cmd.Printf("  Skipping — pending review already exists (ID: %d)\n", existing[0].ID)
		}
		return nil
	}

	// Look up existing store record before deleting (needed for incremental diff context).
	pendingStore := proofstore.NewFileStore(filepath.Join(config.ConfigDir(), "pending.json"))
	var existingRecord *proofstore.PendingRecord
	if storeRecords, lerr := pendingStore.List(); lerr == nil {
		for i := range storeRecords {
			r := &storeRecords[i]
			if r.Owner == owner && r.Repo == repo && r.Number == number {
				existingRecord = r
				break
			}
		}
	}

	if opts.ReReview && len(existing) > 0 {
		ghClient.DeletePendingReview(ctx, owner, repo, number, existing[0].ID)
		if opts.Output != "json" {
			cmd.Printf("  Deleted existing pending review (ID: %d), re-reviewing...\n", existing[0].ID)
		}
	}

	// Fetch context
	prCtx, err := ghClient.GetPRContext(ctx, owner, repo, number)
	if err != nil {
		return fmt.Errorf("fetching PR context: %w", err)
	}

	// Apply config if loaded
	if cfg != nil {
		// Resolve instructions: per-repo override > global config
		instructions := cfg.Review.Instructions
		if repoInstr := cfg.RepoInstructions(owner, repo); repoInstr != "" {
			instructions = repoInstr
		}
		prCtx.Instructions = instructions
		prCtx.Model = cfg.Review.Model
	}
	if opts.Model != "" {
		prCtx.Model = opts.Model
	}
	if prCtx.Model == "" {
		prCtx.Model = "gpt-4.1"
	}

	if opts.DryRun {
		if opts.Output == "json" {
			items := []dryRunResultItem{{
				Owner:  owner,
				Repo:   repo,
				Number: number,
				Title:  prCtx.Title,
				Author: "",
				Status: "NEW",
			}}
			data, err := json.Marshal(items)
			if err != nil {
				return fmt.Errorf("marshaling JSON: %w", err)
			}
			cmd.Println(string(data))
		} else {
			cmd.Printf("  %s — %s\n  (dry run — skipping AI review)\n", prCtx.Title, prCtx.Description)
		}
		return nil
	}

	// Fetch repo instructions
	repoInstructions, err := ghClient.FetchRepoInstructions(ctx, owner, repo, prCtx.Files)
	if err == nil && repoInstructions != nil {
		prCtx.RepoInstructions = *repoInstructions
	}

	// Fetch repo-level .proof.yaml
	repoCfg, err := ghClient.FetchRepoConfig(ctx, owner, repo)
	if err != nil {
		cmd.PrintErrf("  Warning: Failed to parse repo .proof.yaml: %v\n", err)
	}

	// Merge repo config: repo config provides defaults where user hasn't configured
	if repoCfg != nil {
		if prCtx.Instructions == "" && repoCfg.Instructions != "" {
			prCtx.Instructions = repoCfg.Instructions
		}
		if prCtx.Model == "" && repoCfg.Model != "" {
			prCtx.Model = repoCfg.Model
		}
	}

	// Diff-aware re-review: when re-reviewing and we have a stored head SHA,
	// fetch only the incremental diff since the last review.
	if opts.ReReview && existingRecord != nil && existingRecord.HeadSHA != "" && prCtx.HeadSHA != "" {
		incrementalDiff, diffErr := ghClient.GetCommitDiff(ctx, owner, repo, existingRecord.HeadSHA, prCtx.HeadSHA)
		if diffErr == nil && incrementalDiff != "" {
			prCtx.Diff = incrementalDiff
			prevSummary := ""
			if existingRecord.OriginalResult != nil {
				prevSummary = existingRecord.OriginalResult.Summary
			}
			prCtx.Instructions += "\n\nThis is a RE-REVIEW. You previously reviewed this PR. Focus only on the changes since your last review. Previous summary: " + prevSummary
			if opts.Output != "json" {
				cmd.Printf("  Re-reviewing incremental diff (%s..%s)\n", existingRecord.HeadSHA[:7], prCtx.HeadSHA[:7])
			}
		}
		// If incremental diff fails or is empty, fall through to full diff
	}

	// Review
	copilotToken := resolveCopilotToken(cfg, token)
	reviewer, cleanup, err := review.NewReviewer(ctx, copilotToken)
	if err != nil {
		return fmt.Errorf("initializing reviewer: %w", err)
	}
	defer cleanup()

	var spin *spinner
	if opts.Output != "json" {
		spin = newSpinner(cmd.OutOrStdout(), "AI reviewing...")
	}
	result, err := reviewer.Review(ctx, *prCtx)
	if spin != nil {
		spin.stop()
	}
	if err != nil {
		return fmt.Errorf("AI review failed: %w", err)
	}

	reviewID, err := ghClient.CreatePendingReview(ctx, owner, repo, number, result, prCtx.Diff)
	if err != nil {
		return fmt.Errorf("creating review: %w", err)
	}

	pendingStore.Add(proofstore.PendingRecord{
		Owner:    owner,
		Repo:     repo,
		Number:   number,
		ReviewID: reviewID,
		Created:  time.Now(),
		HeadSHA:  prCtx.HeadSHA,
		OriginalResult: &proofstore.OriginalReview{
			Summary:      result.Summary,
			Verdict:      result.Verdict,
			CommentCount: len(result.Comments),
			CommentPaths: extractPaths(result.Comments),
		},
	})

	if opts.Output == "json" {
		items := []pollResultItem{{
			Owner:        owner,
			Repo:         repo,
			Number:       number,
			ReviewID:     reviewID,
			Verdict:      result.Verdict,
			CommentCount: len(result.Comments),
			Summary:      result.Summary,
		}}
		data, err := json.Marshal(items)
		if err != nil {
			return fmt.Errorf("marshaling JSON: %w", err)
		}
		cmd.Println(string(data))
	} else {
		cmd.Printf("  Done — pending review created (ID: %d) — %d comments, verdict: %s\n",
			reviewID, len(result.Comments), result.Verdict)
		cmd.Printf("    View: https://github.com/%s/%s/pull/%d\n", owner, repo, number)
	}
	return nil
}
