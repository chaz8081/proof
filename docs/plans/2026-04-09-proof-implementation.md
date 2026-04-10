# Proof Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build a Go CLI tool that AI-reviews GitHub PRs via Copilot SDK and creates pending reviews for human curation.

**Architecture:** Cobra CLI → internal packages (config, github, review) → Copilot SDK + go-github. The `review.Reviewer` interface decouples AI from GitHub operations. Pending reviews on GitHub replace local drafts.

**Tech Stack:** Go, `github/copilot-sdk/go`, `google/go-github/v68`, `spf13/cobra`, `gopkg.in/yaml.v3`

**Design doc:** `docs/plans/2026-04-09-proof-design.md`

---

### Task 1: Project Scaffolding

**Files:**
- Create: `go.mod`
- Create: `.gitignore`
- Create: `cmd/proof/main.go`
- Create: `internal/config/config.go` (empty package)
- Create: `internal/github/github.go` (empty package)
- Create: `internal/review/review.go` (empty package)
- Create: `internal/cli/root.go` (empty package)

**Step 1: Initialize Go module**

Run: `go mod init github.com/chaz8081/proof`

**Step 2: Create .gitignore**

```
# Binaries
proof
*.exe
*.dll
*.so
*.dylib

# Test
*.test
*.out
cover.out

# IDE
.idea/
.vscode/
*.swp

# OS
.DS_Store
```

**Step 3: Create minimal main.go**

```go
// cmd/proof/main.go
package main

import (
	"fmt"
	"os"

	"github.com/chaz8081/proof/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
```

**Step 4: Create root command stub**

```go
// internal/cli/root.go
package cli

import "github.com/spf13/cobra"

var rootCmd = &cobra.Command{
	Use:   "proof",
	Short: "AI-assisted PR review with human-in-the-loop",
	Long:  `Proof pre-reviews GitHub PRs using AI, creates pending reviews for you to curate, then lets you submit when ready. Let it rise before you bake it in.`,
}

func Execute() error {
	return rootCmd.Execute()
}
```

**Step 5: Create empty package files**

```go
// internal/config/config.go
package config
```

```go
// internal/github/github.go
package github
```

```go
// internal/review/review.go
package review
```

**Step 6: Install dependencies and verify build**

Run: `go get github.com/spf13/cobra && go build ./...`
Expected: Clean build, no errors.

**Step 7: Verify CLI runs**

Run: `go run ./cmd/proof --help`
Expected: Shows "AI-assisted PR review with human-in-the-loop" help text.

**Step 8: Commit**

```bash
git add go.mod go.sum .gitignore cmd/ internal/
git commit -m "scaffold: project structure with cobra CLI skeleton"
```

---

### Task 2: Configuration Loading

**Files:**
- Create: `internal/config/config.go`
- Create: `internal/config/config_test.go`

**Step 1: Write the failing tests**

```go
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
	if !cfg.Poll.IgnoreDrafts {
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
	if cfg.Poll.IgnoreDrafts != true {
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
```

**Step 2: Run tests to verify they fail**

Run: `go test ./internal/config/ -v`
Expected: FAIL — functions not defined.

**Step 3: Write the implementation**

```go
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
		// Only set default if not explicitly provided.
		// Since zero value of bool is false and we want default true,
		// we set it here. The YAML will override if present.
		cfg.Poll.IgnoreDrafts = true
	}
}
```

Note: The `IgnoreDrafts` default-true logic has a limitation — YAML `false` and "not set" are indistinguishable with a bare `bool`. This is acceptable for MVP since skipping drafts is almost always desired. If needed later, switch to `*bool`.

**Step 4: Install yaml dependency and run tests**

Run: `go get gopkg.in/yaml.v3 && go test ./internal/config/ -v`
Expected: All PASS.

**Step 5: Commit**

```bash
git add internal/config/ go.mod go.sum
git commit -m "feat: config loading from YAML with defaults"
```

---

### Task 3: Review Types and Interface

**Files:**
- Modify: `internal/review/review.go`
- Create: `internal/review/review_test.go`

**Step 1: Write the tests**

