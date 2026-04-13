// internal/review/factory.go
package review

import (
	"context"
	"fmt"
)

// NewReviewer creates a Reviewer. Without the copilot build tag, this returns an error.
// Build with -tags=copilot to enable the Copilot SDK reviewer.
var NewReviewer func(ctx context.Context, copilotToken string) (Reviewer, func(), error)

// ListModels returns available models. Requires copilot build tag.
var ListModels func(ctx context.Context, copilotToken string) ([]ModelSummary, error)

// ModelSummary holds basic information about an available AI model.
type ModelSummary struct {
	ID   string
	Name string
}

func init() {
	if NewReviewer == nil {
		NewReviewer = func(ctx context.Context, copilotToken string) (Reviewer, func(), error) {
			return nil, nil, fmt.Errorf("AI reviewer not available. Build with Copilot SDK support:\n  go build -tags=copilot -o proof ./cmd/proof")
		}
	}
	if ListModels == nil {
		ListModels = func(ctx context.Context, copilotToken string) ([]ModelSummary, error) {
			return nil, fmt.Errorf("model listing not available — build with -tags=copilot")
		}
	}
}
