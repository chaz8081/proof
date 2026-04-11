// internal/cli/curate.go
package cli

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/chaz8081/proof/internal/config"
	proofgh "github.com/chaz8081/proof/internal/github"
	proofstore "github.com/chaz8081/proof/internal/store"
	"github.com/spf13/cobra"
)

func init() {
	curateCmd := &cobra.Command{
		Use:   "curate <owner/repo#number>",
		Short: "Review and manage pending comments in the terminal",
		Long:  "Interactive terminal-based curation of pending review comments. Keep, delete, or skip each comment, then submit.",
		Example: `  proof curate owner/repo#123
  proof curate https://github.com/owner/repo/pull/123`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			owner, repo, number, err := parsePRRef(args[0])
			if err != nil {
				return err
			}

			token, err := resolveToken()
			if err != nil {
				return err
			}

			ghClient := proofgh.NewClient(token)

			// Find pending review
			pending, err := ghClient.ListPendingReviews(ctx, owner, repo, number)
			if err != nil {
				return fmt.Errorf("listing pending reviews: %w", err)
			}
			if len(pending) == 0 {
				return fmt.Errorf("no pending review on %s/%s#%d\nRun 'proof poll %s/%s#%d' first.", owner, repo, number, owner, repo, number)
			}

			rev := pending[0]

			// Fetch comments
			comments, err := ghClient.GetReviewComments(ctx, owner, repo, number, rev.ID)
			if err != nil {
				return fmt.Errorf("fetching comments: %w", err)
			}

			cmd.Printf("Pending review on %s/%s#%d (%d comments)\n", owner, repo, number, len(comments))
			cmd.Printf("Summary: %s\n\n", rev.Body)

			if len(comments) == 0 {
				cmd.Println("No inline comments to curate.")
			}

			reader := bufio.NewReader(os.Stdin)
			kept := 0
			deleted := 0
			skipped := 0

			for i, c := range comments {
				cmd.Printf("Comment %d/%d — %s:%d\n", i+1, len(comments), c.GetPath(), c.GetLine())
				cmd.Printf("  %s\n\n", c.GetBody())

				cmd.Print("  (k)eep / (d)elete / (s)kip / (q)uit? ")
				input, _ := reader.ReadString('\n')
				input = strings.TrimSpace(strings.ToLower(input))

				switch input {
				case "d", "delete":
					// Delete the comment via API
					err := ghClient.DeleteReviewComment(ctx, owner, repo, c.GetID())
					if err != nil {
						cmd.PrintErrf("  Warning: Failed to delete comment: %v\n", err)
					} else {
						cmd.Println("  Deleted.")
						deleted++
					}
				case "q", "quit":
					cmd.Printf("\nStopped. Kept: %d, Deleted: %d, Skipped: %d\n", kept, deleted, skipped+len(comments)-i-1)
					return nil
				case "s", "skip":
					skipped++
				case "k", "keep", "":
					kept++
				default:
					kept++ // unknown input = keep
				}
				cmd.Println()
			}

			cmd.Printf("Kept: %d, Deleted: %d, Skipped: %d\n\n", kept, deleted, skipped)

			// Prompt for verdict and submit
			cmd.Print("Submit review? (COMMENT/APPROVE/REQUEST_CHANGES/cancel) [COMMENT]: ")
			verdictInput, _ := reader.ReadString('\n')
			verdictInput = strings.TrimSpace(strings.ToUpper(verdictInput))

			if verdictInput == "CANCEL" || verdictInput == "C" {
				cmd.Println("Review not submitted. Comments are still pending on GitHub.")
				return nil
			}

			if verdictInput == "" {
				verdictInput = "COMMENT"
			}

			if err := validateVerdict(verdictInput); err != nil {
				return err
			}

			if err := ghClient.SubmitReview(ctx, owner, repo, number, rev.ID, verdictInput); err != nil {
				return fmt.Errorf("submitting review: %w", err)
			}

			// Clean up store
			pendingStore := proofstore.NewFileStore(filepath.Join(config.ConfigDir(), "pending.json"))
			pendingStore.Remove(owner, repo, number)

			cmd.Printf("✓ Review submitted on %s/%s#%d as %s\n", owner, repo, number, verdictInput)
			return nil
		},
	}

	rootCmd.AddCommand(curateCmd)
}
