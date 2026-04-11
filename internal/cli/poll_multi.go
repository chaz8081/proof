// internal/cli/poll_multi.go
package cli

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"time"

	"github.com/chaz8081/proof/internal/config"
	proofgh "github.com/chaz8081/proof/internal/github"
	"github.com/chaz8081/proof/internal/review"
	proofstore "github.com/chaz8081/proof/internal/store"
	"github.com/spf13/cobra"
)

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
			return fmt.Errorf("No config found at ~/.proof/config.yaml\nRun 'proof config init' to create one, then edit it to add your repos.")
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

		includeOwnResolved := opts.IncludeOwn || cfg.Poll.IncludeOwn
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
			cmd.Println("No PRs waiting for your review.")
		} else {
			cmd.Printf("Found %d PR(s) requesting your review:\n\n", len(prs))

			// Interactive selection: default when not in batch/watch/dry-run mode
			if !opts.Batch && opts.Every == "" && !opts.DryRun && isInteractive() {
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
			} else {
				for _, pr := range prs {
					cmd.Printf("  • %s/%s#%d — %s (by @%s)\n", pr.Owner, pr.Repo, pr.Number, pr.Title, pr.Author)
				}
			}

			if opts.DryRun {
				cmd.Println("\n(dry run — skipping AI review)")
			} else {
				copilotToken := resolveCopilotToken(cfg, token)
				reviewer, cleanup, err := review.NewReviewer(ctx, copilotToken)
				if err != nil {
					return fmt.Errorf("initializing reviewer: %w", err)
				}
				defer cleanup()

				cmd.Println()
				for _, pr := range prs {
					if err := reviewPR(cmd, ghClient, pr, opts, cfg, reviewer, pendingStore); err != nil {
						cmd.PrintErrf("  Warning: %v\n", err)
					}
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
func reviewPR(
	cmd *cobra.Command,
	ghClient *proofgh.Client,
	pr proofgh.PRInfo,
	opts pollOptions,
	cfg *config.Config,
	reviewer review.Reviewer,
	pendingStore *proofstore.FileStore,
) error {
	ctx := cmd.Context()

	cmd.Printf("Reviewing %s/%s#%d...\n", pr.Owner, pr.Repo, pr.Number)

	prCtx, err := ghClient.GetPRContext(ctx, pr.Owner, pr.Repo, pr.Number)
	if err != nil {
		return fmt.Errorf("error fetching PR: %v", err)
	}

	if cfg.Poll.MaxFiles > 0 && len(prCtx.Files) > cfg.Poll.MaxFiles {
		cmd.Printf("  Skipping — %d files exceeds max_files (%d)\n", len(prCtx.Files), cfg.Poll.MaxFiles)
		return nil
	}

	if cfg.Poll.MaxDiffBytes > 0 && len(prCtx.Diff) > cfg.Poll.MaxDiffBytes {
		cmd.Printf("  Skipping — diff size %d bytes exceeds max_diff_bytes (%d)\n",
			len(prCtx.Diff), cfg.Poll.MaxDiffBytes)
		return nil
	}

	// Before creating, check if we already have a pending review
	existing, err := ghClient.ListPendingReviews(ctx, pr.Owner, pr.Repo, pr.Number)
	if err != nil {
		return fmt.Errorf("error checking existing reviews: %v", err)
	}
	if len(existing) > 0 {
		if !opts.ReReview {
			cmd.Printf("  Skipping — pending review already exists (ID: %d)\n", existing[0].ID)
			return nil
		}
		// Delete existing pending review before creating new one
		if err := ghClient.DeletePendingReview(ctx, pr.Owner, pr.Repo, pr.Number, existing[0].ID); err != nil {
			return fmt.Errorf("failed to delete existing review: %v", err)
		}
		cmd.Printf("  Deleted existing pending review (ID: %d), re-reviewing...\n", existing[0].ID)
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

	// Resolve instructions: per-repo override > global config
	instructions := cfg.Review.Instructions
	if repoInstr := cfg.RepoInstructions(pr.Owner, pr.Repo); repoInstr != "" {
		instructions = repoInstr
	}
	prCtx.Instructions = instructions
	prCtx.Model = reviewModel

	result, err := reviewer.Review(ctx, *prCtx)
	if err != nil {
		return fmt.Errorf("error during AI review: %v", err)
	}

	reviewID, err := ghClient.CreatePendingReview(ctx, pr.Owner, pr.Repo, pr.Number, result, prCtx.Diff)
	if err != nil {
		return fmt.Errorf("error creating review: %v", err)
	}

	if err := pendingStore.Add(proofstore.PendingRecord{
		Owner:    pr.Owner,
		Repo:     pr.Repo,
		Number:   pr.Number,
		ReviewID: reviewID,
		Created:  time.Now(),
	}); err != nil {
		cmd.PrintErrf("  Warning: Failed to record pending review locally: %v\n", err)
	}

	cmd.Printf("  Done — pending review created (ID: %d) — %d comments, verdict: %s\n",
		reviewID, len(result.Comments), result.Verdict)
	cmd.Printf("    View: https://github.com/%s/%s/pull/%d\n", pr.Owner, pr.Repo, pr.Number)
	return nil
}
