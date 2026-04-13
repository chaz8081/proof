// internal/cli/root.go
package cli

import "github.com/spf13/cobra"

var rootCmd = &cobra.Command{
	Use:           "proof",
	Short:         "AI-assisted PR review with human-in-the-loop",
	Long:          `Proof pre-reviews GitHub PRs using AI and creates pending reviews for you to curate. Submit reviews through GitHub's UI to maintain human accountability. Let it rise before you bake it in.`,
	SilenceUsage:  true,
	SilenceErrors: true,
}

func Execute() error {
	return rootCmd.Execute()
}
