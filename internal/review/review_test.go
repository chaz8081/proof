// internal/review/review_test.go
package review

import (
	"context"
	"strings"
	"testing"
)

// mockReviewer verifies the interface is implementable.
type mockReviewer struct {
	result *ReviewResult
	err    error
}

func (m *mockReviewer) Review(_ context.Context, _ PRContext) (*ReviewResult, error) {
	return m.result, m.err
}

func TestReviewerInterface(t *testing.T) {
	mock := &mockReviewer{
		result: &ReviewResult{
			Summary: "Looks good overall",
			Verdict: "APPROVE",
			Comments: []InlineComment{
				{
					Path:     "main.go",
					Line:     42,
					Body:     "Consider using a constant here",
					Severity: "nit",
				},
			},
		},
	}

	var r Reviewer = mock
	result, err := r.Review(context.Background(), PRContext{
		Owner:       "chaz8081",
		Repo:        "proof",
		Number:      1,
		Title:       "Add feature",
		Description: "This PR adds a feature",
		Diff:        "diff --git a/main.go...",
		Files:       []string{"main.go"},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Summary != "Looks good overall" {
		t.Errorf("unexpected summary: %q", result.Summary)
	}
	if result.Verdict != "APPROVE" {
		t.Errorf("unexpected verdict: %q", result.Verdict)
	}
	if len(result.Comments) != 1 {
		t.Fatalf("expected 1 comment, got %d", len(result.Comments))
	}
	if result.Comments[0].Line != 42 {
		t.Errorf("expected line 42, got %d", result.Comments[0].Line)
	}
}

func TestInlineComment_FormattedBody(t *testing.T) {
	tests := []struct {
		name     string
		comment  InlineComment
		contains string
	}{
		{
			name: "with severity and suggestion",
			comment: InlineComment{
				Path:       "main.go",
				Line:       10,
				Body:       "Use errors.New instead",
				Severity:   "suggestion",
				Suggestion: "return errors.New(\"failed\")",
			},
			contains: "```suggestion",
		},
		{
			name: "without suggestion",
			comment: InlineComment{
				Path:     "main.go",
				Line:     10,
				Body:     "Nice refactor",
				Severity: "nit",
			},
			contains: "[nit]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := tt.comment.FormattedBody()
			if body == "" {
				t.Fatal("expected non-empty body")
			}
			if !strings.Contains(body, tt.contains) {
				t.Errorf("expected body to contain %q, got %q", tt.contains, body)
			}
		})
	}
}
