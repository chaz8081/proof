package cli

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"time"

	"github.com/chaz8081/proof/internal/config"
	proofstore "github.com/chaz8081/proof/internal/store"
	"github.com/spf13/cobra"
)

type statsOutput struct {
	TotalReviews         int            `json:"total_reviews"`
	AvgComments          float64        `json:"avg_comments_per_pr"`
	AvgDuration          float64        `json:"avg_duration_seconds"`
	Verdicts             map[string]int `json:"verdicts"`
	ByRepo               map[string]int `json:"by_repo"`
	ByModel              map[string]int `json:"by_model"`
	TotalInputTokens     int            `json:"total_input_tokens,omitempty"`
	TotalOutputTokens    int            `json:"total_output_tokens,omitempty"`
	TotalPremiumRequests int            `json:"total_premium_requests,omitempty"`
}

func init() {
	var since string
	var outputFormat string

	statsCmd := &cobra.Command{
		Use:   "stats",
		Short: "Show aggregate review metrics",
		Example: `  proof stats              # all-time stats
  proof stats --since 7d   # last 7 days
  proof stats --since 30d  # last 30 days
  proof stats -o json      # JSON output`,
		RunE: func(cmd *cobra.Command, args []string) error {
			historyStore := proofstore.NewHistoryStore(filepath.Join(config.DataDir(), "reviews.jsonl"))
			records, err := historyStore.List()
			if err != nil {
				return fmt.Errorf("reading review history: %w", err)
			}

			if len(records) == 0 {
				cmd.Println("No review history yet. Run 'proof poll' to start reviewing.")
				return nil
			}

			// Filter by --since
			if since != "" {
				duration, err := parseDuration(since)
				if err != nil {
					return fmt.Errorf("invalid --since value %q: %w", since, err)
				}
				cutoff := time.Now().Add(-duration)
				var filtered []proofstore.ReviewRecord
				for _, r := range records {
					if r.Timestamp.After(cutoff) {
						filtered = append(filtered, r)
					}
				}
				records = filtered
				if len(records) == 0 {
					cmd.Printf("No reviews in the last %s.\n", since)
					return nil
				}
			}

			// Compute stats
			totalComments := 0
			totalFiles := 0
			totalDuration := 0.0
			totalInputTokens := 0
			totalOutputTokens := 0
			totalPremiumRequests := 0
			verdicts := make(map[string]int)
			repos := make(map[string]int)
			models := make(map[string]int)

			for _, r := range records {
				totalComments += r.CommentCount
				totalFiles += r.FileCount
				totalDuration += r.Duration
				totalInputTokens += r.InputTokens
				totalOutputTokens += r.OutputTokens
				totalPremiumRequests += r.PremiumRequests
				verdicts[r.Verdict]++
				repos[fmt.Sprintf("%s/%s", r.Owner, r.Repo)]++
				models[r.Model]++
			}

			avgComments := float64(totalComments) / float64(len(records))
			avgDuration := totalDuration / float64(len(records))

			// JSON output
			if outputFormat == "json" {
				out := statsOutput{
					TotalReviews:         len(records),
					AvgComments:          avgComments,
					AvgDuration:          avgDuration,
					Verdicts:             verdicts,
					ByRepo:               repos,
					ByModel:              models,
					TotalInputTokens:     totalInputTokens,
					TotalOutputTokens:    totalOutputTokens,
					TotalPremiumRequests: totalPremiumRequests,
				}
				data, _ := json.MarshalIndent(out, "", "  ")
				cmd.Println(string(data))
				return nil
			}

			// Display
			cmd.Println("Review Stats")
			cmd.Println("────────────────────────────────")
			cmd.Printf("Total reviews:     %d\n", len(records))
			cmd.Printf("Avg comments/PR:   %.1f\n", avgComments)
			cmd.Printf("Avg review time:   %.1fs\n", avgDuration)
			cmd.Printf("Total files:       %d\n", totalFiles)
			cmd.Println()

			// Verdicts
			cmd.Println("Verdicts:")
			for _, v := range []string{"APPROVE", "COMMENT", "REQUEST_CHANGES"} {
				if count, ok := verdicts[v]; ok {
					pct := float64(count) / float64(len(records)) * 100
					cmd.Printf("  %-18s %d (%.0f%%)\n", v, count, pct)
				}
			}
			cmd.Println()

			// Top repos
			cmd.Println("By Repo:")
			type repoCount struct {
				name  string
				count int
			}
			var repoList []repoCount
			for name, count := range repos {
				repoList = append(repoList, repoCount{name, count})
			}
			sort.Slice(repoList, func(i, j int) bool { return repoList[i].count > repoList[j].count })
			for _, r := range repoList {
				cmd.Printf("  %-30s %d reviews\n", r.name, r.count)
			}
			cmd.Println()

			// Models
			cmd.Println("By Model:")
			for model, count := range models {
				cmd.Printf("  %-20s %d reviews\n", model, count)
			}

			// Usage (only shown when token data is available)
			if totalInputTokens > 0 || totalPremiumRequests > 0 {
				cmd.Println()
				cmd.Println("Usage:")
				cmd.Printf("  Premium requests:  %d (%.1f avg/review)\n", totalPremiumRequests, float64(totalPremiumRequests)/float64(len(records)))
				cmd.Printf("  Tokens consumed:   %s in / %s out\n", formatTokens(totalInputTokens), formatTokens(totalOutputTokens))
				cmd.Printf("  Avg tokens/PR:     %s in / %s out\n", formatTokens(totalInputTokens/len(records)), formatTokens(totalOutputTokens/len(records)))
			}

			return nil
		},
	}

	statsCmd.Flags().StringVar(&since, "since", "", "Time window (e.g., 7d, 30d, 24h)")
	statsCmd.Flags().StringVarP(&outputFormat, "output", "o", "", "Output format (json)")
	statsCmd.RegisterFlagCompletionFunc("since", completeSince)
	statsCmd.RegisterFlagCompletionFunc("output", completeOutputFormat)
	rootCmd.AddCommand(statsCmd)
}

// formatTokens formats a token count as a human-readable string (e.g., "1.5k", "2.3M").
func formatTokens(n int) string {
	if n >= 1_000_000 {
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	}
	if n >= 1000 {
		return fmt.Sprintf("%.1fk", float64(n)/1000)
	}
	return fmt.Sprintf("%d", n)
}

// parseDuration parses durations like "7d", "30d", "24h", "1h"
func parseDuration(s string) (time.Duration, error) {
	if len(s) < 2 {
		return 0, fmt.Errorf("too short")
	}
	unit := s[len(s)-1]
	val := s[:len(s)-1]
	var n int
	_, err := fmt.Sscanf(val, "%d", &n)
	if err != nil {
		return 0, err
	}
	switch unit {
	case 'd':
		return time.Duration(n) * 24 * time.Hour, nil
	case 'h':
		return time.Duration(n) * time.Hour, nil
	case 'm':
		return time.Duration(n) * time.Minute, nil
	default:
		return time.ParseDuration(s)
	}
}
