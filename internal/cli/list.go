// internal/cli/list.go
package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/chaz8081/proof/internal/config"
	proofgh "github.com/chaz8081/proof/internal/github"
	proofstore "github.com/chaz8081/proof/internal/store"
	"github.com/spf13/cobra"
)

func init() {
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "Show your pending reviews across watched repos",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}

			token := os.Getenv("GITHUB_TOKEN")
			if token == "" {
				return fmt.Errorf("GITHUB_TOKEN not set")
			}

			ghClient := proofgh.NewClient(token)

			pendingStore := proofstore.NewFileStore(filepath.Join(config.ConfigDir(), "pending.json"))

			prs, err := ghClient.FindReviewRequests(ctx, cfg.Repos,
				proofgh.WithTeams(cfg.Teams),
			)
			if err != nil {
				return fmt.Errorf("finding review requests: %w", err)
			}

			stored, err := pendingStore.List()
			if err != nil {
				cmd.PrintErrf("Warning: Failed to read pending review store: %v\n", err)
			} else {
				// Build a set of PRs already found by search
				seen := make(map[string]bool)
				for _, pr := range prs {
					seen[fmt.Sprintf("%s/%s#%d", pr.Owner, pr.Repo, pr.Number)] = true
				}
				// Add stored PRs not found by search
				for _, rec := range stored {
					key := fmt.Sprintf("%s/%s#%d", rec.Owner, rec.Repo, rec.Number)
					if !seen[key] {
						prs = append(prs, proofgh.PRInfo{
							Owner:  rec.Owner,
							Repo:   rec.Repo,
							Number: rec.Number,
						})
					}
				}
			}

			found := false
			for _, pr := range prs {
				pending, err := ghClient.ListPendingReviews(ctx, pr.Owner, pr.Repo, pr.Number)
				if err != nil {
					cmd.PrintErrf("Warning: Error checking %s/%s#%d: %v\n", pr.Owner, pr.Repo, pr.Number, err)
					continue
				}
				if len(pending) == 0 {
					// Clean up stale store entry
					pendingStore.Remove(pr.Owner, pr.Repo, pr.Number)
				}
				if len(pending) > 0 {
					if !found {
						cmd.Println("Pending reviews:")
						found = true
					}
					for _, rev := range pending {
						cmd.Printf("  • %s/%s#%d (review ID: %d)\n", pr.Owner, pr.Repo, pr.Number, rev.ID)
						cmd.Printf("    %s\n", truncate(rev.Body, 80))
						cmd.Printf("    View: https://github.com/%s/%s/pull/%d\n", pr.Owner, pr.Repo, pr.Number)
					}
				}
			}

			if !found {
				cmd.Println("No pending reviews.")
			}

			return nil
		},
	}

	rootCmd.AddCommand(listCmd)
}

func truncate(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max-3]) + "..."
}
