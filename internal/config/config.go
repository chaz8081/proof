// internal/config/config.go
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Repos  []string     `yaml:"repos"`
	Teams  []string     `yaml:"teams,omitempty"`
	Poll   PollConfig   `yaml:"poll,omitempty"`
	Review ReviewConfig `yaml:"review,omitempty"`
}

type PollConfig struct {
	IgnoreDrafts *bool `yaml:"ignore_drafts,omitempty"`
	IgnoreWIP    bool  `yaml:"ignore_wip,omitempty"`
	MaxFiles     int   `yaml:"max_files,omitempty"`
	MaxDiffBytes int   `yaml:"max_diff_bytes,omitempty"`
}

type ReviewConfig struct {
	DefaultVerdict string `yaml:"default_verdict,omitempty"`
	Instructions   string `yaml:"instructions,omitempty"`
	Model          string `yaml:"model,omitempty"`
}

func LoadFromPath(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	cfg := &Config{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	if err := applyDefaults(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func Load() (*Config, error) {
	return LoadFromPath(filepath.Join(ConfigDir(), "config.yaml"))
}

func ConfigDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".proof")
}

func DefaultConfig() *Config {
	cfg := &Config{
		Repos: []string{"owner/repo"},
	}
	_ = applyDefaults(cfg) // defaults are always valid
	return cfg
}

func boolPtr(v bool) *bool { return &v }

var validVerdicts = map[string]bool{
	"APPROVE":         true,
	"REQUEST_CHANGES": true,
	"COMMENT":         true,
}

func applyDefaults(cfg *Config) error {
	if cfg.Review.DefaultVerdict == "" {
		cfg.Review.DefaultVerdict = "COMMENT"
	} else if !validVerdicts[cfg.Review.DefaultVerdict] {
		return fmt.Errorf("invalid default_verdict %q — must be APPROVE, REQUEST_CHANGES, or COMMENT", cfg.Review.DefaultVerdict)
	}
	if cfg.Review.Model == "" {
		cfg.Review.Model = "gpt-4.1"
	}
	if cfg.Poll.IgnoreDrafts == nil {
		cfg.Poll.IgnoreDrafts = boolPtr(true)
	}
	return nil
}
