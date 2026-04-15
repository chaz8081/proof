package cli

import (
	"fmt"
	"path/filepath"

	"github.com/chaz8081/proof/internal/config"
	"github.com/chaz8081/proof/internal/review"
	proofstore "github.com/chaz8081/proof/internal/store"
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

	cfg, err := config.Load()
	if err == nil && cfg.Review.Profiles != nil {
		for name := range cfg.Review.Profiles {
			completions = append(completions, name)
		}
	}

	return completions, cobra.ShellCompDirectiveNoFileComp
}

// completePendingPRs provides completion for commands that take a PR ref
// by reading from the local pending review store.
func completePendingPRs(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	store := proofstore.NewFileStore(filepath.Join(config.DataDir(), "pending.json"))
	records, err := store.List()
	if err != nil || len(records) == 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	var completions []string
	for _, r := range records {
		completions = append(completions, fmt.Sprintf("%s/%s#%d", r.Owner, r.Repo, r.Number))
	}
	return completions, cobra.ShellCompDirectiveNoFileComp
}

// completeHistoryPRs provides completion from review history for commands like diff.
func completeHistoryPRs(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	history := proofstore.NewHistoryStore(filepath.Join(config.DataDir(), "reviews.jsonl"))
	records, err := history.List()
	if err != nil || len(records) == 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	// Deduplicate PR refs
	seen := make(map[string]bool)
	var completions []string
	for _, r := range records {
		key := fmt.Sprintf("%s/%s#%d", r.Owner, r.Repo, r.Number)
		if !seen[key] {
			seen[key] = true
			completions = append(completions, key)
		}
	}
	return completions, cobra.ShellCompDirectiveNoFileComp
}

// completeOutputFormat provides completion for --output flags.
func completeOutputFormat(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return []string{"json"}, cobra.ShellCompDirectiveNoFileComp
}

// completeSince provides completion for --since flags.
func completeSince(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return []string{"1h", "24h", "7d", "30d", "90d"}, cobra.ShellCompDirectiveNoFileComp
}

// completeEvery provides completion for --every flag.
func completeEvery(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return []string{"1m", "5m", "10m", "30m", "1h"}, cobra.ShellCompDirectiveNoFileComp
}