```go
// internal/review/review_test.go
package review

import (
	"context"
	"testing"
)

// mockReviewer verifies the interface is implementable.
type mockReviewer struct {
	result *ReviewResult
	err    error
}

func (m *mockReviewer) Review(_ context.Context, _ PRContext) (*ReviewResult, error) {
	return m.result, m.err
}

func TestReviewerInterface(t *testing.T) {
	mock := &mockReviewer{
		result: &ReviewResult{
			Summary: "Looks good overall",
			Verdict: "APPROVE",
			Comments: []InlineComment{
				{
					Path:     "main.go",
					Line:     42,
					Body:     "Consider using a constant here",
					Severity: "nit",
				},
			},
		},
	}

	var r Reviewer = mock
	result, err := r.Review(context.Background(), PRContext{
		Owner:       "chaz8081",
		Repo:        "proof",
		Number:      1,
		Title:       "Add feature",
		Description: "This PR adds a feature",
		Diff:        "diff --git a/main.go...",
		Files:       []string{"main.go"},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Summary != "Looks good overall" {
		t.Errorf("unexpected summary: %q", result.Summary)
	}
	if result.Verdict != "APPROVE" {
		t.Errorf("unexpected verdict: %q", result.Verdict)
	}
	if len(result.Comments) != 1 {
		t.Fatalf("expected 1 comment, got %d", len(result.Comments))
	}
	if result.Comments[0].Line != 42 {
		t.Errorf("expected line 42, got %d", result.Comments[0].Line)
	}
}

func TestInlineComment_FormattedBody(t *testing.T) {
	tests := []struct {
		name     string
		comment  InlineComment
		contains string
	}{
		{
			name: "with severity and suggestion",
			comment: InlineComment{
				Path:       "main.go",
				Line:       10,
				Body:       "Use errors.New instead",
				Severity:   "suggestion",
				Suggestion: "return errors.New(\"failed\")",
			},
			contains: "```suggestion",
		},
		{
			name: "without suggestion",
			comment: InlineComment{
				Path:     "main.go",
				Line:     10,
				Body:     "Nice refactor",
				Severity: "nit",
			},
			contains: "[nit]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := tt.comment.FormattedBody()
			if body == "" {
				t.Fatal("expected non-empty body")
			}
			if !contains(body, tt.contains) {
				t.Errorf("expected body to contain %q, got %q", tt.contains, body)
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./internal/review/ -v`
Expected: FAIL — types not defined.

**Step 3: Write the implementation**

```go
// internal/review/review.go
package review

import (
	"context"
	"fmt"
	"strings"
)

// Reviewer generates a structured code review from a PR context.
type Reviewer interface {
	Review(ctx context.Context, pr PRContext) (*ReviewResult, error)
}

type PRContext struct {
	Owner       string
	Repo        string
	Number      int
	Title       string
	Description string
	Diff        string
	Files       []string
}

type ReviewResult struct {
	Summary  string          `json:"summary"`
	Verdict  string          `json:"verdict"`
	Comments []InlineComment `json:"comments"`
}

type InlineComment struct {
	Path       string `json:"path"`
	Line       int    `json:"line"`
	Body       string `json:"body"`
	Severity   string `json:"severity"`
	Suggestion string `json:"suggestion,omitempty"`
}

// FormattedBody returns the comment body formatted for GitHub,
// with severity tag and optional suggestion block.
func (c InlineComment) FormattedBody() string {
	var b strings.Builder

	fmt.Fprintf(&b, "**[%s]** %s", c.Severity, c.Body)

	if c.Suggestion != "" {
		fmt.Fprintf(&b, "\n\n```suggestion\n%s\n```", c.Suggestion)
	}

	return b.String()
}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./internal/review/ -v`
Expected: All PASS.

**Step 5: Commit**

```bash
git add internal/review/
git commit -m "feat: define Reviewer interface and review types"
```

---

### Task 4: GitHub Client — PR Detection

**Files:**
- Modify: `internal/github/github.go`
- Create: `internal/github/github_test.go`

This task creates the GitHub client wrapper that finds PRs where the authenticated user is a requested reviewer, filtered by the configured repo list.

**Step 1: Write the failing tests**

```go
// internal/github/github_test.go
package github

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	gh "github.com/google/go-github/v68/github"
)

func testClient(t *testing.T, mux *http.ServeMux) *Client {
	t.Helper()
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	ghClient := gh.NewClient(nil)
	ghClient.BaseURL, _ = ghClient.BaseURL.Parse(server.URL + "/")

	return &Client{gh: ghClient}
}

func TestFindReviewRequests_FiltersByRepo(t *testing.T) {
	mux := http.NewServeMux()

	// Mock the search endpoint
	mux.HandleFunc("/search/issues", func(w http.ResponseWriter, r *http.Request) {
		result := &gh.IssuesSearchResult{
			Total: gh.Ptr(2),
			Issues: []*gh.Issue{
				{
					Number:            gh.Ptr(1),
					Title:             gh.Ptr("Add feature"),
					RepositoryURL:     gh.Ptr("https://api.github.com/repos/owner/repo-a"),
					PullRequestLinks: &gh.PullRequestLinks{URL: gh.Ptr("https://api.github.com/repos/owner/repo-a/pulls/1")},
					User:              &gh.User{Login: gh.Ptr("alice")},
					Draft:             gh.Ptr(false),
				},
				{
					Number:            gh.Ptr(5),
					Title:             gh.Ptr("Fix bug"),
					RepositoryURL:     gh.Ptr("https://api.github.com/repos/owner/repo-b"),
					PullRequestLinks: &gh.PullRequestLinks{URL: gh.Ptr("https://api.github.com/repos/owner/repo-b/pulls/5")},
					User:              &gh.User{Login: gh.Ptr("bob")},
					Draft:             gh.Ptr(false),
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	})

	client := testClient(t, mux)
	prs, err := client.FindReviewRequests(context.Background(), []string{"owner/repo-a", "owner/repo-b"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(prs) != 2 {
		t.Fatalf("expected 2 PRs, got %d", len(prs))
	}
	if prs[0].Number != 1 || prs[0].Owner != "owner" || prs[0].Repo != "repo-a" {
		t.Errorf("unexpected first PR: %+v", prs[0])
	}
}

func TestFindReviewRequests_SkipsDrafts(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/search/issues", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("q")
		// Verify the query includes draft:false
		if !containsStr(q, "draft:false") {
			t.Errorf("expected query to filter drafts, got: %s", q)
		}
		result := &gh.IssuesSearchResult{Total: gh.Ptr(0), Issues: []*gh.Issue{}}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	})

	client := testClient(t, mux)
	prs, err := client.FindReviewRequests(context.Background(), []string{"owner/repo"}, WithIgnoreDrafts(true))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(prs) != 0 {
		t.Errorf("expected 0 PRs, got %d", len(prs))
	}
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
```

**Step 2: Run tests to verify they fail**

Run: `go get github.com/google/go-github/v68 && go test ./internal/github/ -v`
Expected: FAIL — Client type not defined.

**Step 3: Write the implementation**

```go
// internal/github/github.go
package github

import (
	"context"
	"fmt"
	"strings"

	gh "github.com/google/go-github/v68/github"
	"golang.org/x/oauth2"
)

type Client struct {
	gh *gh.Client
}

func NewClient(token string) *Client {
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	tc := oauth2.NewClient(context.Background(), ts)
	return &Client{gh: gh.NewClient(tc)}
}

// PRInfo is a lightweight representation of a PR needing review.
type PRInfo struct {
	Owner  string
	Repo   string
	Number int
	Title  string
	Author string
	Draft  bool
}

type FindOptions struct {
	IgnoreDrafts bool
	IgnoreWIP    bool
}

type FindOption func(*FindOptions)

func WithIgnoreDrafts(v bool) FindOption {
	return func(o *FindOptions) { o.IgnoreDrafts = v }
}

func WithIgnoreWIP(v bool) FindOption {
	return func(o *FindOptions) { o.IgnoreWIP = v }
}

// FindReviewRequests searches for open PRs where the authenticated user
// is a requested reviewer, filtered to the given repos.
func (c *Client) FindReviewRequests(ctx context.Context, repos []string, opts ...FindOption) ([]PRInfo, error) {
	options := &FindOptions{}
	for _, opt := range opts {
		opt(options)
	}

	query := "is:open is:pr review-requested:@me"
	if options.IgnoreDrafts {
		query += " draft:false"
	}

	// Build repo filter
	if len(repos) > 0 {
		var repoFilters []string
		for _, r := range repos {
			if strings.HasSuffix(r, "/*") {
				org := strings.TrimSuffix(r, "/*")
				repoFilters = append(repoFilters, fmt.Sprintf("org:%s", org))
			} else {
				repoFilters = append(repoFilters, fmt.Sprintf("repo:%s", r))
			}
		}
		query += " " + strings.Join(repoFilters, " ")
	}

	result, _, err := c.gh.Search.Issues(ctx, query, &gh.SearchOptions{
		Sort:  "updated",
		Order: "desc",
	})
	if err != nil {
		return nil, fmt.Errorf("searching for review requests: %w", err)
	}

	var prs []PRInfo
	for _, issue := range result.Issues {
		owner, repo := parseRepoURL(issue.GetRepositoryURL())
		pr := PRInfo{
			Owner:  owner,
			Repo:   repo,
			Number: issue.GetNumber(),
			Title:  issue.GetTitle(),
			Author: issue.GetUser().GetLogin(),
			Draft:  issue.GetDraft(),
		}

		if options.IgnoreWIP && strings.Contains(strings.ToLower(pr.Title), "wip") {
			continue
		}

		prs = append(prs, pr)
	}

	return prs, nil
}

// parseRepoURL extracts owner/repo from a GitHub API repository URL.
// e.g., "https://api.github.com/repos/owner/repo" → "owner", "repo"
func parseRepoURL(url string) (string, string) {
	parts := strings.Split(url, "/")
	if len(parts) >= 2 {
		return parts[len(parts)-2], parts[len(parts)-1]
	}
	return "", ""
}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./internal/github/ -v`
Expected: All PASS.

**Step 5: Commit**

```bash
git add internal/github/ go.mod go.sum
git commit -m "feat: GitHub client with PR review request detection"
```

---

### Task 5: GitHub Client — Diff Fetching

**Files:**
- Modify: `internal/github/github.go`
- Modify: `internal/github/github_test.go`

**Step 1: Write the failing test**

Add to `internal/github/github_test.go`:

```go
func TestGetPRDiff(t *testing.T) {
	mux := http.NewServeMux()

	mux.HandleFunc("/repos/owner/repo/pulls/1", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Accept") == "application/vnd.github.diff" {
			w.Write([]byte("diff --git a/main.go b/main.go\n"))
			return
		}
		pr := &gh.PullRequest{
			Number: gh.Ptr(1),
			Title:  gh.Ptr("Add feature"),
			Body:   gh.Ptr("This adds a feature"),
			Head:   &gh.PullRequestBranch{Ref: gh.Ptr("feature")},
			Base:   &gh.PullRequestBranch{Ref: gh.Ptr("main")},
			User:   &gh.User{Login: gh.Ptr("alice")},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(pr)
	})

	mux.HandleFunc("/repos/owner/repo/pulls/1/files", func(w http.ResponseWriter, r *http.Request) {
		files := []*gh.CommitFile{
			{Filename: gh.Ptr("main.go")},
			{Filename: gh.Ptr("util.go")},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(files)
	})

	client := testClient(t, mux)
	prCtx, err := client.GetPRContext(context.Background(), "owner", "repo", 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if prCtx.Title != "Add feature" {
		t.Errorf("unexpected title: %q", prCtx.Title)
	}
	if prCtx.Diff == "" {
		t.Error("expected non-empty diff")
	}
	if len(prCtx.Files) != 2 {
		t.Errorf("expected 2 files, got %d", len(prCtx.Files))
	}
}
```

**Step 2: Run tests to verify the new test fails**

Run: `go test ./internal/github/ -v -run TestGetPRDiff`
Expected: FAIL — GetPRContext not defined.

**Step 3: Write the implementation**

Add to `internal/github/github.go`:

```go
import (
	"github.com/chaz8081/proof/internal/review"
)

// GetPRContext fetches all context needed for AI review: PR metadata, diff, and file list.
func (c *Client) GetPRContext(ctx context.Context, owner, repo string, number int) (*review.PRContext, error) {
	pr, _, err := c.gh.PullRequests.Get(ctx, owner, repo, number)
	if err != nil {
		return nil, fmt.Errorf("getting PR: %w", err)
	}

	diff, _, err := c.gh.PullRequests.GetRaw(ctx, owner, repo, number, gh.RawOptions{Type: gh.Diff})
	if err != nil {
		return nil, fmt.Errorf("getting diff: %w", err)
	}

	commitFiles, _, err := c.gh.PullRequests.ListFiles(ctx, owner, repo, number, nil)
	if err != nil {
		return nil, fmt.Errorf("listing files: %w", err)
	}

	files := make([]string, len(commitFiles))
	for i, f := range commitFiles {
		files[i] = f.GetFilename()
	}

	return &review.PRContext{
		Owner:       owner,
		Repo:        repo,
		Number:      number,
		Title:       pr.GetTitle(),
		Description: pr.GetBody(),
		Diff:        diff,
		Files:       files,
	}, nil
}
```

**Step 4: Run tests**

Run: `go test ./internal/github/ -v`
Expected: All PASS.

**Step 5: Commit**

```bash
git add internal/github/
git commit -m "feat: fetch PR diff and metadata for review context"
```

---

### Task 6: GitHub Client — Create and Submit Pending Reviews

**Files:**
- Modify: `internal/github/github.go`
- Modify: `internal/github/github_test.go`

**Step 1: Write the failing tests**

Add to `internal/github/github_test.go`:

```go
func TestCreatePendingReview(t *testing.T) {
	mux := http.NewServeMux()

	var capturedBody map[string]any
	mux.HandleFunc("/repos/owner/repo/pulls/1/reviews", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		json.NewDecoder(r.Body).Decode(&capturedBody)

		review := &gh.PullRequestReview{
			ID:    gh.Ptr(int64(42)),
			State: gh.Ptr("PENDING"),
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(review)
	})

	client := testClient(t, mux)
	result := &review.ReviewResult{
		Summary: "Looks good with minor issues",
		Verdict: "COMMENT",
		Comments: []review.InlineComment{
			{
				Path:     "main.go",
				Line:     10,
				Body:     "Consider error handling",
				Severity: "issue",
			},
		},
	}

	reviewID, err := client.CreatePendingReview(context.Background(), "owner", "repo", 1, result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reviewID != 42 {
		t.Errorf("expected review ID 42, got %d", reviewID)
	}

	// Verify no event was sent (pending review)
	if event, ok := capturedBody["event"]; ok && event != "" {
		t.Errorf("expected no event for pending review, got %q", event)
	}
}

func TestSubmitReview(t *testing.T) {
	mux := http.NewServeMux()

	mux.HandleFunc("/repos/owner/repo/pulls/1/reviews/42/events", func(w http.ResponseWriter, r *http.Request) {
		review := &gh.PullRequestReview{
			ID:    gh.Ptr(int64(42)),
			State: gh.Ptr("APPROVED"),
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(review)
	})

	client := testClient(t, mux)
	err := client.SubmitReview(context.Background(), "owner", "repo", 1, 42, "APPROVE")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestListPendingReviews(t *testing.T) {
	mux := http.NewServeMux()

	mux.HandleFunc("/repos/owner/repo/pulls/1/reviews", func(w http.ResponseWriter, r *http.Request) {
		reviews := []*gh.PullRequestReview{
			{
				ID:    gh.Ptr(int64(42)),
				State: gh.Ptr("PENDING"),
				Body:  gh.Ptr("AI review summary"),
				User:  &gh.User{Login: gh.Ptr("chaz8081")},
			},
			{
				ID:    gh.Ptr(int64(43)),
				State: gh.Ptr("APPROVED"),
				Body:  gh.Ptr("LGTM"),
				User:  &gh.User{Login: gh.Ptr("alice")},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(reviews)
	})

	client := testClient(t, mux)
	reviews, err := client.ListPendingReviews(context.Background(), "owner", "repo", 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(reviews) != 1 {
		t.Fatalf("expected 1 pending review, got %d", len(reviews))
	}
	if reviews[0].ID != 42 {
		t.Errorf("expected review ID 42, got %d", reviews[0].ID)
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./internal/github/ -v -run "TestCreate|TestSubmit|TestList"`
Expected: FAIL — methods not defined.

**Step 3: Write the implementation**

Add to `internal/github/github.go`:

```go
// PendingReviewInfo represents a pending review on a PR.
type PendingReviewInfo struct {
	ID   int64
	Body string
	User string
}

// CreatePendingReview creates a PENDING review with inline comments.
// The review is only visible to the authenticated user until submitted.
func (c *Client) CreatePendingReview(ctx context.Context, owner, repo string, number int, result *review.ReviewResult) (int64, error) {
	var comments []*gh.DraftReviewComment
	for _, comment := range result.Comments {
		comments = append(comments, &gh.DraftReviewComment{
			Path: gh.Ptr(comment.Path),
			Line: gh.Ptr(comment.Line),
			Side: gh.Ptr("RIGHT"),
			Body: gh.Ptr(comment.FormattedBody()),
		})
	}

	r, _, err := c.gh.PullRequests.CreateReview(ctx, owner, repo, number, &gh.PullRequestReviewRequest{
		Body:     gh.Ptr(result.Summary),
		Comments: comments,
		// Event intentionally omitted — creates a PENDING review
	})
	if err != nil {
		return 0, fmt.Errorf("creating pending review: %w", err)
	}

	return r.GetID(), nil
}

// SubmitReview submits a pending review with the given verdict.
// Valid events: APPROVE, REQUEST_CHANGES, COMMENT
func (c *Client) SubmitReview(ctx context.Context, owner, repo string, number int, reviewID int64, event string) error {
	_, _, err := c.gh.PullRequests.SubmitReview(ctx, owner, repo, number, reviewID, &gh.PullRequestReviewRequest{
		Event: gh.Ptr(event),
	})
	if err != nil {
		return fmt.Errorf("submitting review: %w", err)
	}
	return nil
}

// ListPendingReviews returns all PENDING reviews on a PR.
func (c *Client) ListPendingReviews(ctx context.Context, owner, repo string, number int) ([]PendingReviewInfo, error) {
	reviews, _, err := c.gh.PullRequests.ListReviews(ctx, owner, repo, number, nil)
	if err != nil {
		return nil, fmt.Errorf("listing reviews: %w", err)
	}

	var pending []PendingReviewInfo
	for _, r := range reviews {
		if r.GetState() == "PENDING" {
			pending = append(pending, PendingReviewInfo{
				ID:   r.GetID(),
				Body: r.GetBody(),
				User: r.GetUser().GetLogin(),
			})
		}
	}
	return pending, nil
}
```

**Step 4: Run tests**

Run: `go test ./internal/github/ -v`
Expected: All PASS.

**Step 5: Commit**

```bash
git add internal/github/
git commit -m "feat: create pending reviews and submit with verdict"
```

---

### Task 7: Copilot SDK Reviewer Implementation

**Files:**
- Create: `internal/review/copilot.go`
- Create: `internal/review/copilot_test.go`

The Copilot SDK requires a running CLI process, so unit tests use the `Reviewer` interface mock. This task builds the real implementation and an integration test that can be run manually.

**Step 1: Write a test for JSON response parsing**

The Copilot reviewer sends a prompt and parses a JSON response. We can test the parsing logic independently.

```go
// internal/review/copilot_test.go
package review

import (
	"testing"
)

func TestParseReviewResponse(t *testing.T) {
	raw := `{
		"summary": "This PR adds a new endpoint. Overall looks clean.",
		"verdict": "COMMENT",
		"comments": [
			{
				"path": "handler.go",
				"line": 25,
				"body": "Missing error check on db.Query",
				"severity": "issue"
			},
			{
				"path": "handler.go",
				"line": 42,
				"body": "Consider using a constant",
				"severity": "nit",
				"suggestion": "const maxRetries = 3"
			}
		]
	}`

	result, err := parseReviewJSON(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Summary == "" {
		t.Error("expected non-empty summary")
	}
	if result.Verdict != "COMMENT" {
		t.Errorf("expected COMMENT verdict, got %q", result.Verdict)
	}
	if len(result.Comments) != 2 {
		t.Fatalf("expected 2 comments, got %d", len(result.Comments))
	}
	if result.Comments[0].Severity != "issue" {
		t.Errorf("expected 'issue' severity, got %q", result.Comments[0].Severity)
	}
	if result.Comments[1].Suggestion != "const maxRetries = 3" {
		t.Errorf("unexpected suggestion: %q", result.Comments[1].Suggestion)
	}
}

func TestParseReviewResponse_ExtractsJSON(t *testing.T) {
	// Models sometimes wrap JSON in markdown code blocks
	raw := "Here is my review:\n```json\n{\"summary\":\"LGTM\",\"verdict\":\"APPROVE\",\"comments\":[]}\n```\nLet me know if you need anything else."

	result, err := parseReviewJSON(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Verdict != "APPROVE" {
		t.Errorf("expected APPROVE, got %q", result.Verdict)
	}
}

func TestParseReviewResponse_InvalidJSON(t *testing.T) {
	_, err := parseReviewJSON("this is not json at all")
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./internal/review/ -v -run TestParse`
Expected: FAIL — parseReviewJSON not defined.

**Step 3: Write the parsing logic and Copilot reviewer**

```go
// internal/review/copilot.go
package review

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	copilot "github.com/github/copilot-sdk/go"
)

// CopilotReviewer implements Reviewer using the GitHub Copilot SDK.
type CopilotReviewer struct {
	client *copilot.Client
}

func NewCopilotReviewer() (*CopilotReviewer, error) {
	client := copilot.NewClient(&copilot.ClientOptions{
		LogLevel: "error",
	})

	return &CopilotReviewer{client: client}, nil
}

func (r *CopilotReviewer) Start(ctx context.Context) error {
	return r.client.Start(ctx)
}

func (r *CopilotReviewer) Stop() {
	r.client.Stop()
}

func (r *CopilotReviewer) Review(ctx context.Context, pr PRContext) (*ReviewResult, error) {
	session, err := r.client.CreateSession(ctx, &copilot.SessionConfig{
		SystemMessage: &copilot.SystemMessageConfig{
			Mode: "replace",
			Content: systemPrompt,
		},
		OnPermissionRequest: copilot.PermissionHandler.ApproveAll,
	})
	if err != nil {
		return nil, fmt.Errorf("creating copilot session: %w", err)
	}
	defer session.Disconnect()

	prompt := buildReviewPrompt(pr)

	var response string
	done := make(chan struct{})
	var reviewErr error

	session.On(func(event copilot.SessionEvent) {
		switch d := event.Data.(type) {
		case *copilot.AssistantMessageData:
			response = d.Content
		case *copilot.SessionIdleData:
			close(done)
		}
	})

	if _, err := session.Send(ctx, copilot.MessageOptions{Prompt: prompt}); err != nil {
		return nil, fmt.Errorf("sending review request: %w", err)
	}

	<-done
	if reviewErr != nil {
		return nil, reviewErr
	}

	return parseReviewJSON(response)
}

const systemPrompt = `You are a senior code reviewer. You will receive a pull request diff and metadata.
Analyze the code changes and respond with ONLY a JSON object (no markdown, no explanation) in this exact format:

{
  "summary": "2-3 sentence high-level assessment of the PR",
  "verdict": "APPROVE or REQUEST_CHANGES or COMMENT",
  "comments": [
    {
      "path": "path/to/file.go",
      "line": 42,
      "body": "Clear, actionable comment about this line",
      "severity": "nit|suggestion|issue|blocker",
      "suggestion": "optional replacement code for this line"
    }
  ]
}

Rules:
- Line numbers must reference lines in the NEW version of the file (right side of diff)
- severity levels: nit (style/preference), suggestion (improvement), issue (likely bug or problem), blocker (must fix)
- Only include suggestion field when you have a concrete code replacement
- Be concise and actionable. Don't restate what the code does — say what should change and why
- If the PR looks good, use verdict APPROVE with an empty or minimal comments array
- Focus on bugs, security issues, and logic errors over style`

func buildReviewPrompt(pr PRContext) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Pull Request: %s/%s#%d\n", pr.Owner, pr.Repo, pr.Number)
	fmt.Fprintf(&b, "**Title**: %s\n", pr.Title)
	if pr.Description != "" {
		fmt.Fprintf(&b, "**Description**: %s\n", pr.Description)
	}
	fmt.Fprintf(&b, "**Files changed**: %s\n\n", strings.Join(pr.Files, ", "))
	fmt.Fprintf(&b, "## Diff\n\n```diff\n%s\n```\n", pr.Diff)
	return b.String()
}

// parseReviewJSON extracts and parses a ReviewResult from the model's response.
// Handles both raw JSON and JSON wrapped in markdown code blocks.
func parseReviewJSON(raw string) (*ReviewResult, error) {
	jsonStr := extractJSON(raw)

	var result ReviewResult
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return nil, fmt.Errorf("parsing review JSON: %w (raw response: %.200s)", err, raw)
	}

	return &result, nil
}

var jsonBlockRe = regexp.MustCompile("(?s)```(?:json)?\\s*\\n?(\\{.*?\\})\\s*\\n?```")

func extractJSON(s string) string {
	// Try to extract from markdown code block first
	if matches := jsonBlockRe.FindStringSubmatch(s); len(matches) > 1 {
		return matches[1]
	}

	// Try to find raw JSON object
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start >= 0 && end > start {
		return s[start : end+1]
	}

	return s
}
```

**Step 4: Run tests**

Run: `go get github.com/github/copilot-sdk/go && go test ./internal/review/ -v`
Expected: All PASS.

Note: If `github.com/github/copilot-sdk/go` fails to resolve (preview access required), the implementation can be adjusted. The interface and parsing logic work independently.

**Step 5: Commit**

```bash
git add internal/review/
git commit -m "feat: Copilot SDK reviewer with structured JSON response parsing"
```

---

### Task 8: CLI — Config Commands

**Files:**
- Create: `internal/cli/config.go`
- Create: `internal/cli/config_test.go`

**Step 1: Write the tests**

```go
// internal/cli/config_test.go
package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestConfigInitCmd_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	cmd := newConfigInitCmd(cfgPath)
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := os.Stat(cfgPath); err != nil {
		t.Fatalf("config file not created: %v", err)
	}
}

func TestConfigInitCmd_DoesNotOverwrite(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	os.WriteFile(cfgPath, []byte("existing"), 0644)

	cmd := newConfigInitCmd(cfgPath)
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when config already exists")
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./internal/cli/ -v -run TestConfig`
Expected: FAIL — newConfigInitCmd not defined.

**Step 3: Write the implementation**

```go
// internal/cli/config.go
package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/chaz8081/proof/internal/config"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func init() {
	configCmd := &cobra.Command{
		Use:   "config",
		Short: "Show or manage configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfgPath := filepath.Join(config.ConfigDir(), "config.yaml")
			data, err := os.ReadFile(cfgPath)
			if err != nil {
				return fmt.Errorf("no config found — run 'proof config init' to create one")
			}
			cmd.Println(string(data))
			return nil
		},
	}

	cfgPath := filepath.Join(config.ConfigDir(), "config.yaml")
	configCmd.AddCommand(newConfigInitCmd(cfgPath))
	rootCmd.AddCommand(configCmd)
}

func newConfigInitCmd(cfgPath string) *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Create a default configuration file",
		RunE: func(cmd *cobra.Command, args []string) error {
			if _, err := os.Stat(cfgPath); err == nil {
				return fmt.Errorf("config already exists at %s", cfgPath)
			}

			if err := os.MkdirAll(filepath.Dir(cfgPath), 0755); err != nil {
				return fmt.Errorf("creating config directory: %w", err)
			}

			cfg := config.DefaultConfig()
			data, err := yaml.Marshal(cfg)
			if err != nil {
				return fmt.Errorf("marshaling config: %w", err)
			}

			if err := os.WriteFile(cfgPath, data, 0644); err != nil {
				return fmt.Errorf("writing config: %w", err)
			}

			cmd.Printf("Config created at %s\n", cfgPath)
			cmd.Println("Edit it to add your repos and teams.")
			return nil
		},
	}
}
```

**Step 4: Run tests**

Run: `go test ./internal/cli/ -v`
Expected: All PASS.

**Step 5: Commit**

```bash
git add internal/cli/
git commit -m "feat: config show and init CLI commands"
```

---

### Task 9: CLI — Poll Command

**Files:**
- Create: `internal/cli/poll.go`

This is the main orchestration command: detect PRs → AI review → create pending reviews. Since it wires together the GitHub client and the Copilot reviewer (both requiring real external services), this is tested via integration/manual testing rather than unit tests.

**Step 1: Write the implementation**

```go
// internal/cli/poll.go
package cli

import (
	"fmt"
	"os"

	"github.com/chaz8081/proof/internal/config"
	proofgh "github.com/chaz8081/proof/internal/github"
	"github.com/chaz8081/proof/internal/review"
	"github.com/spf13/cobra"
)

func init() {
	var dryRun bool

	pollCmd := &cobra.Command{
		Use:   "poll",
		Short: "Check for PRs needing review and generate AI draft reviews",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("loading config: %w\nRun 'proof config init' to create one", err)
			}

			token := os.Getenv("GITHUB_TOKEN")
			if token == "" {
				return fmt.Errorf("GITHUB_TOKEN not set — export your GitHub token or use 'gh auth token'")
			}

			ghClient := proofgh.NewClient(token)

			prs, err := ghClient.FindReviewRequests(ctx, cfg.Repos,
				proofgh.WithIgnoreDrafts(cfg.Poll.IgnoreDrafts),
				proofgh.WithIgnoreWIP(cfg.Poll.IgnoreWIP),
			)
			if err != nil {
				return fmt.Errorf("finding review requests: %w", err)
			}

			if len(prs) == 0 {
				cmd.Println("No PRs waiting for your review.")
				return nil
			}

			cmd.Printf("Found %d PR(s) requesting your review:\n\n", len(prs))
			for _, pr := range prs {
				cmd.Printf("  • %s/%s#%d — %s (by @%s)\n", pr.Owner, pr.Repo, pr.Number, pr.Title, pr.Author)
			}

			if dryRun {
				cmd.Println("\n(dry run — skipping AI review)")
				return nil
			}

			reviewer, err := review.NewCopilotReviewer()
			if err != nil {
				return fmt.Errorf("initializing reviewer: %w", err)
			}
			if err := reviewer.Start(ctx); err != nil {
				return fmt.Errorf("starting copilot: %w", err)
			}
			defer reviewer.Stop()

			cmd.Println()
			for _, pr := range prs {
				cmd.Printf("Reviewing %s/%s#%d...\n", pr.Owner, pr.Repo, pr.Number)

				if cfg.Poll.MaxFiles > 0 {
					prCtx, err := ghClient.GetPRContext(ctx, pr.Owner, pr.Repo, pr.Number)
					if err != nil {
						cmd.PrintErrf("  ⚠ Error fetching PR: %v\n", err)
						continue
					}
					if len(prCtx.Files) > cfg.Poll.MaxFiles {
						cmd.Printf("  ⚠ Skipping — %d files exceeds max_files (%d)\n", len(prCtx.Files), cfg.Poll.MaxFiles)
						continue
					}
				}

				prCtx, err := ghClient.GetPRContext(ctx, pr.Owner, pr.Repo, pr.Number)
				if err != nil {
					cmd.PrintErrf("  ⚠ Error fetching PR context: %v\n", err)
					continue
				}

				result, err := reviewer.Review(ctx, *prCtx)
				if err != nil {
					cmd.PrintErrf("  ⚠ Error during AI review: %v\n", err)
					continue
				}

				reviewID, err := ghClient.CreatePendingReview(ctx, pr.Owner, pr.Repo, pr.Number, result)
				if err != nil {
					cmd.PrintErrf("  ⚠ Error creating review: %v\n", err)
					continue
				}

				cmd.Printf("  ✓ Pending review created (ID: %d) — %d comments, verdict: %s\n",
					reviewID, len(result.Comments), result.Verdict)
				cmd.Printf("    View: https://github.com/%s/%s/pull/%d\n", pr.Owner, pr.Repo, pr.Number)
			}

			return nil
		},
	}

	pollCmd.Flags().BoolVar(&dryRun, "dry-run", false, "List PRs without generating reviews")
	rootCmd.AddCommand(pollCmd)
}
```

**Step 2: Verify it builds**

Run: `go build ./...`
Expected: Clean build.

**Step 3: Commit**

```bash
git add internal/cli/poll.go
git commit -m "feat: poll command — detect PRs and create pending AI reviews"
```

---

### Task 10: CLI — List Command

**Files:**
- Create: `internal/cli/list.go`

**Step 1: Write the implementation**

```go
// internal/cli/list.go
package cli

import (
	"fmt"
	"os"

	"github.com/chaz8081/proof/internal/config"
	proofgh "github.com/chaz8081/proof/internal/github"
	"github.com/spf13/cobra"
)

func init() {
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "Show your pending reviews across watched repos",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}

			token := os.Getenv("GITHUB_TOKEN")
			if token == "" {
				return fmt.Errorf("GITHUB_TOKEN not set")
			}

			ghClient := proofgh.NewClient(token)

			prs, err := ghClient.FindReviewRequests(ctx, cfg.Repos)
			if err != nil {
				return fmt.Errorf("finding review requests: %w", err)
			}

			found := false
			for _, pr := range prs {
				pending, err := ghClient.ListPendingReviews(ctx, pr.Owner, pr.Repo, pr.Number)
				if err != nil {
					cmd.PrintErrf("⚠ Error checking %s/%s#%d: %v\n", pr.Owner, pr.Repo, pr.Number, err)
					continue
				}
				if len(pending) > 0 {
					if !found {
						cmd.Println("Pending reviews:")
						found = true
					}
					for _, rev := range pending {
						cmd.Printf("  • %s/%s#%d (review ID: %d)\n", pr.Owner, pr.Repo, pr.Number, rev.ID)
						cmd.Printf("    %s\n", truncate(rev.Body, 80))
						cmd.Printf("    View: https://github.com/%s/%s/pull/%d\n", pr.Owner, pr.Repo, pr.Number)
					}
				}
			}

			if !found {
				cmd.Println("No pending reviews.")
			}

			return nil
		},
	}

	rootCmd.AddCommand(listCmd)
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}
```

**Step 2: Verify it builds**

Run: `go build ./...`
Expected: Clean build.

**Step 3: Commit**

```bash
git add internal/cli/list.go
git commit -m "feat: list command — show pending reviews across repos"
```

---

### Task 11: CLI — Submit Command

**Files:**
- Create: `internal/cli/submit.go`

**Step 1: Write the implementation**

```go
// internal/cli/submit.go
package cli

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	proofgh "github.com/chaz8081/proof/internal/github"
	"github.com/spf13/cobra"
)

func init() {
	var verdict string

	submitCmd := &cobra.Command{
		Use:   "submit <owner/repo#number>",
		Short: "Submit a pending review",
		Long:  "Submit a pending review to GitHub. Finds your pending review on the PR and submits it.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			owner, repo, number, err := parsePRRef(args[0])
			if err != nil {
				return err
			}

			token := os.Getenv("GITHUB_TOKEN")
			if token == "" {
				return fmt.Errorf("GITHUB_TOKEN not set")
			}

			ghClient := proofgh.NewClient(token)

			pending, err := ghClient.ListPendingReviews(ctx, owner, repo, number)
			if err != nil {
				return fmt.Errorf("listing pending reviews: %w", err)
			}
			if len(pending) == 0 {
				return fmt.Errorf("no pending review found on %s/%s#%d", owner, repo, number)
			}

			reviewID := pending[0].ID

			event := strings.ToUpper(verdict)
			if event == "" {
				event = "COMMENT"
			}

			if err := ghClient.SubmitReview(ctx, owner, repo, number, reviewID, event); err != nil {
				return fmt.Errorf("submitting review: %w", err)
			}

			cmd.Printf("Review submitted on %s/%s#%d as %s\n", owner, repo, number, event)
			cmd.Printf("View: https://github.com/%s/%s/pull/%d\n", owner, repo, number)
			return nil
		},
	}

	submitCmd.Flags().StringVar(&verdict, "verdict", "COMMENT", "Review verdict: APPROVE, REQUEST_CHANGES, or COMMENT")
	rootCmd.AddCommand(submitCmd)
}

// parsePRRef parses "owner/repo#123" into components.
func parsePRRef(ref string) (owner, repo string, number int, err error) {
	// Split on #
	parts := strings.SplitN(ref, "#", 2)
	if len(parts) != 2 {
		return "", "", 0, fmt.Errorf("invalid PR reference %q — expected owner/repo#number", ref)
	}

	number, err = strconv.Atoi(parts[1])
	if err != nil {
		return "", "", 0, fmt.Errorf("invalid PR number in %q: %w", ref, err)
	}

	repoParts := strings.SplitN(parts[0], "/", 2)
	if len(repoParts) != 2 {
		return "", "", 0, fmt.Errorf("invalid repo in %q — expected owner/repo", ref)
	}

	return repoParts[0], repoParts[1], number, nil
}
```

**Step 2: Write a test for parsePRRef**

```go
// Add to internal/cli/submit_test.go
package cli

import "testing"

func TestParsePRRef(t *testing.T) {
	tests := []struct {
		input       string
		wantOwner   string
		wantRepo    string
		wantNumber  int
		wantErr     bool
	}{
		{"owner/repo#123", "owner", "repo", 123, false},
		{"my-org/my-repo#1", "my-org", "my-repo", 1, false},
		{"bad-format", "", "", 0, true},
		{"owner/repo#abc", "", "", 0, true},
		{"noslash#123", "", "", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			owner, repo, number, err := parsePRRef(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if owner != tt.wantOwner || repo != tt.wantRepo || number != tt.wantNumber {
				t.Errorf("got (%s, %s, %d), want (%s, %s, %d)",
					owner, repo, number, tt.wantOwner, tt.wantRepo, tt.wantNumber)
			}
		})
	}
}
```

**Step 3: Run tests**

Run: `go test ./internal/cli/ -v`
Expected: All PASS.

**Step 4: Verify full build**

Run: `go build ./...`
Expected: Clean build.

**Step 5: Commit**

```bash
git add internal/cli/submit.go internal/cli/submit_test.go
git commit -m "feat: submit command — post pending review with verdict"
```

---

### Task 12: Integration Verification and README

**Files:**
- Create: `README.md`

**Step 1: Run all tests**

Run: `go test ./... -v`
Expected: All PASS.

**Step 2: Build the binary**

Run: `go build -o proof ./cmd/proof && ./proof --help`
Expected: Shows help with poll, list, submit, config subcommands.

**Step 3: Write README**

```markdown
# Proof

> Let it rise before you bake it in.

AI-assisted PR review with human-in-the-loop. Proof pre-reviews GitHub PRs using AI,
creates pending reviews for you to curate in GitHub's UI, then lets you submit when ready.

## Install

```bash
go install github.com/chaz8081/proof/cmd/proof@latest
```

## Setup

```bash
# Initialize config
proof config init

# Edit ~/.proof/config.yaml to add your repos
# Set your GitHub token
export GITHUB_TOKEN=$(gh auth token)
```

## Usage

```bash
# Check for PRs and generate AI reviews (creates pending reviews on GitHub)
proof poll

# List only — don't generate reviews yet
proof poll --dry-run

# Show your pending reviews
proof list

# Submit a pending review
proof submit owner/repo#123
proof submit owner/repo#123 --verdict APPROVE
proof submit owner/repo#123 --verdict REQUEST_CHANGES
```

## How It Works

1. `proof poll` finds PRs where you're a requested reviewer
2. Each PR's diff is analyzed by AI (via GitHub Copilot SDK)
3. A **pending review** is created on GitHub — visible only to you
4. You curate the review in GitHub's UI (edit, delete, add comments)
5. `proof submit` publishes the review, or submit directly in GitHub

## Configuration

`~/.proof/config.yaml`:

```yaml
repos:
  - owner/repo-a
  - myorg/*          # all repos in an org

teams:
  - myorg/my-team

poll:
  ignore_drafts: true
  ignore_wip: true
  max_files: 50

review:
  default_verdict: COMMENT
```
```

**Step 4: Commit**

```bash
git add README.md
git commit -m "docs: add README with install, setup, and usage instructions"
```

**Step 5: Final full test run**

Run: `go test ./... -v -count=1`
Expected: All PASS. Clean build.

---

## Summary

| Task | What | Tests |
|------|------|-------|
| 1 | Project scaffolding | Build verification |
| 2 | Config loading | 4 unit tests |
| 3 | Review types + interface | 3 unit tests |
| 4 | GitHub PR detection | 2 unit tests (httptest) |
| 5 | GitHub diff fetching | 1 unit test (httptest) |
| 6 | Pending review create/submit | 3 unit tests (httptest) |
| 7 | Copilot SDK reviewer | 3 unit tests (JSON parsing) |
| 8 | CLI config commands | 2 unit tests |
| 9 | CLI poll command | Build verification |
| 10 | CLI list command | Build verification |
| 11 | CLI submit command | 1 unit test (parsePRRef) |
| 12 | Integration + README | Full test suite run |
