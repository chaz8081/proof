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
	if cfg.Repos[0].Name != "owner/repo-a" {
		t.Errorf("expected 'owner/repo-a', got %q", cfg.Repos[0].Name)
	}
	if cfg.Repos[1].Name != "myorg/*" {
		t.Errorf("expected 'myorg/*', got %q", cfg.Repos[1].Name)
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
	if len(cfg.Repos) != 1 || cfg.Repos[0].Name != "owner/repo" {
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

func TestLoadConfig_ModelDefault(t *testing.T) {
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
	if cfg.Review.Model != "gpt-4.1" {
		t.Errorf("expected default model 'gpt-4.1', got %q", cfg.Review.Model)
	}
}

func TestLoadConfig_ModelFromYAML(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(cfgPath, []byte(`
repos:
  - owner/repo
review:
  model: gpt-4o
`), 0644)
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadFromPath(cfgPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Review.Model != "gpt-4o" {
		t.Errorf("expected model 'gpt-4o', got %q", cfg.Review.Model)
	}
}

func TestLoadConfig_MaxDiffBytes(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(cfgPath, []byte(`
repos:
  - owner/repo
poll:
  max_diff_bytes: 524288
`), 0644)
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadFromPath(cfgPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Poll.MaxDiffBytes != 524288 {
		t.Errorf("expected max_diff_bytes 524288, got %d", cfg.Poll.MaxDiffBytes)
	}
}

func TestLoadConfig_MaxDiffBytes_DefaultsToZero(t *testing.T) {
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
	if cfg.Poll.MaxDiffBytes != 0 {
		t.Errorf("expected max_diff_bytes to default to 0 (no limit), got %d", cfg.Poll.MaxDiffBytes)
	}
}

func TestValidate_NoIssues(t *testing.T) {
	cfg := &Config{
		Repos: []RepoEntry{{Name: "owner/repo"}, {Name: "myorg/*"}},
		Review: ReviewConfig{
			DefaultVerdict: "COMMENT",
		},
		Poll: PollConfig{
			MaxFiles:     10,
			MaxDiffBytes: 1024,
		},
	}
	issues := cfg.Validate()
	if len(issues) != 0 {
		t.Errorf("expected no issues, got: %v", issues)
	}
}

func TestValidate_EmptyRepos(t *testing.T) {
	cfg := &Config{
		Repos: []RepoEntry{},
	}
	issues := cfg.Validate()
	if len(issues) == 0 {
		t.Fatal("expected issue for empty repos, got none")
	}
	found := false
	for _, issue := range issues {
		if issue == "no repos configured — add repos to watch for review requests" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected empty repos message, got: %v", issues)
	}
}

func TestValidate_InvalidRepoFormat(t *testing.T) {
	cfg := &Config{
		Repos: []RepoEntry{{Name: "validorg/repo"}, {Name: "noslash"}},
	}
	issues := cfg.Validate()
	if len(issues) == 0 {
		t.Fatal("expected issue for invalid repo format, got none")
	}
	found := false
	for _, issue := range issues {
		if issue == `invalid repo "noslash" — expected owner/repo or org/*` {
			found = true
		}
	}
	if !found {
		t.Errorf("expected invalid repo message for 'noslash', got: %v", issues)
	}
}

func TestValidate_InvalidDefaultVerdict(t *testing.T) {
	cfg := &Config{
		Repos: []RepoEntry{{Name: "owner/repo"}},
		Review: ReviewConfig{
			DefaultVerdict: "LGTM",
		},
	}
	issues := cfg.Validate()
	if len(issues) == 0 {
		t.Fatal("expected issue for invalid verdict, got none")
	}
	found := false
	for _, issue := range issues {
		if issue == `invalid default_verdict "LGTM" — must be APPROVE, REQUEST_CHANGES, or COMMENT` {
			found = true
		}
	}
	if !found {
		t.Errorf("expected invalid verdict message, got: %v", issues)
	}
}

func TestValidate_NegativeMaxFiles(t *testing.T) {
	cfg := &Config{
		Repos: []RepoEntry{{Name: "owner/repo"}},
		Poll:  PollConfig{MaxFiles: -1},
	}
	issues := cfg.Validate()
	if len(issues) == 0 {
		t.Fatal("expected issue for negative max_files, got none")
	}
	found := false
	for _, issue := range issues {
		if issue == "max_files cannot be negative" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected negative max_files message, got: %v", issues)
	}
}

func TestValidate_NegativeMaxDiffBytes(t *testing.T) {
	cfg := &Config{
		Repos: []RepoEntry{{Name: "owner/repo"}},
		Poll:  PollConfig{MaxDiffBytes: -100},
	}
	issues := cfg.Validate()
	if len(issues) == 0 {
		t.Fatal("expected issue for negative max_diff_bytes, got none")
	}
	found := false
	for _, issue := range issues {
		if issue == "max_diff_bytes cannot be negative" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected negative max_diff_bytes message, got: %v", issues)
	}
}

func TestValidate_MultipleIssues(t *testing.T) {
	cfg := &Config{
		Repos: []RepoEntry{},
		Poll:  PollConfig{MaxFiles: -1, MaxDiffBytes: -1},
	}
	issues := cfg.Validate()
	if len(issues) < 3 {
		t.Errorf("expected at least 3 issues, got %d: %v", len(issues), issues)
	}
}

func TestValidate_EmptyVerdictIsSkipped(t *testing.T) {
	cfg := &Config{
		Repos: []RepoEntry{{Name: "owner/repo"}},
		Review: ReviewConfig{
			DefaultVerdict: "",
		},
	}
	issues := cfg.Validate()
	if len(issues) != 0 {
		t.Errorf("expected no issues for empty verdict (no validation), got: %v", issues)
	}
}

func TestLoadConfig_ModelNotOverriddenWhenExplicit(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(cfgPath, []byte(`
repos:
  - owner/repo
review:
  model: o4-mini
`), 0644)
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadFromPath(cfgPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Review.Model != "o4-mini" {
		t.Errorf("expected model 'o4-mini' to be preserved, got %q", cfg.Review.Model)
	}
}

// --- RepoEntry / mixed YAML format tests ---

func TestLoadConfig_MixedRepoFormats(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(cfgPath, []byte(`
repos:
  - owner/repo-a
  - name: owner/repo-b
    instructions: |
      Focus on security in this repo.
      Flag any hardcoded credentials.
  - myorg/*
`), 0644)
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadFromPath(cfgPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(cfg.Repos) != 3 {
		t.Fatalf("expected 3 repos, got %d", len(cfg.Repos))
	}
	if cfg.Repos[0].Name != "owner/repo-a" {
		t.Errorf("expected Repos[0].Name = 'owner/repo-a', got %q", cfg.Repos[0].Name)
	}
	if cfg.Repos[0].Instructions != "" {
		t.Errorf("expected Repos[0].Instructions empty, got %q", cfg.Repos[0].Instructions)
	}
	if cfg.Repos[1].Name != "owner/repo-b" {
		t.Errorf("expected Repos[1].Name = 'owner/repo-b', got %q", cfg.Repos[1].Name)
	}
	if cfg.Repos[1].Instructions == "" {
		t.Error("expected Repos[1].Instructions to be non-empty")
	}
	if cfg.Repos[2].Name != "myorg/*" {
		t.Errorf("expected Repos[2].Name = 'myorg/*', got %q", cfg.Repos[2].Name)
	}
}

func TestRepoNames(t *testing.T) {
	cfg := &Config{
		Repos: []RepoEntry{
			{Name: "owner/repo-a"},
			{Name: "owner/repo-b", Instructions: "Check security."},
			{Name: "myorg/*"},
		},
	}
	names := cfg.RepoNames()
	if len(names) != 3 {
		t.Fatalf("expected 3 names, got %d", len(names))
	}
	if names[0] != "owner/repo-a" {
		t.Errorf("expected names[0] = 'owner/repo-a', got %q", names[0])
	}
	if names[1] != "owner/repo-b" {
		t.Errorf("expected names[1] = 'owner/repo-b', got %q", names[1])
	}
	if names[2] != "myorg/*" {
		t.Errorf("expected names[2] = 'myorg/*', got %q", names[2])
	}
}

func TestRepoInstructions_Found(t *testing.T) {
	cfg := &Config{
		Repos: []RepoEntry{
			{Name: "owner/repo-a"},
			{Name: "owner/repo-b", Instructions: "Focus on security.\n"},
		},
	}
	got := cfg.RepoInstructions("owner", "repo-b")
	if got != "Focus on security.\n" {
		t.Errorf("expected per-repo instructions, got %q", got)
	}
}

func TestRepoInstructions_NotFound(t *testing.T) {
	cfg := &Config{
		Repos: []RepoEntry{
			{Name: "owner/repo-a"},
		},
	}
	got := cfg.RepoInstructions("owner", "repo-z")
	if got != "" {
		t.Errorf("expected empty string for unknown repo, got %q", got)
	}
}

func TestRepoInstructions_NoInstructions(t *testing.T) {
	cfg := &Config{
		Repos: []RepoEntry{
			{Name: "owner/repo-a"},
		},
	}
	got := cfg.RepoInstructions("owner", "repo-a")
	if got != "" {
		t.Errorf("expected empty string for repo with no instructions, got %q", got)
	}
}
