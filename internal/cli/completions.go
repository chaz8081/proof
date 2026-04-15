package cli

import (
	"github.com/chaz8081/proof/internal/config"
	"github.com/chaz8081/proof/internal/review"
	"github.com/spf13/cobra"
)

// completeModels provides dynamic shell completion for the --model flag
// by querying available models from the Copilot SDK.
func completeModels(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	cfg, _ := config.Load()
	token := resolveCopilotToken(cfg, "")

	models, err := review.ListModels(cmd.Context(), token)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	var completions []string
	for _, m := range models {
		completions = append(completions, m.ID)
	}
	return completions, cobra.ShellCompDirectiveNoFileComp
}

// completeProfiles provides completion for the --profile flag.
func completeProfiles(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	completions := []string{"quick", "thorough"}

	// Also add custom profiles from config
	cfg, err := config.Load()
	if err == nil && cfg.Review.Profiles != nil {
		for name := range cfg.Review.Profiles {
			completions = append(completions, name)
		}
	}

	return completions, cobra.ShellCompDirectiveNoFileComp
}
