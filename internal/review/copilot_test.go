package review

import (
	"testing"
)

func TestParseReviewResponse(t *testing.T) {
	raw := `{
		"summary": "This PR adds a new endpoint. Overall looks clean.",
		"verdict": "COMMENT",
		"comments": [
			{
				"path": "handler.go",
				"line": 25,
				"body": "Missing error check on db.Query",
				"severity": "issue"
			},
			{
				"path": "handler.go",
				"line": 42,
				"body": "Consider using a constant",
				"severity": "nit",
				"suggestion": "const maxRetries = 3"
			}
		]
	}`

	result, err := parseReviewJSON(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Summary == "" {
		t.Error("expected non-empty summary")
	}
	if result.Verdict != "COMMENT" {
		t.Errorf("expected COMMENT verdict, got %q", result.Verdict)
	}
	if len(result.Comments) != 2 {
		t.Fatalf("expected 2 comments, got %d", len(result.Comments))
	}
	if result.Comments[0].Severity != "issue" {
		t.Errorf("expected 'issue' severity, got %q", result.Comments[0].Severity)
	}
	if result.Comments[1].Suggestion != "const maxRetries = 3" {
		t.Errorf("unexpected suggestion: %q", result.Comments[1].Suggestion)
	}
}

func TestParseReviewResponse_ExtractsJSON(t *testing.T) {
	// Models sometimes wrap JSON in markdown code blocks
	raw := "Here is my review:\n```json\n{\"summary\":\"LGTM\",\"verdict\":\"APPROVE\",\"comments\":[]}\n```\nLet me know if you need anything else."

	result, err := parseReviewJSON(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Verdict != "APPROVE" {
		t.Errorf("expected APPROVE, got %q", result.Verdict)
	}
}

func TestParseReviewResponse_InvalidJSON(t *testing.T) {
	_, err := parseReviewJSON("this is not json at all")
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}
