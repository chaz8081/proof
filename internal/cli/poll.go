// internal/cli/poll.go
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

func init() {
	var dryRun bool
	var model string
	var reReview bool
	var every string
	var includeOwn bool

	pollCmd := &cobra.Command{
		Use:   "poll [owner/repo#number]",
		Short: "Check for PRs needing review and generate AI draft reviews",
		Long:  `Poll for PRs requesting your review and generate AI reviews. Optionally specify a single PR to review directly.`,
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			// Single-PR mode: review just one PR directly, no search needed
			if len(args) > 0 {
				owner, repo, number, err := parsePRRef(args[0])
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

				cmd.Printf("Reviewing %s/%s#%d...\n", owner, repo, number)

				// Check for existing pending review
				existing, err := ghClient.ListPendingReviews(ctx, owner, repo, number)
				if err == nil && len(existing) > 0 && !reReview {
					cmd.Printf("  Skipping — pending review already exists (ID: %d)\n", existing[0].ID)
					return nil
				}
				if reReview && len(existing) > 0 {
					ghClient.DeletePendingReview(ctx, owner, repo, number, existing[0].ID)
					cmd.Printf("  Deleted existing pending review (ID: %d), re-reviewing...\n", existing[0].ID)
				}

				// Fetch context
				prCtx, err := ghClient.GetPRContext(ctx, owner, repo, number)
				if err != nil {
					return fmt.Errorf("fetching PR context: %w", err)
				}

				// Apply config if loaded
				if cfg != nil {
					prCtx.Instructions = cfg.Review.Instructions
					prCtx.Model = cfg.Review.Model
				}
				if model != "" {
					prCtx.Model = model
				}
				if prCtx.Model == "" {
					prCtx.Model = "gpt-4.1"
				}

				if dryRun {
					cmd.Printf("  %s — %s (by @%s)\n  (dry run — skipping AI review)\n", prCtx.Title, prCtx.Description, "")
					return nil
				}

				// Fetch repo instructions
				repoInstructions, err := ghClient.FetchRepoInstructions(ctx, owner, repo, prCtx.Files)
				if err == nil && repoInstructions != nil {
					prCtx.RepoInstructions = *repoInstructions
				}

				// Review
				copilotToken := resolveCopilotToken(cfg, token)
				reviewer, cleanup, err := review.NewReviewer(ctx, copilotToken)
				if err != nil {
					return fmt.Errorf("initializing reviewer: %w", err)
				}
				defer cleanup()

				result, err := reviewer.Review(ctx, *prCtx)
				if err != nil {
					return fmt.Errorf("AI review failed: %w", err)
				}

				reviewID, err := ghClient.CreatePendingReview(ctx, owner, repo, number, result, prCtx.Diff)
				if err != nil {
					return fmt.Errorf("creating review: %w", err)
				}

				pendingStore := proofstore.NewFileStore(filepath.Join(config.ConfigDir(), "pending.json"))
				pendingStore.Add(proofstore.PendingRecord{
					Owner: owner, Repo: repo, Number: number, ReviewID: reviewID, Created: time.Now(),
				})

				cmd.Printf("  Done — pending review created (ID: %d) — %d comments, verdict: %s\n",
					reviewID, len(result.Comments), result.Verdict)
				cmd.Printf("    View: https://github.com/%s/%s/pull/%d\n", owner, repo, number)
				return nil
			}

			// Multi-PR mode: search for PRs requesting review

			// Parse --every flag and set up watch mode
			var duration time.Duration
			var sigCh chan os.Signal
			if every != "" {
				var err error
				duration, err = time.ParseDuration(every)
				if err != nil {
					return fmt.Errorf("invalid --every value %q: %w (examples: 5m, 30m, 1h)", every, err)
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

				includeOwnResolved := includeOwn || cfg.Poll.IncludeOwn
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
					for _, pr := range prs {
						cmd.Printf("  • %s/%s#%d — %s (by @%s)\n", pr.Owner, pr.Repo, pr.Number, pr.Title, pr.Author)
					}

					if dryRun {
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
							cmd.Printf("Reviewing %s/%s#%d...\n", pr.Owner, pr.Repo, pr.Number)

							prCtx, err := ghClient.GetPRContext(ctx, pr.Owner, pr.Repo, pr.Number)
							if err != nil {
								cmd.PrintErrf("  Warning: Error fetching PR: %v\n", err)
								continue
							}

							if cfg.Poll.MaxFiles > 0 && len(prCtx.Files) > cfg.Poll.MaxFiles {
								cmd.Printf("  Skipping — %d files exceeds max_files (%d)\n", len(prCtx.Files), cfg.Poll.MaxFiles)
								continue
							}

							if cfg.Poll.MaxDiffBytes > 0 && len(prCtx.Diff) > cfg.Poll.MaxDiffBytes {
								cmd.Printf("  Skipping — diff size %d bytes exceeds max_diff_bytes (%d)\n",
									len(prCtx.Diff), cfg.Poll.MaxDiffBytes)
								continue
							}

							// Before creating, check if we already have a pending review
							existing, err := ghClient.ListPendingReviews(ctx, pr.Owner, pr.Repo, pr.Number)
							if err != nil {
								cmd.PrintErrf("  Warning: Error checking existing reviews: %v\n", err)
								continue
							}
							if len(existing) > 0 {
								if !reReview {
									cmd.Printf("  Skipping — pending review already exists (ID: %d)\n", existing[0].ID)
									continue
								}
								// Delete existing pending review before creating new one
								if err := ghClient.DeletePendingReview(ctx, pr.Owner, pr.Repo, pr.Number, existing[0].ID); err != nil {
									cmd.PrintErrf("  Warning: Failed to delete existing review: %v\n", err)
									continue
								}
								cmd.Printf("  Deleted existing pending review (ID: %d), re-reviewing...\n", existing[0].ID)
							}

							reviewModel := cfg.Review.Model
							if model != "" {
								reviewModel = model
							}

							repoInstructions, err := ghClient.FetchRepoInstructions(ctx, pr.Owner, pr.Repo, prCtx.Files)
							if err != nil {
								cmd.PrintErrf("  Warning: Failed to fetch repo instructions: %v\n", err)
							}
							if repoInstructions != nil {
								prCtx.RepoInstructions = *repoInstructions
							}

							prCtx.Instructions = cfg.Review.Instructions
							prCtx.Model = reviewModel
							result, err := reviewer.Review(ctx, *prCtx)
							if err != nil {
								cmd.PrintErrf("  Warning: Error during AI review: %v\n", err)
								continue
							}

							reviewID, err := ghClient.CreatePendingReview(ctx, pr.Owner, pr.Repo, pr.Number, result, prCtx.Diff)
							if err != nil {
								cmd.PrintErrf("  Warning: Error creating review: %v\n", err)
								continue
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
						}
					}
				}

				if every == "" {
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
		},
	}

	pollCmd.Flags().BoolVar(&dryRun, "dry-run", false, "List PRs without generating reviews")
	pollCmd.Flags().StringVar(&model, "model", "", "AI model to use (overrides config)")
	pollCmd.Flags().BoolVar(&reReview, "re-review", false, "Force re-review of PRs with existing pending reviews")
	pollCmd.Flags().StringVar(&every, "every", "", "Poll repeatedly at this interval (e.g., 5m, 1h)")
	pollCmd.Flags().BoolVar(&includeOwn, "include-own", false, "Include your own PRs in the review scan")
	rootCmd.AddCommand(pollCmd)
}
