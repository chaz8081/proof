// internal/cli/config_test.go
package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestConfigInitCmd_DelegatesToSetup(t *testing.T) {
	// config init is now an alias for the setup wizard.
	// Verify the command exists and its Use field is correct.
	cmd := newConfigInitCmd("/dev/null/config.yaml")
	if cmd.Use != "init" {
		t.Errorf("expected Use == 'init', got %q", cmd.Use)
	}
}

func TestConfigShowCmd_MissingFile(t *testing.T) {
	cmd := newConfigShowCmd("/nonexistent/path/config.yaml")
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when config file does not exist")
	}
	if !strings.Contains(err.Error(), "no config found") {
		t.Errorf("expected 'no config found' in error, got: %v", err)
	}
}
