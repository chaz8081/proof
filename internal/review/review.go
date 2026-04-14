// internal/review/review.go
package review

import (
	"context"
	"fmt"
	"strings"
)

// Reviewer generates a structured code review from a PR context.
type Reviewer interface {
	Review(ctx context.Context, pr PRContext) (*ReviewResult, error)
}

type RepoInstructions struct {
	RepoWide          string   // from .github/copilot-instructions.md
	PathSpecific      []string // from matching .github/instructions/*.instructions.md
	AgentInstructions string   // from AGENTS.md
}

type PRContext struct {
	Owner            string
	Repo             string
	Number           int
	Title            string
	Description      string
	Diff             string
	Files            []string
	Instructions     string
	Model            string
	RepoInstructions RepoInstructions
	HeadSHA          string
}

// ReviewUsage holds token and premium request counts from the AI provider.
type ReviewUsage struct {
	InputTokens     int
	OutputTokens    int
	CacheReadTokens int
	PremiumRequests int
}

type ReviewResult struct {
	Summary  string          `json:"summary"`
	Verdict  string          `json:"verdict"`
	Comments []InlineComment `json:"comments"`
	Usage    ReviewUsage     `json:"-"` // not from AI response, populated by reviewer
}

type InlineComment struct {
	Path       string `json:"path"`
	Line       int    `json:"line"`
	Body       string `json:"body"`
	Severity   string `json:"severity"`
	Suggestion string `json:"suggestion,omitempty"`
}

// FormattedBody returns the comment body formatted for GitHub,
// with severity tag and optional suggestion block.
func (c InlineComment) FormattedBody() string {
	var b strings.Builder

	fmt.Fprintf(&b, "**[%s]** %s", c.Severity, c.Body)

	if c.Suggestion != "" {
		fmt.Fprintf(&b, "\n\n```suggestion\n%s\n```", c.Suggestion)
	}

	return b.String()
}
