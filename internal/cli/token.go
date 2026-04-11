package cli

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/chaz8081/proof/internal/config"
)

// resolveGitHubToken returns the token for GitHub API calls (posting reviews).
// Priority: config auth.github_token > GITHUB_TOKEN env > gh auth token
func resolveGitHubToken(cfg *config.Config) (string, error) {
	if cfg != nil && cfg.Auth.GithubToken != "" {
		return cfg.Auth.GithubToken, nil
	}
	return resolveToken()
}

// resolveCopilotToken returns the token for Copilot SDK auth.
// Priority: config auth.copilot_token > PROOF_COPILOT_TOKEN env > falls back to GitHub token
func resolveCopilotToken(cfg *config.Config, githubToken string) string {
	if cfg != nil && cfg.Auth.CopilotToken != "" {
		return cfg.Auth.CopilotToken
	}
	if t := os.Getenv("PROOF_COPILOT_TOKEN"); t != "" {
		return t
	}
	return githubToken // same account if not configured separately
}

// resolveToken returns the GitHub token from GITHUB_TOKEN env var,
// falling back to `gh auth token` if not set.
func resolveToken() (string, error) {
	token := os.Getenv("GITHUB_TOKEN")
	if token != "" {
		return token, nil
	}

	cmd := exec.Command("gh", "auth", "token")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("GITHUB_TOKEN not set and 'gh auth token' failed: %w\nSet GITHUB_TOKEN or run 'gh auth login'", err)
	}

	token = strings.TrimSpace(string(output))
	if token == "" {
		return "", fmt.Errorf("GITHUB_TOKEN not set and 'gh auth token' returned empty\nRun 'gh auth login' to authenticate")
	}

	return token, nil
}
