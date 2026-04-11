// internal/cli/status.go
package cli

import (
	"path/filepath"
	"strings"

	"github.com/chaz8081/proof/internal/config"
	proofstore "github.com/chaz8081/proof/internal/store"
	"github.com/spf13/cobra"
)

func init() {
	statusCmd := &cobra.Command{
		Use:     "status",
		Short:   "Show configuration and review status at a glance",
		Example: "  proof status",
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.Println("Proof Status")
			cmd.Println(strings.Repeat("─", 35))
			cmd.Println()

			cfgPath := filepath.Join(config.ConfigDir(), "config.yaml")

			// Config
			cfg, err := config.Load()
			if err != nil {
				cmd.Printf("Config:   %s ✗ (%v)\n", cfgPath, err)
				cmd.Println()
				cmd.Println("Run 'proof setup' to create a config.")
				return nil
			}
			cmd.Printf("Config:   %s ✓\n", cfgPath)

			// Auth
			ghToken, ghErr := resolveGitHubToken(cfg)
			if ghErr != nil {
				cmd.Println("Auth:     not configured (run 'proof setup' or set GITHUB_TOKEN)")
			} else {
				copilotToken := resolveCopilotToken(cfg, ghToken)
				if copilotToken != ghToken {
					cmd.Println("Auth:     dual-account (separate reviewer + copilot tokens)")
				} else {
					cmd.Println("Auth:     single account")
				}
			}

			// Repos
			repoNames := cfg.RepoNames()
			cmd.Printf("Repos:    %d watched\n", len(repoNames))

			// Model
			cmd.Printf("Model:    %s\n", cfg.Review.Model)

			// Verdict
			cmd.Printf("Verdict:  %s\n", cfg.Review.DefaultVerdict)

			// Pending reviews from local store
			pendingStore := proofstore.NewFileStore(filepath.Join(config.ConfigDir(), "pending.json"))
			stored, _ := pendingStore.List()
			if len(stored) > 0 {
				cmd.Printf("\nPending:  %d review(s)\n", len(stored))
				for _, rec := range stored {
					cmd.Printf("  • %s/%s#%d (review ID: %d)\n", rec.Owner, rec.Repo, rec.Number, rec.ReviewID)
				}
			} else {
				cmd.Println("\nPending:  none")
			}

			return nil
		},
	}

	rootCmd.AddCommand(statusCmd)
}
