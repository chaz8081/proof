// internal/cli/validate_test.go
package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigValidateCmd_ValidConfig(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(cfgPath, []byte(`
repos:
  - owner/repo
review:
  default_verdict: COMMENT
`), 0644)
	if err != nil {
		t.Fatal(err)
	}

	cmd := newConfigValidateCmd(cfgPath)
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Config is valid") {
		t.Errorf("expected 'Config is valid', got: %q", out)
	}
}

func TestConfigValidateCmd_EmptyRepos(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(cfgPath, []byte("repos: []\n"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	cmd := newConfigValidateCmd(cfgPath)
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "no repos configured") {
		t.Errorf("expected 'no repos configured' in output, got: %q", out)
	}
}

func TestConfigValidateCmd_InvalidRepoFormat(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(cfgPath, []byte("repos:\n  - noslash\n"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	cmd := newConfigValidateCmd(cfgPath)
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "noslash") {
		t.Errorf("expected invalid repo message for 'noslash', got: %q", out)
	}
}

func TestConfigValidateCmd_MissingConfigFile(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "nonexistent.yaml")

	cmd := newConfigValidateCmd(cfgPath)
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing config file, got nil")
	}
}

func TestConfigValidateCmd_NegativeMaxFiles(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(cfgPath, []byte("repos:\n  - owner/repo\npoll:\n  max_files: -5\n"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	cmd := newConfigValidateCmd(cfgPath)
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "max_files cannot be negative") {
		t.Errorf("expected 'max_files cannot be negative', got: %q", out)
	}
}
