// internal/cli/validate.go
package cli

import (
	"fmt"
	"path/filepath"

	"github.com/chaz8081/proof/internal/config"
	"github.com/spf13/cobra"
)

func init() {
	cfgPath := filepath.Join(config.ConfigDir(), "config.yaml")
	configCmd.AddCommand(newConfigValidateCmd(cfgPath))
}

func newConfigValidateCmd(cfgPath string) *cobra.Command {
	return &cobra.Command{
		Use:   "validate",
		Short: "Validate the configuration file",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadFromPath(cfgPath)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			issues := cfg.Validate()
			if len(issues) == 0 {
				cmd.Println("Config is valid.")
				return nil
			}

			cmd.Println("Config issues found:")
			for _, issue := range issues {
				cmd.Printf("  - %s\n", issue)
			}
			return nil
		},
	}
}
