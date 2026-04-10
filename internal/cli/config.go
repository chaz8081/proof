// internal/cli/config.go
package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/chaz8081/proof/internal/config"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func init() {
	configCmd := &cobra.Command{
		Use:   "config",
		Short: "Show or manage configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfgPath := filepath.Join(config.ConfigDir(), "config.yaml")
			data, err := os.ReadFile(cfgPath)
			if err != nil {
				return fmt.Errorf("no config found — run 'proof config init' to create one")
			}
			cmd.Println(string(data))
			return nil
		},
	}

	cfgPath := filepath.Join(config.ConfigDir(), "config.yaml")
	configCmd.AddCommand(newConfigInitCmd(cfgPath))
	rootCmd.AddCommand(configCmd)
}

func newConfigInitCmd(cfgPath string) *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Create a default configuration file",
		RunE: func(cmd *cobra.Command, args []string) error {
			if _, err := os.Stat(cfgPath); err == nil {
				return fmt.Errorf("config already exists at %s", cfgPath)
			}

			if err := os.MkdirAll(filepath.Dir(cfgPath), 0755); err != nil {
				return fmt.Errorf("creating config directory: %w", err)
			}

			cfg := config.DefaultConfig()
			data, err := yaml.Marshal(cfg)
			if err != nil {
				return fmt.Errorf("marshaling config: %w", err)
			}

			if err := os.WriteFile(cfgPath, data, 0644); err != nil {
				return fmt.Errorf("writing config: %w", err)
			}

			cmd.Printf("Config created at %s\n", cfgPath)
			cmd.Println("Edit it to add your repos and teams.")
			return nil
		},
	}
}
