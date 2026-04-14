// internal/cli/poll_multi.go
package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	"github.com/chaz8081/proof/internal/config"
	proofgh "github.com/chaz8081/proof/internal/github"
	"github.com/chaz8081/proof/internal/review"
	proofstore "github.com/chaz8081/proof/internal/store"
	"github.com/spf13/cobra"
)

// pollResultItem is the JSON output shape for a completed review.
type pollResultItem struct {
	Owner        string `json:"owner"`
	Repo         string `json:"repo"`
	Number       int    `json:"number"`
	ReviewID     int64  `json:"review_id"`
	Verdict      string `json:"verdict"`
	CommentCount int    `json:"comment_count"`
	Summary      string `json:"summary"`
}

// dryRunResultItem is the JSON output shape for a dry-run PR listing.
type dryRunResultItem struct {
	Owner  string `json:"owner"`
	Repo   string `json:"repo"`
	Number int    `json:"number"`
	Title  string `json:"title"`
	Author string `json:"author"`
	Status string `json:"status"`
}

// pollMultiplePRs handles the multi-PR flow: search, interactive selection, batch, and watch modes.
func pollMultiplePRs(cmd *cobra.Command, opts pollOptions) error {
	ctx := cmd.Context()

	// Parse --every flag and set up watch mode
	var duration time.Duration
	var sigCh chan os.Signal
	if opts.Every != "" {
		var err error
		duration, err = time.ParseDuration(opts.Every)
		if err != nil {
			return fmt.Errorf("invalid --every value %q: %w (examples: 5m, 30m, 1h)", opts.Every, err)
		}
		if duration < 1*time.Minute {
			return fmt.Errorf("--every interval must be at least 1 minute")
		}
		cmd.Printf("Watching for review requests every %s (Ctrl+C to stop)\n\n", duration)

		sigCh = make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt)
		defer signal.Stop(sigCh)
	}

	for {
		pendingStore := proofstore.NewFileStore(filepath.Join(config.ConfigDir(), "pending.json"))

		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("No config found at ~/.proof/config.yaml\nRun 'proof setup' to create one, then edit it to add your repos.")
		}

		if len(cfg.Repos) == 0 {
			return fmt.Errorf("No repos configured. Edit ~/.proof/config.yaml and add repos to watch.\nExample:\n  repos:\n    - owner/repo\n    - myorg/*")
		}

		token, err := resolveGitHubToken(cfg)
		if err != nil {
			return err
		}

		ghClient := proofgh.NewClient(token)

		// Check rate limit before starting
		rateInfo, err := ghClient.CheckRateLimit(ctx)
		if err == nil && rateInfo.Remaining < 10 {
			cmd.Printf("Warning: GitHub API rate limit low (%d/%d remaining, resets %s)\n",
				rateInfo.Remaining, rateInfo.Limit, rateInfo.Reset.Format("15:04:05"))
			if rateInfo.Remaining == 0 {
				waitTime := time.Until(rateInfo.Reset)
				if waitTime > 0 {
					cmd.Printf("Rate limited. Waiting %s for reset...\n", waitTime.Round(time.Second))
					time.Sleep(waitTime)
				}
			}
		}

		includeOwnResolved := (opts.IncludeOwn || cfg.Poll.IncludeOwn) && !opts.ExcludeOwn
		prs, err := ghClient.FindReviewRequests(ctx, cfg.RepoNames(),
			proofgh.WithIgnoreDrafts(*cfg.Poll.IgnoreDrafts),
			proofgh.WithIgnoreWIP(cfg.Poll.IgnoreWIP),
			proofgh.WithTeams(cfg.Teams),
			proofgh.WithIncludeOwn(includeOwnResolved),
		)
		if err != nil {
			return fmt.Errorf("finding review requests: %w", err)
		}

		if len(prs) == 0 {
			if opts.Output == "json" {
				cmd.Println("[]")
			} else {
				cmd.Println("No PRs waiting for your review.")
			}
		} else {
			if opts.Output != "json" {
				cmd.Printf("Found %d PR(s) requesting your review:\n\n", len(prs))
			}

			// Interactive selection: default when not in batch/watch/dry-run/json mode
			if !opts.Batch && opts.Every == "" && !opts.DryRun && opts.Output != "json" && isInteractive() {
				items := make([]prDisplayItem, len(prs))
				for i, pr := range prs {
					status := "NEW"
					var revID int64
					existing, err := ghClient.ListPendingReviews(ctx, pr.Owner, pr.Repo, pr.Number)
					if err == nil && len(existing) > 0 {
						status = "PENDING"
						revID = existing[0].ID
					}
					items[i] = prDisplayItem{
						Index:    i + 1,
						Owner:    pr.Owner,
						Repo:     pr.Repo,
						Number:   pr.Number,
						Title:    pr.Title,
						Author:   pr.Author,
						Status:   status,
						ReviewID: revID,
					}
				}

				displayPRList(cmd.OutOrStdout(), items)

				selected, err := promptSelection(os.Stdin, cmd.OutOrStdout(), len(items))
				if err != nil {
					return fmt.Errorf("selection error: %w", err)
				}

				// Filter PRs based on selection
				if selected == nil {
					// Default: new only
					var filtered []proofgh.PRInfo
					for _, item := range items {
						if item.Status == "NEW" {
							filtered = append(filtered, prs[item.Index-1])
						}
					}
					prs = filtered
				} else {
					var filtered []proofgh.PRInfo
					for _, idx := range selected {
						filtered = append(filtered, prs[idx-1])
					}
					prs = filtered
				}

				if len(prs) == 0 {
					cmd.Println("No PRs selected for review.")
					if opts.Every == "" {
						return nil
					}
				} else {
					cmd.Println() // blank line before review output
				}
			} else if opts.Output != "json" {
				for _, pr := range prs {
					cmd.Printf("  • %s/%s#%d — %s (by @%s)\n", pr.Owner, pr.Repo, pr.Number, pr.Title, pr.Author)
				}
			}

			if opts.DryRun {
				if opts.Output == "json" {
					dryItems := make([]dryRunResultItem, len(prs))
					for i, pr := range prs {
						status := "NEW"
						existing, err := ghClient.ListPendingReviews(ctx, pr.Owner, pr.Repo, pr.Number)
						if err == nil && len(existing) > 0 {
							status = "PENDING"
						}
						dryItems[i] = dryRunResultItem{
							Owner:  pr.Owner,
							Repo:   pr.Repo,
							Number: pr.Number,
							Title:  pr.Title,
							Author: pr.Author,
							Status: status,
						}
					}
					data, err := json.Marshal(dryItems)
					if err != nil {
						return fmt.Errorf("marshaling JSON: %w", err)
					}
					cmd.Println(string(data))
				} else {
					cmd.Println("\n(dry run — skipping AI review)")
				}
			} else {
				copilotToken := resolveCopilotToken(cfg, token)
				reviewer, cleanup, err := review.NewReviewer(ctx, copilotToken)
				if err != nil {
					return fmt.Errorf("initializing reviewer: %w", err)
				}
				defer cleanup()

				if opts.Output != "json" {
					cmd.Println()
				}
				var results []pollResultItem
				for _, pr := range prs {
					item, err := reviewPR(cmd, ghClient, pr, opts, cfg, reviewer, pendingStore)
					if err != nil {
						cmd.PrintErrf("  Warning: %v\n", err)
					} else if item != nil {
						results = append(results, *item)
					}
				}
				if opts.Output == "json" {
					if results == nil {
						results = []pollResultItem{}
					}
					data, err := json.Marshal(results)
					if err != nil {
						return fmt.Errorf("marshaling JSON: %w", err)
					}
					cmd.Println(string(data))
				}
			}
		}

		if opts.Every == "" {
			return nil // single run, exit
		}

		// Watch mode: wait for next tick or signal
		select {
		case <-time.After(duration):
			cmd.Println()
			continue
		case <-sigCh:
			cmd.Println("\nStopped watching.")
			return nil
		case <-ctx.Done():
			return nil
		}
	}
}

