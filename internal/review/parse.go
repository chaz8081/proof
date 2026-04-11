package review

import (
	"encoding/json"
	"fmt"
	"path/filepath"
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

// buildSystemPrompt composes the base system prompt with repo instructions and optional user-configured instructions.
func buildSystemPrompt(userInstructions string, repoInstructions RepoInstructions) string {
	prompt := systemPrompt

	if repoInstructions.RepoWide != "" {
		prompt += "\n\n## Repository Review Instructions\n" + repoInstructions.RepoWide
	}
	for _, pi := range repoInstructions.PathSpecific {
		prompt += "\n\n## Path-Specific Instructions\n" + pi
	}
	if repoInstructions.AgentInstructions != "" {
		prompt += "\n\n## Agent Instructions\n" + repoInstructions.AgentInstructions
	}
	if userInstructions != "" {
		prompt += "\n\n## Your Custom Review Instructions\n" + userInstructions
	}

	return prompt
}

// buildReviewPrompt constructs the prompt sent to the model for a PR review.
func buildReviewPrompt(pr PRContext) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Pull Request: %s/%s#%d\n", pr.Owner, pr.Repo, pr.Number)
	fmt.Fprintf(&b, "**Title**: %s\n", pr.Title)
	if pr.Description != "" {
		fmt.Fprintf(&b, "**Description**: %s\n", pr.Description)
	}

	// Group files by directory and type for context
	fmt.Fprintf(&b, "**Files changed** (%d):\n", len(pr.Files))
	groups := groupFilesByDir(pr.Files)
	for dir, files := range groups {
		fmt.Fprintf(&b, "  %s/: %s\n", dir, strings.Join(files, ", "))
	}

	// Add cross-file hints
	hints := detectCrossFileHints(pr.Files)
	if len(hints) > 0 {
		b.WriteString("\n**Cross-file relationships to check:**\n")
		for _, hint := range hints {
			fmt.Fprintf(&b, "- %s\n", hint)
		}
	}

	fmt.Fprintf(&b, "\n## Diff\n\n```diff\n%s\n```\n", pr.Diff)
	return b.String()
}

// groupFilesByDir groups files by their directory.
func groupFilesByDir(files []string) map[string][]string {
	groups := make(map[string][]string)
	for _, f := range files {
		dir := filepath.Dir(f)
		if dir == "." {
			dir = "(root)"
		}
		groups[dir] = append(groups[dir], filepath.Base(f))
	}
	return groups
}

// detectCrossFileHints identifies common cross-file patterns that reviewers should check.
func detectCrossFileHints(files []string) []string {
	var hints []string

	fileSet := make(map[string]bool)
	for _, f := range files {
		fileSet[f] = true
	}

	for _, f := range files {
		base := filepath.Base(f)
		dir := filepath.Dir(f)
		ext := filepath.Ext(f)
		nameWithoutExt := strings.TrimSuffix(base, ext)

		// Test file changed without implementation (or vice versa)
		if strings.HasSuffix(nameWithoutExt, "_test") {
			implFile := filepath.Join(dir, strings.TrimSuffix(nameWithoutExt, "_test")+ext)
			if !fileSet[implFile] {
				hints = append(hints, fmt.Sprintf("Test file %s changed but implementation %s was not modified — verify tests still match the implementation", f, implFile))
			}
		} else {
			testFile := filepath.Join(dir, nameWithoutExt+"_test"+ext)
			if fileSet[testFile] {
				hints = append(hints, fmt.Sprintf("Both %s and its test %s changed — verify tests cover the new behavior", f, testFile))
			}
		}

		// Interface/types files changed
		if base == "types.go" || base == "interfaces.go" || base == "models.go" {
			hints = append(hints, fmt.Sprintf("Type definitions in %s changed — verify all implementations and callers are updated", f))
		}

		// Config changes
		if base == "config.go" || base == "config.yaml" || base == "config.json" {
			hints = append(hints, fmt.Sprintf("Configuration in %s changed — verify defaults, validation, and documentation are updated", f))
		}
	}

	// Deduplicate
	seen := make(map[string]bool)
	var unique []string
	for _, h := range hints {
		if !seen[h] {
			seen[h] = true
			unique = append(unique, h)
		}
	}

	return unique
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
