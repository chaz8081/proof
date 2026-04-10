//go:build copilot

package review

import (
	"context"
	"fmt"
	"time"

	copilot "github.com/github/copilot-sdk/go"
)

func init() {
	NewReviewer = func(ctx context.Context) (Reviewer, func(), error) {
		r, err := NewCopilotReviewer()
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
func NewCopilotReviewer() (*CopilotReviewer, error) {
	client := copilot.NewClient(&copilot.ClientOptions{
		LogLevel: "error",
	})

	return &CopilotReviewer{client: client}, nil
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
			Content: buildSystemPrompt(pr.Instructions),
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
