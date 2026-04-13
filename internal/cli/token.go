package cli

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/chaz8081/proof/internal/config"
)

// resolveToken returns a GitHub token, using the default gh account.
func resolveToken() (string, error) {
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		return token, nil
	}
	return ghAuthToken("")
}

// resolveGitHubToken returns the token for the reviewer account.
func resolveGitHubToken(cfg *config.Config) (string, error) {
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		return token, nil
	}
	if cfg != nil && cfg.Auth.Reviewer != "" {
		return ghAuthToken(cfg.Auth.Reviewer)
	}
	return ghAuthToken("")
}

// resolveCopilotToken returns the token for the Copilot account.
func resolveCopilotToken(cfg *config.Config, fallbackToken string) string {
	if token := os.Getenv("PROOF_COPILOT_TOKEN"); token != "" {
		return token
	}
	if cfg != nil && cfg.Auth.Copilot != "" {
		token, err := ghAuthToken(cfg.Auth.Copilot)
		if err == nil {
			return token
		}
	}
	return fallbackToken
}

// ghAuthToken gets a token from gh CLI, optionally for a specific user.
func ghAuthToken(user string) (string, error) {
	args := []string{"auth", "token"}
	if user != "" {
		args = append(args, "--user", user)
	}
	cmd := exec.Command("gh", args...)
	output, err := cmd.Output()
	if err != nil {
		if user != "" {
			return "", fmt.Errorf("failed to get token for %q via 'gh auth token --user %s': %w\nRun 'gh auth login' to authenticate this account", user, user, err)
		}
		return "", fmt.Errorf("GITHUB_TOKEN not set and 'gh auth token' failed: %w\nSet GITHUB_TOKEN or run 'gh auth login'", err)
	}
	token := strings.TrimSpace(string(output))
	if token == "" {
		return "", fmt.Errorf("'gh auth token' returned empty token")
	}
	return token, nil
}

// listGHAccounts returns the list of GitHub accounts configured in gh CLI.
func listGHAccounts() ([]string, error) {
	// Parse gh auth status output to extract account names
	cmd := exec.Command("gh", "auth", "status")
	output, _ := cmd.CombinedOutput() // exits non-zero sometimes, but still prints accounts

	var accounts []string
	for _, line := range strings.Split(string(output), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "✓ Logged in to") && strings.Contains(line, "account") {
			// "✓ Logged in to github.com account youruser (keyring)"
			parts := strings.Fields(line)
			for i, p := range parts {
				if p == "account" && i+1 < len(parts) {
					accounts = append(accounts, parts[i+1])
				}
			}
		}
	}
	return accounts, nil
}
