// internal/cli/config.go
package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/chaz8081/proof/internal/config"
	"github.com/chaz8081/proof/internal/review"
	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Show or manage configuration",
	Example: `  proof config show        # display current config
  proof config edit        # interactively edit config
  proof config init        # create config (alias for 'proof setup')
  proof config validate    # check config for issues
  proof config models      # list available AI models`,
	// No RunE — shows help with subcommand list by default.
}

func init() {
	cfgPath := filepath.Join(config.ConfigDir(), "config.yaml")
	configCmd.AddCommand(newConfigInitCmd(cfgPath))
	configCmd.AddCommand(newConfigShowCmd(cfgPath))
	configCmd.AddCommand(newConfigModelsCmd())
	configCmd.AddCommand(newConfigEditCmd())
	rootCmd.AddCommand(configCmd)
}

func newConfigModelsCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "models",
		Short:   "List available AI models",
		Example: "  proof config models",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _ := config.Load()
			copilotToken := resolveCopilotToken(cfg, "")

			models, err := review.ListModels(cmd.Context(), copilotToken)
			if err != nil {
				return err
			}

			if len(models) == 0 {
				cmd.Println("No models available.")
				return nil
			}

			cmd.Println("Available models:")
			for _, m := range models {
				name := m.Name
				if name == "" {
					name = m.ID
				}
				cmd.Printf("  %-30s %s\n", m.ID, name)
			}
			return nil
		},
	}
}

func newConfigShowCmd(cfgPath string) *cobra.Command {
	return &cobra.Command{
		Use:     "show",
		Short:   "Display current configuration",
		Example: `  proof config show`,
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := os.ReadFile(cfgPath)
			if err != nil {
				return fmt.Errorf("no config found — run 'proof setup' to create one")
			}
			cmd.Print(string(data))
			return nil
		},
	}
}

func detectGitHubUser() string {
	cmd := exec.Command("gh", "api", "user", "--jq", ".login")
	output, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}

func newConfigInitCmd(cfgPath string) *cobra.Command {
	return &cobra.Command{
		Use:     "init",
		Short:   "Create a configuration file (runs the setup wizard)",
		Example: `  proof config init`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return setupCmd.RunE(setupCmd, args)
		},
	}
}
