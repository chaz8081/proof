// internal/cli/poll.go
package cli

import (
	"fmt"
	"os"

	"github.com/chaz8081/proof/internal/config"
	proofgh "github.com/chaz8081/proof/internal/github"
	"github.com/chaz8081/proof/internal/review"
	"github.com/spf13/cobra"
)

func init() {
	var dryRun bool

	pollCmd := &cobra.Command{
		Use:   "poll",
		Short: "Check for PRs needing review and generate AI draft reviews",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("loading config: %w\nRun 'proof config init' to create one", err)
			}

			token := os.Getenv("GITHUB_TOKEN")
			if token == "" {
				return fmt.Errorf("GITHUB_TOKEN not set — export your GitHub token or use 'gh auth token'")
			}

			ghClient := proofgh.NewClient(token)

			prs, err := ghClient.FindReviewRequests(ctx, cfg.Repos,
				proofgh.WithIgnoreDrafts(*cfg.Poll.IgnoreDrafts),
				proofgh.WithIgnoreWIP(cfg.Poll.IgnoreWIP),
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

				result, err := reviewer.Review(ctx, *prCtx)
				if err != nil {
					cmd.PrintErrf("  Warning: Error during AI review: %v\n", err)
					continue
				}

				reviewID, err := ghClient.CreatePendingReview(ctx, pr.Owner, pr.Repo, pr.Number, result)
				if err != nil {
					cmd.PrintErrf("  Warning: Error creating review: %v\n", err)
					continue
				}

				cmd.Printf("  Done — pending review created (ID: %d) — %d comments, verdict: %s\n",
					reviewID, len(result.Comments), result.Verdict)
				cmd.Printf("    View: https://github.com/%s/%s/pull/%d\n", pr.Owner, pr.Repo, pr.Number)
			}

			return nil
		},
	}

	pollCmd.Flags().BoolVar(&dryRun, "dry-run", false, "List PRs without generating reviews")
	rootCmd.AddCommand(pollCmd)
}
