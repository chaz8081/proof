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

func TestGroupFilesByDir(t *testing.T) {
	files := []string{
		"internal/review/parse.go",
		"internal/review/parse_test.go",
		"main.go",
		"cmd/root.go",
	}

	groups := groupFilesByDir(files)

	// root-level files should land in "(root)"
	rootFiles, ok := groups["(root)"]
	if !ok {
		t.Fatal("expected '(root)' group for root-level files")
	}
	if len(rootFiles) != 1 || rootFiles[0] != "main.go" {
		t.Errorf("expected '(root)' to contain [main.go], got %v", rootFiles)
	}

	// internal/review should contain both parse files
	reviewFiles, ok := groups["internal/review"]
	if !ok {
		t.Fatal("expected 'internal/review' group")
	}
	if len(reviewFiles) != 2 {
		t.Errorf("expected 2 files in 'internal/review', got %d: %v", len(reviewFiles), reviewFiles)
	}

	// cmd should contain root.go
	cmdFiles, ok := groups["cmd"]
	if !ok {
		t.Fatal("expected 'cmd' group")
	}
	if len(cmdFiles) != 1 || cmdFiles[0] != "root.go" {
		t.Errorf("expected 'cmd' to contain [root.go], got %v", cmdFiles)
	}
}

func TestGroupFilesByDir_Empty(t *testing.T) {
	groups := groupFilesByDir(nil)
	if len(groups) != 0 {
		t.Errorf("expected empty groups for nil input, got %v", groups)
	}
}

func TestDetectCrossFileHints_ImplAndTestBothChanged(t *testing.T) {
	files := []string{
		"internal/review/parse.go",
		"internal/review/parse_test.go",
	}

	hints := detectCrossFileHints(files)

	// Expect a hint that both impl and test changed
	found := false
	for _, h := range hints {
		if strings.Contains(h, "parse.go") && strings.Contains(h, "parse_test.go") && strings.Contains(h, "verify tests cover") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected impl+test hint for parse.go/parse_test.go, got: %v", hints)
	}
}

func TestDetectCrossFileHints_TestOnlyChanged(t *testing.T) {
	files := []string{
		"internal/review/parse_test.go",
	}

	hints := detectCrossFileHints(files)

	// Expect a hint that impl was not modified
	found := false
	for _, h := range hints {
		if strings.Contains(h, "parse_test.go") && strings.Contains(h, "verify tests still match") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected test-only hint for parse_test.go, got: %v", hints)
	}
}

func TestDetectCrossFileHints_TypesGo(t *testing.T) {
	files := []string{
		"internal/core/types.go",
	}

	hints := detectCrossFileHints(files)

	found := false
	for _, h := range hints {
		if strings.Contains(h, "types.go") && strings.Contains(h, "verify all implementations") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected types.go hint, got: %v", hints)
	}
}

func TestDetectCrossFileHints_ConfigChanges(t *testing.T) {
	for _, configFile := range []string{"config.go", "config.yaml", "config.json"} {
		t.Run(configFile, func(t *testing.T) {
			hints := detectCrossFileHints([]string{"internal/" + configFile})

			found := false
			for _, h := range hints {
				if strings.Contains(h, configFile) && strings.Contains(h, "verify defaults") {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("expected config hint for %s, got: %v", configFile, hints)
			}
		})
	}
}

func TestDetectCrossFileHints_NoMatches(t *testing.T) {
	files := []string{
		"internal/review/review.go",
		"cmd/root.go",
	}

	hints := detectCrossFileHints(files)

	if len(hints) != 0 {
		t.Errorf("expected no hints for unrelated files, got: %v", hints)
	}
}

func TestDetectCrossFileHints_Deduplicated(t *testing.T) {
	// types.go appearing once should only produce one hint
	files := []string{"pkg/types.go"}

	hints := detectCrossFileHints(files)

	count := 0
	for _, h := range hints {
		if strings.Contains(h, "types.go") {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected exactly 1 types.go hint (no duplicates), got %d: %v", count, hints)
	}
}
