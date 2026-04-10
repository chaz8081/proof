// internal/cli/list.go
package cli

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sync"
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

type pendingResult struct {
	Owner    string
	Repo     string
	Number   int
	ReviewID int64
	Body     string
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

			// Phase 1: Local fast path — show stored pending reviews immediately
			stored, _ := pendingStore.List()
			if len(stored) > 0 && output != "json" {
				cmd.Println("Pending reviews (from local store):")
				for _, rec := range stored {
					cmd.Printf("  • %s/%s#%d (review ID: %d)\n", rec.Owner, rec.Repo, rec.Number, rec.ReviewID)
					cmd.Printf("    View: https://github.com/%s/%s/pull/%d\n", rec.Owner, rec.Repo, rec.Number)
				}
			}

			// Phase 2: Verify against GitHub + find any not in store (concurrent)
			prs, err := ghClient.FindReviewRequests(ctx, cfg.Repos,
				proofgh.WithTeams(cfg.Teams),
			)
			if err != nil {
				return fmt.Errorf("finding review requests: %w", err)
			}

			// Merge stored PRs not found by search
			seen := make(map[string]bool)
			for _, pr := range prs {
				seen[fmt.Sprintf("%s/%s#%d", pr.Owner, pr.Repo, pr.Number)] = true
			}
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

			var (
				mu      sync.Mutex
				results []pendingResult
				wg      sync.WaitGroup
			)

			// Limit concurrency to avoid rate limit issues
			sem := make(chan struct{}, 5) // max 5 concurrent API calls

			for _, pr := range prs {
				wg.Add(1)
				go func(pr proofgh.PRInfo) {
					defer wg.Done()
					sem <- struct{}{}        // acquire
					defer func() { <-sem }() // release

					pending, err := ghClient.ListPendingReviews(ctx, pr.Owner, pr.Repo, pr.Number)
					if err != nil {
						cmd.PrintErrf("Warning: Error checking %s/%s#%d: %v\n", pr.Owner, pr.Repo, pr.Number, err)
						return
					}
					if len(pending) == 0 {
						pendingStore.Remove(pr.Owner, pr.Repo, pr.Number)
						return
					}

					mu.Lock()
					for _, rev := range pending {
						results = append(results, pendingResult{
							Owner:    pr.Owner,
							Repo:     pr.Repo,
							Number:   pr.Number,
							ReviewID: rev.ID,
							Body:     rev.Body,
						})
					}
					mu.Unlock()
				}(pr)
			}
			wg.Wait()

			if output == "json" {
				items := make([]listOutputItem, 0, len(results))
				for _, r := range results {
					items = append(items, listOutputItem{
						Owner:    r.Owner,
						Repo:     r.Repo,
						Number:   r.Number,
						ReviewID: r.ReviewID,
						Body:     r.Body,
					})
				}
				out, err := json.MarshalIndent(items, "", "  ")
				if err != nil {
					return fmt.Errorf("marshaling JSON: %w", err)
				}
				fmt.Fprintln(cmd.OutOrStdout(), string(out))
				return nil
			}

			if len(results) > 0 {
				cmd.Println("Pending reviews (verified):")
				for _, r := range results {
					cmd.Printf("  • %s/%s#%d (review ID: %d)\n", r.Owner, r.Repo, r.Number, r.ReviewID)
					cmd.Printf("    %s\n", truncate(r.Body, 80))
					cmd.Printf("    View: https://github.com/%s/%s/pull/%d\n", r.Owner, r.Repo, r.Number)
				}
			} else if len(stored) == 0 {
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
