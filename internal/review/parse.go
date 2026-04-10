package review

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// systemPrompt is the system message sent to the model for code review.
const systemPrompt = `You are a senior code reviewer. You will receive a pull request diff and metadata.
Analyze the code changes and respond with ONLY a JSON object (no markdown, no explanation) in this exact format:

{
  "summary": "2-3 sentence high-level assessment of the PR",
  "verdict": "APPROVE or REQUEST_CHANGES or COMMENT",
  "comments": [
    {
      "path": "path/to/file.go",
      "line": 42,
      "body": "Clear, actionable comment about this line",
      "severity": "nit|suggestion|issue|blocker",
      "suggestion": "optional replacement code for this line"
    }
  ]
}

Rules:
- Line numbers must reference lines in the NEW version of the file (right side of diff)
- severity levels: nit (style/preference), suggestion (improvement), issue (likely bug or problem), blocker (must fix)
- Only include suggestion field when you have a concrete code replacement
- Be concise and actionable. Don't restate what the code does — say what should change and why
- If the PR looks good, use verdict APPROVE with an empty or minimal comments array
- Focus on bugs, security issues, and logic errors over style`

// buildSystemPrompt composes the base system prompt with optional user-configured instructions.
func buildSystemPrompt(userInstructions string) string {
	if userInstructions == "" {
		return systemPrompt
	}
	return systemPrompt + "\n\nAdditional review instructions from configuration:\n" + userInstructions
}

// buildReviewPrompt constructs the prompt sent to the model for a PR review.
func buildReviewPrompt(pr PRContext) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Pull Request: %s/%s#%d\n", pr.Owner, pr.Repo, pr.Number)
	fmt.Fprintf(&b, "**Title**: %s\n", pr.Title)
	if pr.Description != "" {
		fmt.Fprintf(&b, "**Description**: %s\n", pr.Description)
	}
	fmt.Fprintf(&b, "**Files changed**: %s\n\n", strings.Join(pr.Files, ", "))
	fmt.Fprintf(&b, "## Diff\n\n```diff\n%s\n```\n", pr.Diff)
	return b.String()
}

// parseReviewJSON extracts and parses a ReviewResult from the model's response.
// Handles both raw JSON and JSON wrapped in markdown code blocks.
func parseReviewJSON(raw string) (*ReviewResult, error) {
	jsonStr := extractJSON(raw)

	var result ReviewResult
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return nil, fmt.Errorf("parsing review JSON: %w (raw response: %.200s)", err, raw)
	}

	return &result, nil
}

var jsonBlockRe = regexp.MustCompile("(?s)```(?:json)?\\s*\\n?(\\{.*?\\})\\s*\\n?```")

func extractJSON(s string) string {
	// Try to extract from markdown code block first
	if matches := jsonBlockRe.FindStringSubmatch(s); len(matches) > 1 {
		return matches[1]
	}

	// Try to find raw JSON object
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start >= 0 && end > start {
		return s[start : end+1]
	}

	return s
}
