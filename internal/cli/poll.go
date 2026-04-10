// internal/cli/poll.go
package cli

import (
	"fmt"
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

	pollCmd := &cobra.Command{
		Use:   "poll",
		Short: "Check for PRs needing review and generate AI draft reviews",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			pendingStore := proofstore.NewFileStore(filepath.Join(config.ConfigDir(), "pending.json"))

			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("No config found at ~/.proof/config.yaml\nRun 'proof config init' to create one, then edit it to add your repos.")
			}

			if len(cfg.Repos) == 0 {
				return fmt.Errorf("No repos configured. Edit ~/.proof/config.yaml and add repos to watch.\nExample:\n  repos:\n    - owner/repo\n    - myorg/*")
			}

			token, err := resolveToken()
			if err != nil {
				return err
			}

			ghClient := proofgh.NewClient(token)

			prs, err := ghClient.FindReviewRequests(ctx, cfg.Repos,
				proofgh.WithIgnoreDrafts(*cfg.Poll.IgnoreDrafts),
				proofgh.WithIgnoreWIP(cfg.Poll.IgnoreWIP),
				proofgh.WithTeams(cfg.Teams),
			)
			if err != nil {
				return fmt.Errorf("finding review requests: %w", err)
			}

			if len(prs) == 0 {
				cmd.Println("No PRs waiting for your review.")
				return nil
			}

			cmd.Printf("Found %d PR(s) requesting your review:\n\n", len(prs))
			for _, pr := range prs {
				cmd.Printf("  • %s/%s#%d — %s (by @%s)\n", pr.Owner, pr.Repo, pr.Number, pr.Title, pr.Author)
			}

			if dryRun {
				cmd.Println("\n(dry run — skipping AI review)")
				return nil
			}

			reviewer, cleanup, err := review.NewReviewer(ctx)
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
					cmd.Printf("  Skipping — pending review already exists (ID: %d)\n", existing[0].ID)
					continue
				}

				reviewModel := cfg.Review.Model
				if model != "" {
					reviewModel = model
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

			return nil
		},
	}

	pollCmd.Flags().BoolVar(&dryRun, "dry-run", false, "List PRs without generating reviews")
	pollCmd.Flags().StringVar(&model, "model", "", "AI model to use (overrides config)")
	rootCmd.AddCommand(pollCmd)
}
