// internal/config/config.go
package config

import (
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
	IgnoreDrafts bool `yaml:"ignore_drafts,omitempty"`
	IgnoreWIP    bool `yaml:"ignore_wip,omitempty"`
	MaxFiles     int  `yaml:"max_files,omitempty"`
}

type ReviewConfig struct {
	DefaultVerdict string `yaml:"default_verdict,omitempty"`
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

	applyDefaults(cfg)
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
	applyDefaults(cfg)
	return cfg
}

func applyDefaults(cfg *Config) {
	if cfg.Review.DefaultVerdict == "" {
		cfg.Review.DefaultVerdict = "COMMENT"
	}
	if !cfg.Poll.IgnoreDrafts {
		cfg.Poll.IgnoreDrafts = true
	}
}
