package review

import (
	"strings"
	"testing"
)

func TestBuildSystemPrompt_NoInstructions(t *testing.T) {
	result := buildSystemPrompt("", RepoInstructions{})
	if result != systemPrompt {
		t.Errorf("expected base systemPrompt when no instructions given, got different result")
	}
}

func TestBuildSystemPrompt_WithUserInstructions(t *testing.T) {
	instructions := "Focus on security vulnerabilities."
	result := buildSystemPrompt(instructions, RepoInstructions{})

	if !strings.HasPrefix(result, systemPrompt) {
		t.Error("expected result to start with base systemPrompt")
	}
	if !strings.Contains(result, "## Your Custom Review Instructions") {
		t.Error("expected result to contain custom instructions header")
	}
	if !strings.Contains(result, instructions) {
		t.Errorf("expected result to contain instructions %q", instructions)
	}
}

func TestBuildSystemPrompt_UserInstructionsAppendedAfterBase(t *testing.T) {
	instructions := "Only flag blockers and issues."
	result := buildSystemPrompt(instructions, RepoInstructions{})

	baseEnd := strings.Index(result, "\n\n## Your Custom Review Instructions")
	if baseEnd < 0 {
		t.Fatal("expected separator between base prompt and instructions")
	}
	if result[:baseEnd] != systemPrompt {
		t.Error("base prompt content must come before the instructions separator")
	}
}

func TestBuildSystemPrompt_WithRepoWide(t *testing.T) {
	ri := RepoInstructions{
		RepoWide: "Always check for nil pointer dereferences.",
	}
	result := buildSystemPrompt("", ri)

	if !strings.HasPrefix(result, systemPrompt) {
		t.Error("expected result to start with base systemPrompt")
	}
	if !strings.Contains(result, "## Repository Review Instructions") {
		t.Error("expected Repository Review Instructions section")
	}
	if !strings.Contains(result, "nil pointer") {
		t.Error("expected repo-wide instruction content")
	}
}

func TestBuildSystemPrompt_WithPathSpecific(t *testing.T) {
	ri := RepoInstructions{
		PathSpecific: []string{"Check error handling in Go files.", "Ensure no panics."},
	}
	result := buildSystemPrompt("", ri)

	count := strings.Count(result, "## Path-Specific Instructions")
	if count != 2 {
		t.Errorf("expected 2 Path-Specific Instructions sections, got %d", count)
	}
	if !strings.Contains(result, "Check error handling") {
		t.Error("expected first path-specific content")
	}
	if !strings.Contains(result, "no panics") {
		t.Error("expected second path-specific content")
	}
}

func TestBuildSystemPrompt_WithAgentInstructions(t *testing.T) {
	ri := RepoInstructions{
		AgentInstructions: "This project uses Beads for issue tracking.",
	}
	result := buildSystemPrompt("", ri)

	if !strings.Contains(result, "## Agent Instructions") {
		t.Error("expected Agent Instructions section")
	}
	if !strings.Contains(result, "Beads") {
		t.Error("expected agent instruction content")
	}
}

func TestBuildSystemPrompt_OrderingIsCorrect(t *testing.T) {
	ri := RepoInstructions{
		RepoWide:          "repo-wide",
		PathSpecific:      []string{"path-specific"},
		AgentInstructions: "agent",
	}
	result := buildSystemPrompt("user", ri)

	repoWideIdx := strings.Index(result, "repo-wide")
	pathSpecIdx := strings.Index(result, "path-specific")
	agentIdx := strings.Index(result, "agent")
	userIdx := strings.Index(result, "user")

	if repoWideIdx < 0 || pathSpecIdx < 0 || agentIdx < 0 || userIdx < 0 {
		t.Fatal("one or more expected sections missing from prompt")
	}
	if !(repoWideIdx < pathSpecIdx && pathSpecIdx < agentIdx && agentIdx < userIdx) {
		t.Errorf("expected order: repo-wide < path-specific < agent < user instructions; got indices %d %d %d %d",
			repoWideIdx, pathSpecIdx, agentIdx, userIdx)
	}
}
