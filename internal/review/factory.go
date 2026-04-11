// internal/review/factory.go
package review

import (
	"context"
	"fmt"
)

// NewReviewer creates a Reviewer. Without the copilot build tag, this returns an error.
// Build with -tags=copilot to enable the Copilot SDK reviewer.
var NewReviewer func(ctx context.Context, copilotToken string) (Reviewer, func(), error)

func init() {
	if NewReviewer == nil {
		NewReviewer = func(ctx context.Context, copilotToken string) (Reviewer, func(), error) {
			return nil, nil, fmt.Errorf("AI reviewer not available. Build with Copilot SDK support:\n  go build -tags=copilot -o proof ./cmd/proof")
		}
	}
}
