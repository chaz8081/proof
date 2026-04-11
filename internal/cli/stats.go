package cli

import (
	"fmt"
	"path/filepath"
	"sort"
	"time"

	"github.com/chaz8081/proof/internal/config"
	proofstore "github.com/chaz8081/proof/internal/store"
	"github.com/spf13/cobra"
)

func init() {
	var since string

	statsCmd := &cobra.Command{
		Use:   "stats",
		Short: "Show aggregate review metrics",
		Example: `  proof stats              # all-time stats
  proof stats --since 7d   # last 7 days
  proof stats --since 30d  # last 30 days`,
		RunE: func(cmd *cobra.Command, args []string) error {
			historyStore := proofstore.NewHistoryStore(filepath.Join(config.ConfigDir(), "reviews.jsonl"))
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
			verdicts := make(map[string]int)
			repos := make(map[string]int)
			models := make(map[string]int)

			for _, r := range records {
				totalComments += r.CommentCount
				totalFiles += r.FileCount
				totalDuration += r.Duration
				verdicts[r.Verdict]++
				repos[fmt.Sprintf("%s/%s", r.Owner, r.Repo)]++
				models[r.Model]++
			}

			avgComments := float64(totalComments) / float64(len(records))
			avgDuration := totalDuration / float64(len(records))

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

			return nil
		},
	}

	statsCmd.Flags().StringVar(&since, "since", "", "Time window (e.g., 7d, 30d, 24h)")
	rootCmd.AddCommand(statsCmd)
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
