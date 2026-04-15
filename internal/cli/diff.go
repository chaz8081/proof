package cli

import (
	"fmt"
	"path/filepath"

	"github.com/chaz8081/proof/internal/config"
	proofstore "github.com/chaz8081/proof/internal/store"
	"github.com/spf13/cobra"
)

func init() {
	diffCmd := &cobra.Command{
		Use:   "diff <owner/repo#number>",
		Short: "Compare two reviews of the same PR",
		Long:  "Shows what changed between the two most recent reviews of a PR — issues fixed, new issues, and verdict changes.",
		Example: `  proof diff owner/repo#42
  proof diff https://github.com/owner/repo/pull/42`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			owner, repo, number, err := parsePRRef(args[0])
			if err != nil {
				return err
			}

			historyStore := proofstore.NewHistoryStore(filepath.Join(config.DataDir(), "reviews.jsonl"))
			reviews, err := historyStore.ListForPR(owner, repo, number)
			if err != nil {
				return fmt.Errorf("reading review history: %w", err)
			}

			if len(reviews) < 2 {
				if len(reviews) == 1 {
					cmd.Printf("Only 1 review found for %s/%s#%d. Need at least 2 to compare.\n", owner, repo, number)
					cmd.Println("Run 'proof poll --re-review' to create a second review.")
				} else {
					cmd.Printf("No reviews found for %s/%s#%d.\n", owner, repo, number)
				}
				return nil
			}

			// Compare last two (most recent)
			prev := reviews[len(reviews)-2]
			curr := reviews[len(reviews)-1]

			cmd.Printf("Review Diff: %s/%s#%d\n", owner, repo, number)
			cmd.Println("────────────────────────────────")
			cmd.Printf("Previous: %s (%s, %d comments)\n", prev.Timestamp.Format("2006-01-02 15:04"), prev.Verdict, prev.CommentCount)
			cmd.Printf("Current:  %s (%s, %d comments)\n", curr.Timestamp.Format("2006-01-02 15:04"), curr.Verdict, curr.CommentCount)
			cmd.Println()

			// Verdict change
			if prev.Verdict != curr.Verdict {
				cmd.Printf("Verdict changed: %s → %s\n", prev.Verdict, curr.Verdict)
			} else {
				cmd.Printf("Verdict unchanged: %s\n", curr.Verdict)
			}

			// Comment count change
			delta := curr.CommentCount - prev.CommentCount
			if delta > 0 {
				cmd.Printf("Comments: %d → %d (+%d new)\n", prev.CommentCount, curr.CommentCount, delta)
			} else if delta < 0 {
				cmd.Printf("Comments: %d → %d (%d resolved)\n", prev.CommentCount, curr.CommentCount, -delta)
			} else {
				cmd.Printf("Comments: %d (unchanged)\n", curr.CommentCount)
			}

			// File count change
			fileDelta := curr.FileCount - prev.FileCount
			if fileDelta != 0 {
				cmd.Printf("Files: %d → %d\n", prev.FileCount, curr.FileCount)
			}

			// Duration
			cmd.Printf("Review time: %.1fs → %.1fs\n", prev.Duration, curr.Duration)

			// Model
			if prev.Model != curr.Model {
				cmd.Printf("Model changed: %s → %s\n", prev.Model, curr.Model)
			}

			return nil
		},
	}

	diffCmd.ValidArgsFunction = completeHistoryPRs
	rootCmd.AddCommand(diffCmd)
}
