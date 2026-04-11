// internal/cli/log.go
package cli

import (
	"encoding/json"
	"path/filepath"

	"github.com/chaz8081/proof/internal/config"
	proofstore "github.com/chaz8081/proof/internal/store"
	"github.com/spf13/cobra"
)

func init() {
	var pr string
	var limit int
	var outputFormat string

	logCmd := &cobra.Command{
		Use:   "log",
		Short: "Show review history",
		Example: `  proof log                    # recent reviews
  proof log --pr owner/repo#42  # reviews for a specific PR
  proof log --limit 5           # last 5 reviews
  proof log -o json             # JSON output`,
		RunE: func(cmd *cobra.Command, args []string) error {
			historyStore := proofstore.NewHistoryStore(filepath.Join(config.ConfigDir(), "reviews.jsonl"))

			var records []proofstore.ReviewRecord
			var err error

			if pr != "" {
				owner, repo, number, err := parsePRRef(pr)
				if err != nil {
					return err
				}
				records, err = historyStore.ListForPR(owner, repo, number)
				if err != nil {
					return err
				}
			} else {
				records, err = historyStore.List()
				if err != nil {
					return err
				}
			}

			if len(records) == 0 {
				cmd.Println("No review history yet. Run 'proof poll' to start reviewing.")
				return nil
			}

			// Reverse for newest-first
			for i, j := 0, len(records)-1; i < j; i, j = i+1, j-1 {
				records[i], records[j] = records[j], records[i]
			}

			// Apply limit
			if limit > 0 && limit < len(records) {
				records = records[:limit]
			}

			// Output
			if outputFormat == "json" {
				data, _ := json.MarshalIndent(records, "", "  ")
				cmd.Println(string(data))
				return nil
			}

			cmd.Printf("Review History (%d total)\n\n", len(records))
			for _, r := range records {
				cmd.Printf("  %s  %s/%s#%d — %s\n", r.Timestamp.Format("2006-01-02 15:04"), r.Owner, r.Repo, r.Number, r.Verdict)
				cmd.Printf("           %d comments, %d files, %.1fs (%s)\n\n", r.CommentCount, r.FileCount, r.Duration, r.Model)
			}

			return nil
		},
	}

	logCmd.Flags().StringVar(&pr, "pr", "", "Filter by PR (owner/repo#number)")
	logCmd.Flags().IntVar(&limit, "limit", 20, "Max records to show")
	logCmd.Flags().StringVarP(&outputFormat, "output", "o", "", "Output format (json)")
	rootCmd.AddCommand(logCmd)
}
