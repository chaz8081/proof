//go:build copilot

package review

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	copilot "github.com/github/copilot-sdk/go"
)

func init() {
	NewReviewer = func(ctx context.Context, copilotToken string) (Reviewer, func(), error) {
		r, err := NewCopilotReviewer(copilotToken)
		if err != nil {
			return nil, nil, err
		}
		if err := r.Start(ctx); err != nil {
			return nil, nil, err
		}
		return r, func() { r.Stop() }, nil
	}
}

// CopilotReviewer implements Reviewer using the GitHub Copilot SDK.
type CopilotReviewer struct {
	client *copilot.Client
}

// NewCopilotReviewer creates a new CopilotReviewer backed by the Copilot SDK.
// If token is empty, it falls back to resolveGitHubToken (gh auth token).
func NewCopilotReviewer(token string) (*CopilotReviewer, error) {
	if token == "" {
		var err error
		token, err = resolveGitHubToken()
		if err != nil {
			return nil, fmt.Errorf("resolving GitHub token for Copilot: %w", err)
		}
	}

	opts := &copilot.ClientOptions{LogLevel: "error"}
	if token != "" {
		opts.GitHubToken = token
	}
	client := copilot.NewClient(opts)

	return &CopilotReviewer{client: client}, nil
}

// resolveGitHubToken returns a GitHub OAuth token suitable for the Copilot API.
// PATs (ghp_) are not accepted; it strips GITHUB_TOKEN from the subprocess env
// so that `gh auth token` returns the keyring OAuth token (gho_) instead.
func resolveGitHubToken() (string, error) {
	cmd := exec.Command("gh", "auth", "token")
	// Inherit current env but remove GITHUB_TOKEN so gh uses the keyring OAuth token.
	cmd.Env = make([]string, 0, len(os.Environ()))
	for _, e := range os.Environ() {
		if !strings.HasPrefix(e, "GITHUB_TOKEN=") {
			cmd.Env = append(cmd.Env, e)
		}
	}
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("'gh auth token' failed: %w\nRun 'gh auth login' to authenticate", err)
	}
	token := strings.TrimSpace(string(out))
	if token == "" {
		return "", fmt.Errorf("'gh auth token' returned empty\nRun 'gh auth login' to authenticate")
	}
	if strings.HasPrefix(token, "ghp_") {
		return "", fmt.Errorf("Copilot API requires an OAuth token, not a PAT\nRun 'gh auth login' with a GitHub account that has Copilot access")
	}
	return token, nil
}

// Start initializes the Copilot CLI server process.
func (r *CopilotReviewer) Start(ctx context.Context) error {
	return r.client.Start(ctx)
}

// Stop shuts down the Copilot CLI server process.
func (r *CopilotReviewer) Stop() {
	r.client.Stop()
}

// Review sends the PR context to Copilot for analysis and returns a structured review.
func (r *CopilotReviewer) Review(ctx context.Context, pr PRContext) (*ReviewResult, error) {
	session, err := r.client.CreateSession(ctx, &copilot.SessionConfig{
		Model: pr.Model,
		SystemMessage: &copilot.SystemMessageConfig{
			Mode:    "replace",
			Content: buildSystemPrompt(pr.Instructions, pr.RepoInstructions),
		},
		OnPermissionRequest: copilot.PermissionHandler.ApproveAll,
	})
	if err != nil {
		return nil, fmt.Errorf("creating copilot session: %w", err)
	}
	defer session.Disconnect()

	prompt := buildReviewPrompt(pr)

	// Give the model up to 3 minutes for large diffs
	sendCtx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()

	event, err := session.SendAndWait(sendCtx, copilot.MessageOptions{Prompt: prompt})
	if err != nil {
		return nil, fmt.Errorf("copilot review request: %w", err)
	}

	if event == nil || event.Data.Content == nil {
		return nil, fmt.Errorf("copilot returned empty response")
	}

	return parseReviewJSON(*event.Data.Content)
}
