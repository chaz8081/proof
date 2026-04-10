package cli

import (
	"encoding/json"
	"fmt"

	proofgh "github.com/chaz8081/proof/internal/github"
	"github.com/spf13/cobra"
)

// showOutputComment is the JSON representation of a single inline comment for show output.
type showOutputComment struct {
	Path string `json:"path"`
	Line int    `json:"line"`
	Body string `json:"body"`
}

// showOutputItem is the JSON representation of a pending review with comments for show output.
type showOutputItem struct {
	Owner    string              `json:"owner"`
	Repo     string              `json:"repo"`
	Number   int                 `json:"number"`
	ReviewID int64               `json:"review_id"`
	Body     string              `json:"body"`
	Comments []showOutputComment `json:"comments"`
}

func init() {
	var output string

	showCmd := &cobra.Command{
		Use:   "show <owner/repo#number>",
		Short: "Preview a pending review before submitting",
		Args:  cobra.ExactArgs(1),
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

			pending, err := ghClient.ListPendingReviews(ctx, owner, repo, number)
			if err != nil {
				return fmt.Errorf("listing pending reviews: %w", err)
			}
			if len(pending) == 0 {
				return fmt.Errorf("no pending review found on %s/%s#%d\nRun 'proof poll' first to generate an AI review.", owner, repo, number)
			}

			rev := pending[0]

			if output == "json" {
				// Fetch inline comments
				ghComments, err := ghClient.GetReviewComments(ctx, owner, repo, number, rev.ID)
				if err != nil {
					cmd.PrintErrf("Warning: Could not fetch comments: %v\n", err)
				}

				var comments []showOutputComment
				for _, c := range ghComments {
					comments = append(comments, showOutputComment{
						Path: c.GetPath(),
						Line: c.GetLine(),
						Body: c.GetBody(),
					})
				}
				if comments == nil {
					comments = []showOutputComment{}
				}

				item := showOutputItem{
					Owner:    owner,
					Repo:     repo,
					Number:   number,
					ReviewID: rev.ID,
					Body:     rev.Body,
					Comments: comments,
				}
				out, err := json.MarshalIndent(item, "", "  ")
				if err != nil {
					return fmt.Errorf("marshaling JSON: %w", err)
				}
				fmt.Fprintln(cmd.OutOrStdout(), string(out))
				return nil
			}

			cmd.Printf("# Pending Review: %s/%s#%d\n", owner, repo, number)
			cmd.Printf("Review ID: %d\n\n", rev.ID)
			cmd.Printf("## Summary\n%s\n\n", rev.Body)

			// Fetch inline comments
			comments, err := ghClient.GetReviewComments(ctx, owner, repo, number, rev.ID)
			if err != nil {
				cmd.PrintErrf("Warning: Could not fetch comments: %v\n", err)
				return nil
			}

			if len(comments) == 0 {
				cmd.Println("No inline comments.")
			} else {
				cmd.Printf("## Comments (%d)\n\n", len(comments))
				for i, c := range comments {
					cmd.Printf("### %d. %s (line %d)\n", i+1, c.GetPath(), c.GetLine())
					cmd.Printf("%s\n\n", c.GetBody())
				}
			}

			cmd.Printf("---\nActions:\n")
			cmd.Printf("  proof submit %s/%s#%d --approve\n", owner, repo, number)
			cmd.Printf("  proof submit %s/%s#%d --request-changes\n", owner, repo, number)
			cmd.Printf("  proof dismiss %s/%s#%d\n", owner, repo, number)

			return nil
		},
	}

	showCmd.Flags().StringVarP(&output, "output", "o", "", "Output format (json)")
	rootCmd.AddCommand(showCmd)
}
