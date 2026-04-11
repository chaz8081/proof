// internal/cli/config.go
package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/chaz8081/proof/internal/config"
	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Show or manage configuration",
	Example: `  proof config show        # display current config
  proof config init        # create config (alias for 'proof setup')
  proof config validate    # check config for issues`,
	// No RunE — shows help with subcommand list by default.
}

func init() {
	cfgPath := filepath.Join(config.ConfigDir(), "config.yaml")
	configCmd.AddCommand(newConfigInitCmd(cfgPath))
	configCmd.AddCommand(newConfigShowCmd(cfgPath))
	rootCmd.AddCommand(configCmd)
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
