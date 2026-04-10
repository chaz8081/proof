// internal/review/factory.go
package review

import (
	"context"
	"fmt"
)

// NewReviewer creates a Reviewer. Without the copilot build tag, this returns an error.
// Build with -tags=copilot to enable the Copilot SDK reviewer.
var NewReviewer func(ctx context.Context) (Reviewer, func(), error)

func init() {
	if NewReviewer == nil {
		NewReviewer = func(ctx context.Context) (Reviewer, func(), error) {
			return nil, nil, fmt.Errorf("no reviewer available — build with -tags=copilot to enable Copilot SDK")
		}
	}
}
