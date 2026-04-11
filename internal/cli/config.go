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
  proof config init        # create default config
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
				return fmt.Errorf("no config found — run 'proof config init' to create one")
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

func generateDefaultConfig(username string) string {
	repoExample := "owner/repo"
	if username != "" {
		repoExample = username + "/my-repo"
	}

	return fmt.Sprintf(`# Proof configuration
# See: https://github.com/chaz8081/proof

# Repos to watch for review requests
# Use owner/repo for specific repos, or org/* for all repos in an org
repos:
  - %s

# Teams whose review requests you want to monitor (optional)
# teams:
#   - org/team-name

# Polling behavior
poll:
  ignore_drafts: true    # Skip draft PRs
  ignore_wip: true       # Skip PRs with WIP in title
  # max_files: 50        # Skip PRs with too many files
  # max_diff_bytes: 0    # Skip PRs with diffs larger than this (bytes)

# Review settings
review:
  default_verdict: COMMENT  # APPROVE, REQUEST_CHANGES, or COMMENT
  model: gpt-4.1            # AI model to use (gpt-4.1, claude-haiku-4.5, gpt-5-mini)
  # instructions: |         # Custom review instructions appended to AI prompt
  #   Focus on security and error handling.
  #   Flag any hardcoded credentials.

# Authentication (optional — defaults to GITHUB_TOKEN / gh auth token)
# auth:
#   github_token: ""     # Token for posting reviews (reviewer identity)
#   copilot_token: ""    # Token for Copilot SDK (AI model access)
`, repoExample)
}

func newConfigInitCmd(cfgPath string) *cobra.Command {
	return &cobra.Command{
		Use:     "init",
		Short:   "Create a default configuration file",
		Example: `  proof config init`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if _, err := os.Stat(cfgPath); err == nil {
				return fmt.Errorf("config already exists at %s", cfgPath)
			}

			if err := os.MkdirAll(filepath.Dir(cfgPath), 0755); err != nil {
				return fmt.Errorf("creating config directory: %w", err)
			}

			username := detectGitHubUser()
			data := generateDefaultConfig(username)

			if err := os.WriteFile(cfgPath, []byte(data), 0644); err != nil {
				return fmt.Errorf("writing config: %w", err)
			}

			cmd.Printf("Config created at %s\n\n", cfgPath)
			if username != "" {
				cmd.Printf("Detected GitHub user: %s\n", username)
			}
			cmd.Println("Next steps:")
			cmd.Println("  1. Edit the config to add your repos")
			cmd.Println("  2. Run 'proof poll --dry-run' to test")
			cmd.Println("  3. Run 'proof poll' to start reviewing")
			return nil
		},
	}
}
