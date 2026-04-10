package review

import (
	"strings"
	"testing"
)

func TestBuildSystemPrompt_NoInstructions(t *testing.T) {
	result := buildSystemPrompt("")
	if result != systemPrompt {
		t.Errorf("expected base systemPrompt when no instructions given, got different result")
	}
}

func TestBuildSystemPrompt_WithInstructions(t *testing.T) {
	instructions := "Focus on security vulnerabilities."
	result := buildSystemPrompt(instructions)

	if !strings.HasPrefix(result, systemPrompt) {
		t.Error("expected result to start with base systemPrompt")
	}
	if !strings.Contains(result, "Additional review instructions from configuration:") {
		t.Error("expected result to contain instructions header")
	}
	if !strings.Contains(result, instructions) {
		t.Errorf("expected result to contain instructions %q", instructions)
	}
}

func TestBuildSystemPrompt_InstructionsAppendedAfterBase(t *testing.T) {
	instructions := "Only flag blockers and issues."
	result := buildSystemPrompt(instructions)

	baseEnd := strings.Index(result, "\n\nAdditional review instructions")
	if baseEnd < 0 {
		t.Fatal("expected separator between base prompt and instructions")
	}
	if result[:baseEnd] != systemPrompt {
		t.Error("base prompt content must come before the instructions separator")
	}
}