// reviewPR is the shared core: fetch PR context → check existing review → run AI review → create pending → store.
// Returns a pollResultItem when the review is successfully created, or nil if skipped.
func reviewPR(
	cmd *cobra.Command,
	ghClient *proofgh.Client,
	pr proofgh.PRInfo,
	opts pollOptions,
	cfg *config.Config,
	reviewer review.Reviewer,
	pendingStore *proofstore.FileStore,
) (*pollResultItem, error) {
	ctx := cmd.Context()

	if opts.Output != "json" {
		cmd.Printf("Reviewing %s/%s#%d...\n", pr.Owner, pr.Repo, pr.Number)
	}

	prCtx, err := ghClient.GetPRContext(ctx, pr.Owner, pr.Repo, pr.Number)
	if err != nil {
		return nil, fmt.Errorf("error fetching PR: %v", err)
	}

	// Load .proofignore patterns
	var ignorePatterns []string
	// Global ignore
	globalIgnorePath := filepath.Join(config.ConfigDir(), ".proofignore")
	if data, rerr := os.ReadFile(globalIgnorePath); rerr == nil {
		ignorePatterns = append(ignorePatterns, review.ParseIgnorePatterns(string(data))...)
	}
	// Repo-level ignore (fetch via GitHub API)
	if repoIgnore, ferr := ghClient.FetchFileContent(ctx, pr.Owner, pr.Repo, ".proofignore"); ferr == nil {
		ignorePatterns = append(ignorePatterns, review.ParseIgnorePatterns(repoIgnore)...)
	}
	// Apply filtering
	if len(ignorePatterns) > 0 {
		originalCount := len(prCtx.Files)
		prCtx.Files = review.FilterFiles(prCtx.Files, ignorePatterns)
		prCtx.Diff = review.FilterDiff(prCtx.Diff, ignorePatterns)
		if filtered := originalCount - len(prCtx.Files); filtered > 0 {
			cmd.Printf("  Filtered %d file(s) via .proofignore\n", filtered)
		}
	}

	if cfg.Poll.MaxFiles > 0 && len(prCtx.Files) > cfg.Poll.MaxFiles {
		if opts.Output != "json" {
			cmd.Printf("  Skipping — %d files exceeds max_files (%d)\n", len(prCtx.Files), cfg.Poll.MaxFiles)
		}
		return nil, nil
	}

	if cfg.Poll.MaxDiffBytes > 0 && len(prCtx.Diff) > cfg.Poll.MaxDiffBytes {
		if opts.Output != "json" {
			cmd.Printf("  Skipping — diff size %d bytes exceeds max_diff_bytes (%d)\n",
				len(prCtx.Diff), cfg.Poll.MaxDiffBytes)
		}
		return nil, nil
	}

	// Before creating, check if we already have a pending review
	existing, err := ghClient.ListPendingReviews(ctx, pr.Owner, pr.Repo, pr.Number)
	if err != nil {
		return nil, fmt.Errorf("error checking existing reviews: %v", err)
	}

	// Look up the existing store record (needed for re-review diff and summary context).
	var existingRecord *proofstore.PendingRecord
	if storeRecords, lerr := pendingStore.List(); lerr == nil {
		for i := range storeRecords {
			r := &storeRecords[i]
			if r.Owner == pr.Owner && r.Repo == pr.Repo && r.Number == pr.Number {
				existingRecord = r
				break
			}
		}
	}

	if len(existing) > 0 {
		if !opts.ReReview {
			if opts.Output != "json" {
				cmd.Printf("  Skipping — pending review already exists (ID: %d)\n", existing[0].ID)
			}
			return nil, nil
		}
		// Delete existing pending review before creating new one
		if err := ghClient.DeletePendingReview(ctx, pr.Owner, pr.Repo, pr.Number, existing[0].ID); err != nil {
			return nil, fmt.Errorf("failed to delete existing review: %v", err)
		}
		if opts.Output != "json" {
			cmd.Printf("  Deleted existing pending review (ID: %d), re-reviewing...\n", existing[0].ID)
		}
	}

	reviewModel := cfg.Review.Model
	if opts.Model != "" {
		reviewModel = opts.Model
	}

	repoInstructions, err := ghClient.FetchRepoInstructions(ctx, pr.Owner, pr.Repo, prCtx.Files)
	if err != nil {
		cmd.PrintErrf("  Warning: Failed to fetch repo instructions: %v\n", err)
	}
	if repoInstructions != nil {
		prCtx.RepoInstructions = *repoInstructions
	}

	// Fetch repo-level .proof.yaml
	repoCfg, err := ghClient.FetchRepoConfig(ctx, pr.Owner, pr.Repo)
	if err != nil {
		cmd.PrintErrf("  Warning: Failed to parse repo .proof.yaml: %v\n", err)
	}

	// Resolve instructions: per-repo override > global config
	instructions := cfg.Review.Instructions
	if repoInstr := cfg.RepoInstructions(pr.Owner, pr.Repo); repoInstr != "" {
		instructions = repoInstr
	}
	prCtx.Instructions = instructions
	prCtx.Model = reviewModel

	// Merge repo config: repo config provides defaults where user hasn't configured
	if repoCfg != nil {
		if prCtx.Instructions == "" && repoCfg.Instructions != "" {
			prCtx.Instructions = repoCfg.Instructions
		}
		if prCtx.Model == "" && repoCfg.Model != "" {
			prCtx.Model = repoCfg.Model
		}
	}

	// Apply review profile if specified
	if opts.Profile != "" {
		p := review.ResolveProfile(opts.Profile, cfg)
		if p == nil {
			return nil, fmt.Errorf("unknown profile %q — available: quick, thorough", opts.Profile)
		}
		if p.Instructions != "" {
			prCtx.Instructions += "\n\n" + p.Instructions
		}
	}

	// Diff-aware re-review: when re-reviewing and we have a stored head SHA,
	// fetch only the incremental diff since the last review.
	if opts.ReReview && existingRecord != nil && existingRecord.HeadSHA != "" && prCtx.HeadSHA != "" {
		incrementalDiff, diffErr := ghClient.GetCommitDiff(ctx, pr.Owner, pr.Repo, existingRecord.HeadSHA, prCtx.HeadSHA)
		if diffErr == nil && incrementalDiff != "" {
			prCtx.Diff = incrementalDiff
			prevSummary := ""
			if existingRecord.OriginalResult != nil {
				prevSummary = existingRecord.OriginalResult.Summary
			}
			prCtx.Instructions += "\n\nThis is a RE-REVIEW. You previously reviewed this PR. Focus only on the changes since your last review. Previous summary: " + prevSummary
			if opts.Output != "json" {
				cmd.Printf("  Re-reviewing incremental diff (%s..%s)\n", shortSHA(existingRecord.HeadSHA), shortSHA(prCtx.HeadSHA))
			}
		}
		// If incremental diff fails or is empty, fall through to full diff
	}

	var spin *spinner
	if opts.Output != "json" {
		spin = newSpinner(cmd.OutOrStdout(), "AI reviewing...")
	}
	start := time.Now()
	result, err := reviewer.Review(ctx, *prCtx)
	duration := time.Since(start)
	if spin != nil {
		spin.stop()
	}
	if err != nil {
		if strings.Contains(err.Error(), "not available") {
			return nil, fmt.Errorf("error during AI review: %v\nRun 'proof config models' to see available models", err)
		}
		return nil, fmt.Errorf("error during AI review: %v", err)
	}

	reviewID, err := ghClient.CreatePendingReview(ctx, pr.Owner, pr.Repo, pr.Number, result, prCtx.Diff, prCtx.Model)
	if err != nil {
		return nil, fmt.Errorf("error creating review: %v", err)
	}

	// Record review in history
	historyStore := proofstore.NewHistoryStore(filepath.Join(config.ConfigDir(), "reviews.jsonl"))
	_ = historyStore.Append(proofstore.ReviewRecord{
		Timestamp:       time.Now(),
		Owner:           pr.Owner,
		Repo:            pr.Repo,
		Number:          pr.Number,
		Title:           prCtx.Title,
		Author:          pr.Author,
		Verdict:         result.Verdict,
		CommentCount:    len(result.Comments),
		FileCount:       len(prCtx.Files),
		DiffBytes:       len(prCtx.Diff),
		Model:           prCtx.Model,
		ReviewID:        reviewID,
		Duration:        duration.Seconds(),
		InputTokens:     result.Usage.InputTokens,
		OutputTokens:    result.Usage.OutputTokens,
		CacheReadTokens: result.Usage.CacheReadTokens,
		PremiumRequests: result.Usage.PremiumRequests,
	})

	if err := pendingStore.Add(proofstore.PendingRecord{
		Owner:    pr.Owner,
		Repo:     pr.Repo,
		Number:   pr.Number,
		ReviewID: reviewID,
		Created:  time.Now(),
		HeadSHA:  prCtx.HeadSHA,
		OriginalResult: &proofstore.OriginalReview{
			Summary:      result.Summary,
			Verdict:      result.Verdict,
			CommentCount: len(result.Comments),
			CommentPaths: extractPaths(result.Comments),
		},
	}); err != nil {
		cmd.PrintErrf("  Warning: Failed to record pending review locally: %v\n", err)
	}

	if opts.Output != "json" {
		cmd.Printf("  Done — pending review created (ID: %d) — %d comments, verdict: %s\n",
			reviewID, len(result.Comments), result.Verdict)
		cmd.Printf("    View: https://github.com/%s/%s/pull/%d\n", pr.Owner, pr.Repo, pr.Number)
	}

	return &pollResultItem{
		Owner:        pr.Owner,
		Repo:         pr.Repo,
		Number:       pr.Number,
		ReviewID:     reviewID,
		Verdict:      result.Verdict,
		CommentCount: len(result.Comments),
		Summary:      result.Summary,
	}, nil
}
