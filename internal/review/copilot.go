//go:build copilot

package review

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
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

	ListModels = func(ctx context.Context, copilotToken string) ([]ModelSummary, error) {
		opts := &copilot.ClientOptions{LogLevel: "error"}
		if copilotToken != "" {
			opts.GitHubToken = copilotToken
		}
		client := copilot.NewClient(opts)
		if err := client.Start(ctx); err != nil {
			return nil, fmt.Errorf("starting copilot: %w", err)
		}
		defer client.Stop()

		models, err := client.ListModels(ctx)
		if err != nil {
			return nil, fmt.Errorf("listing models: %w", err)
		}

		var result []ModelSummary
		for _, m := range models {
			result = append(result, ModelSummary{ID: m.ID, Name: m.Name})
		}
		return result, nil
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
// Creates a fresh session per review and retries once on timeout.
func (r *CopilotReviewer) Review(ctx context.Context, pr PRContext) (*ReviewResult, error) {
	result, err := r.reviewOnce(ctx, pr)
	if err != nil && isTimeout(err) {
		// Retry once on timeout — transient SDK hangs are common
		result, err = r.reviewOnce(ctx, pr)
		if err != nil {
			return nil, fmt.Errorf("review failed after retry: %w", err)
		}
	}
	return result, err
}

func (r *CopilotReviewer) reviewOnce(ctx context.Context, pr PRContext) (*ReviewResult, error) {
	// Fresh session per review — isolates failures
	sessionCtx, sessionCancel := context.WithTimeout(ctx, 30*time.Second)
	defer sessionCancel()

	session, err := r.client.CreateSession(sessionCtx, &copilot.SessionConfig{
		Model: pr.Model,
		SystemMessage: &copilot.SystemMessageConfig{
			Mode:    "replace",
			Content: buildSystemPrompt(pr.Instructions, pr.RepoInstructions),
		},
		OnPermissionRequest: copilot.PermissionHandler.ApproveAll,
	})
	if err != nil {
		if isTimeout(err) {
			return nil, fmt.Errorf("timed out creating Copilot session (model: %s) — the Copilot service may be slow or unavailable. Try again or use a different model with --model", pr.Model)
		}
		return nil, fmt.Errorf("creating copilot session: %w", err)
	}
	defer session.Disconnect()

	prompt := buildReviewPrompt(pr)

	// Track response and usage via event handler
	var response string
	var usage ReviewUsage
	done := make(chan struct{})
	var reviewErr error
	var doneOnce sync.Once

	unsubscribe := session.On(func(event copilot.SessionEvent) {
		switch event.Type {
		case copilot.SessionEventTypeAssistantMessage:
			if event.Data.Content != nil {
				response = *event.Data.Content
			}
		case copilot.SessionEventTypeAssistantUsage:
			if event.Data.InputTokens != nil {
				usage.InputTokens = int(*event.Data.InputTokens)
			}
			if event.Data.OutputTokens != nil {
				usage.OutputTokens = int(*event.Data.OutputTokens)
			}
			if event.Data.CacheReadTokens != nil {
				usage.CacheReadTokens = int(*event.Data.CacheReadTokens)
			}
		case copilot.SessionEventTypeSessionIdle:
			doneOnce.Do(func() { close(done) })
		case copilot.SessionEventTypeSessionError:
			errMsg := "session error"
			if event.Data.Message != nil {
				errMsg = *event.Data.Message
			}
			reviewErr = fmt.Errorf("%s", errMsg)
			doneOnce.Do(func() { close(done) })
		}
	})
	defer unsubscribe()

	// Give the model up to 3 minutes for the actual review
	sendCtx, sendCancel := context.WithTimeout(ctx, 3*time.Minute)
	defer sendCancel()

	if _, err := session.Send(sendCtx, copilot.MessageOptions{Prompt: prompt}); err != nil {
		if isTimeout(err) {
			return nil, fmt.Errorf("timed out waiting for AI review (model: %s, %d files, %d bytes diff) — try a faster model with --model or reduce diff size", pr.Model, len(pr.Files), len(pr.Diff))
		}
		return nil, fmt.Errorf("copilot review request: %w", err)
	}

	// Wait for idle or error
	select {
	case <-done:
	case <-sendCtx.Done():
		return nil, fmt.Errorf("timed out waiting for AI review (model: %s, %d files, %d bytes diff) — try a faster model with --model or reduce diff size", pr.Model, len(pr.Files), len(pr.Diff))
	}

	if reviewErr != nil {
		return nil, reviewErr
	}

	if response == "" {
		return nil, fmt.Errorf("copilot returned empty response for %s/%s#%d — try again or use a different model", pr.Owner, pr.Repo, pr.Number)
	}

	result, err := parseReviewJSON(response)
	if err != nil {
		return nil, err
	}

	// Attach usage data; count each review call as 1 premium request
	result.Usage = usage
	result.Usage.PremiumRequests = 1

	return result, nil
}

func isTimeout(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "context deadline exceeded") ||
		strings.Contains(msg, "context canceled") ||
		strings.Contains(msg, "timeout")
}
