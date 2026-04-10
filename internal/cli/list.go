// internal/cli/list.go
package cli

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"time"

	"github.com/chaz8081/proof/internal/config"
	proofgh "github.com/chaz8081/proof/internal/github"
	proofstore "github.com/chaz8081/proof/internal/store"
	"github.com/spf13/cobra"
)

// listOutputItem is the JSON representation of a single pending review for list output.
type listOutputItem struct {
	Owner    string `json:"owner"`
	Repo     string `json:"repo"`
	Number   int    `json:"number"`
	ReviewID int64  `json:"review_id"`
	Body     string `json:"body"`
}

func init() {
	var output string

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "Show your pending reviews across watched repos",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

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

			if output == "json" {
				var items []listOutputItem
				for _, pr := range prs {
					pending, err := ghClient.ListPendingReviews(ctx, pr.Owner, pr.Repo, pr.Number)
					if err != nil {
						cmd.PrintErrf("Warning: Error checking %s/%s#%d: %v\n", pr.Owner, pr.Repo, pr.Number, err)
						continue
					}
					if len(pending) == 0 {
						pendingStore.Remove(pr.Owner, pr.Repo, pr.Number)
					}
					for _, rev := range pending {
						items = append(items, listOutputItem{
							Owner:    pr.Owner,
							Repo:     pr.Repo,
							Number:   pr.Number,
							ReviewID: rev.ID,
							Body:     rev.Body,
						})
					}
				}
				if items == nil {
					items = []listOutputItem{}
				}
				out, err := json.MarshalIndent(items, "", "  ")
				if err != nil {
					return fmt.Errorf("marshaling JSON: %w", err)
				}
				fmt.Fprintln(cmd.OutOrStdout(), string(out))
				return nil
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

	listCmd.Flags().StringVarP(&output, "output", "o", "", "Output format (json)")
	rootCmd.AddCommand(listCmd)
}

func truncate(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max-3]) + "..."
}
