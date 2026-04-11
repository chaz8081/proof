// internal/cli/submit.go
package cli

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/chaz8081/proof/internal/config"
	proofgh "github.com/chaz8081/proof/internal/github"
	proofstore "github.com/chaz8081/proof/internal/store"
	"github.com/spf13/cobra"
)

var validVerdicts = map[string]bool{
	"APPROVE":         true,
	"REQUEST_CHANGES": true,
	"COMMENT":         true,
}

func validateVerdict(verdict string) error {
	if !validVerdicts[verdict] {
		return fmt.Errorf("invalid verdict %q — must be APPROVE, REQUEST_CHANGES, or COMMENT", verdict)
	}
	return nil
}

// resolveVerdict validates flag mutual exclusivity and returns the resolved verdict string.
// An empty string is returned when none of the shorthand flags are set, leaving
// the caller to fall back to config or a default.
func resolveVerdict(approve, requestChanges bool, verdict string) (string, error) {
	set := 0
	if approve {
		set++
	}
	if requestChanges {
		set++
	}
	if verdict != "" {
		set++
	}
	if set > 1 {
		return "", fmt.Errorf("only one of --verdict, --approve, or --request-changes can be specified")
	}

	if approve {
		return "APPROVE", nil
	}
	if requestChanges {
		return "REQUEST_CHANGES", nil
	}
	return verdict, nil
}

func init() {
	var verdict string
	var approve bool
	var requestChanges bool

	submitCmd := &cobra.Command{
		Use:   "submit <owner/repo#number>",
		Short: "Submit a pending review",
		Long:  "Submit a pending review to GitHub. Finds your pending review on the PR and submits it.",
		Example: `  proof submit owner/repo#123
  proof submit owner/repo#123 --approve
  proof submit owner/repo#123 --request-changes
  proof submit owner/repo#123 --verdict COMMENT`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			pendingStore := proofstore.NewFileStore(filepath.Join(config.ConfigDir(), "pending.json"))

			resolved, err := resolveVerdict(approve, requestChanges, verdict)
			if err != nil {
				return err
			}

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
				return fmt.Errorf("No pending review found on %s/%s#%d\nRun 'proof poll' first to generate an AI review, or create one manually on GitHub.", owner, repo, number)
			}

			reviewID := pending[0].ID

			event := strings.ToUpper(resolved)
			if event == "" {
				cfg, err := config.Load()
				if err != nil {
					event = "COMMENT" // fallback if no config
				} else {
					event = cfg.Review.DefaultVerdict
				}
			}

			if err := validateVerdict(event); err != nil {
				return err
			}

			if err := ghClient.SubmitReview(ctx, owner, repo, number, reviewID, event); err != nil {
				return fmt.Errorf("submitting review: %w", err)
			}

			if err := pendingStore.Remove(owner, repo, number); err != nil {
				cmd.PrintErrf("Warning: Failed to update pending review store: %v\n", err)
			}

			cmd.Printf("Review submitted on %s/%s#%d as %s\n", owner, repo, number, event)
			cmd.Printf("View: https://github.com/%s/%s/pull/%d\n", owner, repo, number)
			return nil
		},
	}

	submitCmd.Flags().StringVar(&verdict, "verdict", "", "Review verdict: APPROVE, REQUEST_CHANGES, or COMMENT")
	submitCmd.Flags().BoolVar(&approve, "approve", false, "Submit as APPROVE")
	submitCmd.Flags().BoolVar(&requestChanges, "request-changes", false, "Submit as REQUEST_CHANGES")
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
