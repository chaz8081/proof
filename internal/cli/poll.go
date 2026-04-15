// internal/cli/poll.go
package cli

import (
	"github.com/chaz8081/proof/internal/config"
	"github.com/spf13/cobra"
)

// pollOptions bundles resolved config and flag values for poll sub-functions.
type pollOptions struct {
	DryRun     bool
	Batch      bool
	ReReview   bool
	IncludeOwn bool
	ExcludeOwn bool
	Model      string
	Every      string
	Output     string
	Profile    string
	Config     *config.Config
}

func init() {
	var dryRun bool
	var model string
	var reReview bool
	var every string
	var includeOwn bool
	var excludeOwn bool
	var batch bool
	var output string
	var profile string

	pollCmd := &cobra.Command{
		Use:   "poll [owner/repo#number]",
		Short: "Check for PRs needing review and generate AI draft reviews",
		Long:  `Poll for PRs requesting your review and generate AI reviews. Optionally specify a single PR to review directly.`,
		Example: `  # Interactive — pick which PRs to review
  proof poll

  # Review a specific PR directly
  proof poll owner/repo#123

  # Include your own PRs
  proof poll --include-own

  # Watch mode — re-scan every 5 minutes
  proof poll --every 5m --batch

  # Preview without generating reviews
  proof poll --dry-run

  # Force fresh review on a PR
  proof poll owner/repo#123 --re-review

  # Use a different AI model
  proof poll --model claude-haiku-4.5`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts := pollOptions{
				DryRun:     dryRun,
				Batch:      batch,
				ReReview:   reReview,
				IncludeOwn: includeOwn,
				ExcludeOwn: excludeOwn,
				Model:      model,
				Every:      every,
				Output:     output,
				Profile:    profile,
			}
			return pollRouter(cmd, args, opts)
		},
	}

	pollCmd.Flags().BoolVar(&dryRun, "dry-run", false, "List PRs without generating reviews")
	pollCmd.Flags().StringVar(&model, "model", "", "AI model to use (overrides config)")
	pollCmd.Flags().BoolVar(&reReview, "re-review", false, "Force re-review of PRs with existing pending reviews")
	pollCmd.Flags().StringVar(&every, "every", "", "Poll repeatedly at this interval (e.g., 5m, 1h)")
	pollCmd.Flags().BoolVar(&includeOwn, "include-own", false, "Include your own PRs in the review scan")
	pollCmd.Flags().BoolVar(&excludeOwn, "exclude-own", false, "Exclude your own PRs (overrides config include_own)")
	pollCmd.Flags().BoolVar(&batch, "batch", false, "Review all PRs without interactive selection")
	pollCmd.Flags().StringVarP(&output, "output", "o", "", "Output format (json)")
	pollCmd.Flags().StringVar(&profile, "profile", "", "Review profile (quick, thorough, or custom)")

	// Dynamic completions
	pollCmd.RegisterFlagCompletionFunc("model", completeModels)
	pollCmd.RegisterFlagCompletionFunc("profile", completeProfiles)
	pollCmd.RegisterFlagCompletionFunc("output", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"json"}, cobra.ShellCompDirectiveNoFileComp
	})

	rootCmd.AddCommand(pollCmd)
}

// shortSHA returns the first 7 characters of a SHA, or the full string if shorter.
func shortSHA(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}

// pollRouter dispatches to the single-PR or multi-PR flow based on args.
func pollRouter(cmd *cobra.Command, args []string, opts pollOptions) error {
	if len(args) > 0 {
		return pollSinglePR(cmd, args[0], opts)
	}
	return pollMultiplePRs(cmd, opts)
}
