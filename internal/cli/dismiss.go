package cli

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/chaz8081/proof/internal/config"
	proofgh "github.com/chaz8081/proof/internal/github"
	proofstore "github.com/chaz8081/proof/internal/store"
	"github.com/spf13/cobra"
)

func init() {
	dismissCmd := &cobra.Command{
		Use:     "dismiss <owner/repo#number>",
		Short:   "Delete a pending review from GitHub",
		Long:    "Deletes your pending review on the specified PR and removes it from the local store.",
		Example: `  proof dismiss owner/repo#123`,
		Args:    cobra.ExactArgs(1),
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
			pendingStore := proofstore.NewFileStore(filepath.Join(config.ConfigDir(), "pending.json"))

			pending, err := ghClient.ListPendingReviews(ctx, owner, repo, number)
			if err != nil {
				if strings.Contains(err.Error(), "404") {
					return fmt.Errorf("PR %s/%s#%d not found — check the owner, repo, and PR number", owner, repo, number)
				}
				return fmt.Errorf("checking pending reviews: %w", err)
			}
			if len(pending) == 0 {
				return fmt.Errorf("no pending review found on %s/%s#%d", owner, repo, number)
			}

			reviewID := pending[0].ID

			if err := ghClient.DeletePendingReview(ctx, owner, repo, number, reviewID); err != nil {
				return err
			}

			if err := pendingStore.Remove(owner, repo, number); err != nil {
				cmd.PrintErrf("Warning: Failed to update local store: %v\n", err)
			}

			cmd.Printf("Pending review deleted from %s/%s#%d\n", owner, repo, number)
			return nil
		},
	}

	rootCmd.AddCommand(dismissCmd)
}
