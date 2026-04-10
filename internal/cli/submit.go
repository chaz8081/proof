// internal/cli/submit.go
package cli

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	proofgh "github.com/chaz8081/proof/internal/github"
	"github.com/spf13/cobra"
)

func init() {
	var verdict string

	submitCmd := &cobra.Command{
		Use:   "submit <owner/repo#number>",
		Short: "Submit a pending review",
		Long:  "Submit a pending review to GitHub. Finds your pending review on the PR and submits it.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			owner, repo, number, err := parsePRRef(args[0])
			if err != nil {
				return err
			}

			token := os.Getenv("GITHUB_TOKEN")
			if token == "" {
				return fmt.Errorf("GITHUB_TOKEN not set")
			}

			ghClient := proofgh.NewClient(token)

			pending, err := ghClient.ListPendingReviews(ctx, owner, repo, number)
			if err != nil {
				return fmt.Errorf("listing pending reviews: %w", err)
			}
			if len(pending) == 0 {
				return fmt.Errorf("no pending review found on %s/%s#%d", owner, repo, number)
			}

			reviewID := pending[0].ID

			event := strings.ToUpper(verdict)
			if event == "" {
				event = "COMMENT"
			}

			if err := ghClient.SubmitReview(ctx, owner, repo, number, reviewID, event); err != nil {
				return fmt.Errorf("submitting review: %w", err)
			}

			cmd.Printf("Review submitted on %s/%s#%d as %s\n", owner, repo, number, event)
			cmd.Printf("View: https://github.com/%s/%s/pull/%d\n", owner, repo, number)
			return nil
		},
	}

	submitCmd.Flags().StringVar(&verdict, "verdict", "COMMENT", "Review verdict: APPROVE, REQUEST_CHANGES, or COMMENT")
	rootCmd.AddCommand(submitCmd)
}

// parsePRRef parses "owner/repo#123" into components.
func parsePRRef(ref string) (owner, repo string, number int, err error) {
	parts := strings.SplitN(ref, "#", 2)
	if len(parts) != 2 {
		return "", "", 0, fmt.Errorf("invalid PR reference %q — expected owner/repo#number", ref)
	}

	number, err = strconv.Atoi(parts[1])
	if err != nil {
		return "", "", 0, fmt.Errorf("invalid PR number in %q: %w", ref, err)
	}

	repoParts := strings.SplitN(parts[0], "/", 2)
	if len(repoParts) != 2 {
		return "", "", 0, fmt.Errorf("invalid repo in %q — expected owner/repo", ref)
	}

	return repoParts[0], repoParts[1], number, nil
}
