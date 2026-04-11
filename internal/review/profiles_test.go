package review

import (
	"testing"

	"github.com/chaz8081/proof/internal/config"
)

func TestResolveProfile_BuiltinQuick(t *testing.T) {
	p := ResolveProfile("quick", nil)
	if p == nil {
		t.Fatal("expected non-nil profile for 'quick'")
	}
	if p.SeverityMin != "issue" {
		t.Errorf("expected severity_min 'issue', got %q", p.SeverityMin)
	}
	if p.MaxComments != 5 {
		t.Errorf("expected max_comments 5, got %d", p.MaxComments)
	}
	if p.Instructions == "" {
		t.Error("expected non-empty instructions for 'quick'")
	}
}

func TestResolveProfile_BuiltinThorough(t *testing.T) {
	p := ResolveProfile("thorough", nil)
	if p == nil {
		t.Fatal("expected non-nil profile for 'thorough'")
	}
	if p.SeverityMin != "nit" {
		t.Errorf("expected severity_min 'nit', got %q", p.SeverityMin)
	}
	if p.MaxComments != 0 {
		t.Errorf("expected max_comments 0 (no limit), got %d", p.MaxComments)
	}
	if p.Instructions == "" {
		t.Error("expected non-empty instructions for 'thorough'")
	}
}

func TestResolveProfile_ConfigDefined(t *testing.T) {
	cfg := &config.Config{
		Review: config.ReviewConfig{
			Profiles: map[string]config.ReviewProfile{
				"security": {
					Instructions: "Focus exclusively on security vulnerabilities.",
					SeverityMin:  "issue",
					MaxComments:  10,
				},
			},
		},
	}

	p := ResolveProfile("security", cfg)
	if p == nil {
		t.Fatal("expected non-nil profile for 'security'")
	}
	if p.SeverityMin != "issue" {
		t.Errorf("expected severity_min 'issue', got %q", p.SeverityMin)
	}
	if p.MaxComments != 10 {
		t.Errorf("expected max_comments 10, got %d", p.MaxComments)
	}
	if p.Instructions == "" {
		t.Error("expected non-empty instructions for 'security'")
	}
}

func TestResolveProfile_BuiltinTakesPriorityOverConfig(t *testing.T) {
	// Even if the user defines 'quick' in config, the builtin should win.
	cfg := &config.Config{
		Review: config.ReviewConfig{
			Profiles: map[string]config.ReviewProfile{
				"quick": {
					Instructions: "user override",
					SeverityMin:  "nit",
					MaxComments:  100,
				},
			},
		},
	}

	p := ResolveProfile("quick", cfg)
	if p == nil {
		t.Fatal("expected non-nil profile for 'quick'")
	}
	// Should match builtin, not user override
	if p.MaxComments != 5 {
		t.Errorf("expected builtin max_comments 5, got %d (user override should not win)", p.MaxComments)
	}
}

func TestResolveProfile_UnknownReturnsNil(t *testing.T) {
	p := ResolveProfile("nonexistent", nil)
	if p != nil {
		t.Errorf("expected nil for unknown profile, got %+v", p)
	}
}

func TestResolveProfile_UnknownWithConfigReturnsNil(t *testing.T) {
	cfg := &config.Config{
		Review: config.ReviewConfig{
			Profiles: map[string]config.ReviewProfile{
				"security": {Instructions: "focus on security"},
			},
		},
	}
	p := ResolveProfile("nonexistent", cfg)
	if p != nil {
		t.Errorf("expected nil for unknown profile, got %+v", p)
	}
}

func TestResolveProfile_NilConfig(t *testing.T) {
	// Built-ins should still work with nil config
	p := ResolveProfile("thorough", nil)
	if p == nil {
		t.Fatal("expected non-nil profile for 'thorough' with nil config")
	}

	// Unknown with nil config should return nil
	p2 := ResolveProfile("unknown", nil)
	if p2 != nil {
		t.Errorf("expected nil for unknown profile with nil config, got %+v", p2)
	}
}
