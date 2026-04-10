package cli

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

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
