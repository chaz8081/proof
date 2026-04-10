//go:build copilot

package review

import (
	"context"
	"fmt"

	copilot "github.com/github/copilot-sdk/go"
)

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
		SystemMessage: &copilot.SystemMessageConfig{
			Mode:    "replace",
			Content: systemPrompt,
		},
		OnPermissionRequest: copilot.PermissionHandler.ApproveAll,
	})
	if err != nil {
		return nil, fmt.Errorf("creating copilot session: %w", err)
	}
	defer session.Disconnect()

	prompt := buildReviewPrompt(pr)

	var response string
	done := make(chan struct{})

	session.On(func(event copilot.SessionEvent) {
		switch d := event.Data.(type) {
		case *copilot.AssistantMessageData:
			response = d.Content
		case *copilot.SessionIdleData:
			close(done)
		}
	})

	if _, err := session.Send(ctx, copilot.MessageOptions{Prompt: prompt}); err != nil {
		return nil, fmt.Errorf("sending review request: %w", err)
	}

	<-done

	return parseReviewJSON(response)
}
