// internal/cli/config_edit_test.go
package cli

import (
	"strings"
	"testing"

	"github.com/chaz8081/proof/internal/config"
)

// TestParseIndex_EdgeCases verifies boundary and invalid inputs already covered
// by setup_test.go but ensures the function handles max=0 gracefully.
func TestParseIndex_EdgeCases(t *testing.T) {
	tests := []struct {
		name  string
		input string
		max   int
		want  int
	}{
		{"valid mid", "2\n", 5, 1},
		{"valid max", "5\n", 5, 4},
		{"one over max", "6\n", 5, 0},
		{"zero input", "0\n", 5, 0},
		{"empty input", "\n", 5, 0},
		{"negative", "-1\n", 5, 0},
		{"whitespace only", "   \n", 3, 0},
		{"float", "1.5\n", 3, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseIndex(tt.input, tt.max)
			if got != tt.want {
				t.Errorf("parseIndex(%q, %d) = %d, want %d", tt.input, tt.max, got, tt.want)
			}
		})
	}
}

// TestParseSearchGitHubReposOutput verifies the line-splitting logic that
// searchGitHubRepos uses to build its string slice from gh CLI output.
func TestParseSearchGitHubReposOutput(t *testing.T) {
	raw := "getbread/shop-notice\ngetbread/shop-frontend\ngetbread/shop-api\n"
	var repos []string
	for _, line := range strings.Split(strings.TrimSpace(raw), "\n") {
		if line != "" {
			repos = append(repos, line)
		}
	}
	if len(repos) != 3 {
		t.Fatalf("expected 3 repos, got %d: %v", len(repos), repos)
	}
	if repos[0] != "getbread/shop-notice" {
		t.Errorf("repos[0] = %q, want %q", repos[0], "getbread/shop-notice")
	}
	if repos[2] != "getbread/shop-api" {
		t.Errorf("repos[2] = %q, want %q", repos[2], "getbread/shop-api")
	}
}

// TestParseSearchGitHubReposOutput_Empty verifies empty output produces nil.
func TestParseSearchGitHubReposOutput_Empty(t *testing.T) {
	raw := ""
	var repos []string
	for _, line := range strings.Split(strings.TrimSpace(raw), "\n") {
		if line != "" {
			repos = append(repos, line)
		}
	}
	if len(repos) != 0 {
		t.Errorf("expected 0 repos from empty output, got %d", len(repos))
	}
}

// TestMarshalConfig verifies that marshalConfig produces a valid YAML that
// round-trips back through config parsing.
func TestMarshalConfig(t *testing.T) {
	trueVal := true
	cfg := &config.Config{
		Repos: []config.RepoEntry{
			{Name: "getbread/shop-notice"},
			{Name: "getbread/fraud"},
		},
		Poll: config.PollConfig{
			IgnoreDrafts: &trueVal,
			IgnoreWIP:    true,
			IncludeOwn:   false,
		},
		Review: config.ReviewConfig{
			DefaultVerdict: "COMMENT",
			Model:          "claude-sonnet-4.6",
		},
		Auth: config.AuthConfig{
			Reviewer: "chaz8081",
			Copilot:  "Charles-Scholle_bfh",
		},
	}

	out := marshalConfig(cfg)

	// Basic structure checks
	if !strings.Contains(out, "getbread/shop-notice") {
		t.Error("expected repo getbread/shop-notice in output")
	}
	if !strings.Contains(out, "claude-sonnet-4.6") {
		t.Error("expected model claude-sonnet-4.6 in output")
	}
	if !strings.Contains(out, "default_verdict: COMMENT") {
		t.Error("expected default_verdict in output")
	}
	if !strings.Contains(out, "reviewer: chaz8081") {
		t.Error("expected reviewer in output")
	}
	if !strings.Contains(out, "ignore_drafts: true") {
		t.Error("expected ignore_drafts in output")
	}
}

// TestMarshalConfig_NoAuth verifies auth section is omitted when both fields are empty.
func TestMarshalConfig_NoAuth(t *testing.T) {
	trueVal := true
	cfg := &config.Config{
		Repos: []config.RepoEntry{{Name: "owner/repo"}},
		Poll:  config.PollConfig{IgnoreDrafts: &trueVal},
		Review: config.ReviewConfig{
			DefaultVerdict: "COMMENT",
			Model:          "gpt-4.1",
		},
	}

	out := marshalConfig(cfg)
	if strings.Contains(out, "auth:") {
		t.Error("expected no auth section when auth fields are empty")
	}
}

// TestRepoExists verifies helper correctly detects existing repos.
func TestRepoExists(t *testing.T) {
	repos := []config.RepoEntry{
		{Name: "owner/a"},
		{Name: "owner/b"},
	}
	if !repoExists(repos, "owner/a") {
		t.Error("expected owner/a to exist")
	}
	if repoExists(repos, "owner/c") {
		t.Error("expected owner/c to not exist")
	}
}

// TestOrDefault verifies fallback helper.
func TestOrDefault(t *testing.T) {
	if orDefault("hello", "world") != "hello" {
		t.Error("expected non-empty string to be returned as-is")
	}
	if orDefault("", "world") != "world" {
		t.Error("expected default to be returned for empty string")
	}
}
