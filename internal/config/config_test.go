// internal/config/config_test.go
package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig_ValidFile(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(cfgPath, []byte(`
repos:
  - owner/repo-a
  - myorg/*
teams:
  - myorg/my-team
poll:
  ignore_drafts: true
  ignore_wip: true
  max_files: 50
review:
  default_verdict: COMMENT
`), 0644)
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadFromPath(cfgPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Repos) != 2 {
		t.Errorf("expected 2 repos, got %d", len(cfg.Repos))
	}
	if cfg.Repos[0] != "owner/repo-a" {
		t.Errorf("expected 'owner/repo-a', got %q", cfg.Repos[0])
	}
	if cfg.Repos[1] != "myorg/*" {
		t.Errorf("expected 'myorg/*', got %q", cfg.Repos[1])
	}
	if len(cfg.Teams) != 1 || cfg.Teams[0] != "myorg/my-team" {
		t.Errorf("unexpected teams: %v", cfg.Teams)
	}
	if cfg.Poll.IgnoreDrafts == nil || !*cfg.Poll.IgnoreDrafts {
		t.Error("expected ignore_drafts to be true")
	}
	if !cfg.Poll.IgnoreWIP {
		t.Error("expected ignore_wip to be true")
	}
	if cfg.Poll.MaxFiles != 50 {
		t.Errorf("expected max_files 50, got %d", cfg.Poll.MaxFiles)
	}
	if cfg.Review.DefaultVerdict != "COMMENT" {
		t.Errorf("expected default_verdict COMMENT, got %q", cfg.Review.DefaultVerdict)
	}
}

func TestLoadConfig_FileNotFound(t *testing.T) {
	_, err := LoadFromPath("/nonexistent/config.yaml")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestLoadConfig_Defaults(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(cfgPath, []byte("repos:\n  - owner/repo\n"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadFromPath(cfgPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Poll.IgnoreDrafts == nil || !*cfg.Poll.IgnoreDrafts {
		t.Error("expected ignore_drafts to default to true")
	}
	if cfg.Poll.MaxFiles != 0 {
		t.Error("expected max_files to default to 0 (no limit)")
	}
	if cfg.Review.DefaultVerdict != "COMMENT" {
		t.Errorf("expected default verdict COMMENT, got %q", cfg.Review.DefaultVerdict)
	}
}

func TestDefaultConfig_GeneratesValidYAML(t *testing.T) {
	cfg := DefaultConfig()
	if len(cfg.Repos) != 1 || cfg.Repos[0] != "owner/repo" {
		t.Errorf("unexpected default repos: %v", cfg.Repos)
	}
}

func TestConfigDir(t *testing.T) {
	dir := ConfigDir()
	if dir == "" {
		t.Error("expected non-empty config dir")
	}
}

func TestLoadConfig_ReviewInstructions(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(cfgPath, []byte(`
repos:
  - owner/repo
review:
  default_verdict: COMMENT
  instructions: "Focus on security and error handling."
`), 0644)
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadFromPath(cfgPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Review.Instructions != "Focus on security and error handling." {
		t.Errorf("expected instructions to be loaded, got %q", cfg.Review.Instructions)
	}
}

func TestLoadConfig_ReviewInstructions_OmittedByDefault(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(cfgPath, []byte("repos:\n  - owner/repo\n"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadFromPath(cfgPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Review.Instructions != "" {
		t.Errorf("expected empty instructions by default, got %q", cfg.Review.Instructions)
	}
}

func TestLoadConfig_InvalidDefaultVerdict(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(cfgPath, []byte("repos:\n  - owner/repo\nreview:\n  default_verdict: LGTM\n"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	_, err = LoadFromPath(cfgPath)
	if err == nil {
		t.Fatal("expected error for invalid default_verdict, got nil")
	}
}

func TestLoadConfig_IgnoreDraftsFalse(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(cfgPath, []byte("repos:\n  - owner/repo\npoll:\n  ignore_drafts: false\n"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadFromPath(cfgPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Poll.IgnoreDrafts == nil {
		t.Fatal("expected ignore_drafts to be set, got nil")
	}
	if *cfg.Poll.IgnoreDrafts {
		t.Error("expected ignore_drafts to remain false when explicitly set to false")
	}
}
