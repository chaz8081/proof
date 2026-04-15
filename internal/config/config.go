// internal/config/config.go
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// RepoEntry represents a single repo in the repos list. It supports both simple
// string entries and extended map entries with optional per-repo instructions.
type RepoEntry struct {
	Name         string `yaml:"name"`
	Instructions string `yaml:"instructions,omitempty"`
}

// UnmarshalYAML handles both string and map formats in repos config.
func (r *RepoEntry) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind == yaml.ScalarNode {
		r.Name = value.Value
		return nil
	}
	type repoEntryAlias RepoEntry
	var alias repoEntryAlias
	if err := value.Decode(&alias); err != nil {
		return err
	}
	*r = RepoEntry(alias)
	return nil
}

type Config struct {
	Repos  []RepoEntry  `yaml:"repos"`
	Teams  []string     `yaml:"teams,omitempty"`
	Poll   PollConfig   `yaml:"poll,omitempty"`
	Review ReviewConfig `yaml:"review,omitempty"`
	Auth   AuthConfig   `yaml:"auth,omitempty"`
}

// RepoNames returns the repo name strings from the Repos slice, for use with
// callers that expect []string (e.g., FindReviewRequests).
func (c *Config) RepoNames() []string {
	names := make([]string, len(c.Repos))
	for i, r := range c.Repos {
		names[i] = r.Name
	}
	return names
}

// RepoInstructions returns per-repo instructions for a given owner/repo, or empty string.
func (c *Config) RepoInstructions(owner, repo string) string {
	fullName := owner + "/" + repo
	for _, r := range c.Repos {
		if r.Name == fullName {
			return r.Instructions
		}
	}
	return ""
}

type AuthConfig struct {
	Reviewer string `yaml:"reviewer,omitempty"` // GitHub account name for posting reviews
	Copilot  string `yaml:"copilot,omitempty"`  // GitHub account name with Copilot subscription
}

type PollConfig struct {
	IgnoreDrafts *bool `yaml:"ignore_drafts,omitempty"`
	IgnoreWIP    bool  `yaml:"ignore_wip,omitempty"`
	MaxFiles     int   `yaml:"max_files,omitempty"`
	MaxDiffBytes int   `yaml:"max_diff_bytes,omitempty"`
	IncludeOwn   bool  `yaml:"include_own,omitempty"`
}

type ReviewProfile struct {
	Instructions string `yaml:"instructions,omitempty"`
	SeverityMin  string `yaml:"severity_min,omitempty"` // nit, suggestion, issue, blocker
	MaxComments  int    `yaml:"max_comments,omitempty"`
}

type ReviewConfig struct {
	DefaultVerdict string                   `yaml:"default_verdict,omitempty"`
	Instructions   string                   `yaml:"instructions,omitempty"`
	Model          string                   `yaml:"model,omitempty"`
	Profiles       map[string]ReviewProfile `yaml:"profiles,omitempty"`
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

// ConfigDir returns the directory for config files.
// Uses $XDG_CONFIG_HOME/proof or ~/.config/proof.
func ConfigDir() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "proof")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "proof")
}

// DataDir returns the directory for data files (pending, history, learning).
// Uses $XDG_DATA_HOME/proof or ~/.local/share/proof.
func DataDir() string {
	if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
		return filepath.Join(xdg, "proof")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share", "proof")
}

func DefaultConfig() *Config {
	cfg := &Config{
		Repos: []RepoEntry{{Name: "owner/repo"}},
	}
	_ = applyDefaults(cfg) // defaults are always valid
	return cfg
}

func (c *Config) Validate() []string {
	var issues []string

	if len(c.Repos) == 0 {
		issues = append(issues, "no repos configured — add repos to watch for review requests")
	}

	for _, r := range c.Repos {
		if !strings.Contains(r.Name, "/") {
			issues = append(issues, fmt.Sprintf("invalid repo %q — expected owner/repo or org/*", r.Name))
		}
	}

	if c.Review.DefaultVerdict != "" {
		valid := map[string]bool{"APPROVE": true, "REQUEST_CHANGES": true, "COMMENT": true}
		if !valid[c.Review.DefaultVerdict] {
			issues = append(issues, fmt.Sprintf("invalid default_verdict %q — must be APPROVE, REQUEST_CHANGES, or COMMENT", c.Review.DefaultVerdict))
		}
	}

	if c.Poll.MaxFiles < 0 {
		issues = append(issues, "max_files cannot be negative")
	}

	if c.Poll.MaxDiffBytes < 0 {
		issues = append(issues, "max_diff_bytes cannot be negative")
	}

	return issues
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
