# Proof — Design Document

**Date**: 2026-04-09
**Status**: Approved

## Overview

Proof is a Go CLI tool that pre-reviews GitHub PRs using AI (GitHub Copilot SDK), creates pending reviews on GitHub for human curation, and lets you submit when ready. The name is a triple pun: proofing (baking), proofreading, and proof (verification).

**Tagline**: "Let it rise before you bake it in."

## Core Workflow

1. **Detect**: Poll GitHub for PRs where you're a requested reviewer (or a team you're on), scoped to configured repos
2. **Analyze**: Fetch PR diff + metadata, send to Copilot SDK for structured code review
3. **Post**: Create a PENDING review on GitHub with inline comments (visible only to you)
4. **Curate**: Review and edit comments in GitHub's UI — full diff context, threading, suggestion blocks
5. **Submit**: Submit the pending review via CLI or GitHub UI

## Architecture

```
proof poll
    ├── Query GitHub for PRs where you're a requested reviewer
    ├── For each new PR:
    │   ├── Fetch diff + PR metadata via go-github
    │   ├── Send to Copilot SDK for structured analysis
    │   └── Create a PENDING review on GitHub with inline comments
    └── Notify (terminal output / optional webhook)

proof list
    └── Query GitHub for your pending reviews across watched repos

proof submit <owner/repo#number>
    └── Submit the pending review (APPROVE / REQUEST_CHANGES / COMMENT)

proof config
    └── Show/edit YAML config
```

## Key Design Decisions

### GitHub-Native Reviews (No Local Drafts)

Instead of storing drafts as local markdown files, Proof creates PENDING reviews directly on GitHub. Pending reviews are only visible to the creator — no one else sees the comments until submission.

**Why**: Eliminates local draft storage, markdown format contracts, and parsing complexity. You get GitHub's full review UX (inline diffs, suggestion blocks, threading) for free.

**Trade-off**: Requires a browser to curate reviews (no offline/terminal-only editing). Acceptable for MVP.

### Go with Copilot SDK

- **Language**: Go — strong GitHub API ecosystem (go-github, cobra), user preference
- **AI Backend**: GitHub Copilot SDK (`github/copilot-sdk/go`, public preview April 2026)
- **Future**: Ollama support via BYOK (Copilot SDK supports OpenAI-compatible providers) or a second `Reviewer` implementation

### Swappable Reviewer Interface

```go
type Reviewer interface {
    Review(ctx context.Context, pr PRContext) (*ReviewResult, error)
}
```

One implementation now (Copilot SDK). Trivial to add Ollama later behind the same interface.

## CLI Commands

```bash
proof poll                          # Detect PRs, run AI review, create pending reviews
proof list                          # Show your pending reviews across watched repos
proof submit <owner/repo#number>    # Submit a pending review
proof submit <owner/repo#number> --approve|--request-changes|--comment
proof config                        # Show config
proof config init                   # Create default config file
```

## Configuration (`~/.proof/config.yaml`)

```yaml
repos:
  - owner/repo-a
  - owner/repo-b
  - org/*                # watch all repos in an org

teams:
  - org/my-team          # detect PRs requesting this team

poll:
  ignore_drafts: true    # skip draft PRs
  ignore_wip: true       # skip PRs with WIP in title
  max_files: 50          # skip massive PRs, flag for manual review

review:
  default_verdict: COMMENT
  severity_levels: [nit, suggestion, issue, blocker]

notifications:
  terminal: true
  # webhook: https://...  # future: slack/teams/etc
```

## Data Types

```go
type PRContext struct {
    Owner, Repo string
    Number      int
    Diff        string
    Description string
    Files       []string
}

type ReviewResult struct {
    Summary  string
    Verdict  string // APPROVE, REQUEST_CHANGES, COMMENT
    Comments []InlineComment
}

type InlineComment struct {
    Path       string
    Line       int
    Body       string
    Severity   string
    Suggestion string // optional suggested code
}
```

## Project Structure

```
proof/
├── cmd/
│   └── proof/
│       └── main.go
├── internal/
│   ├── config/          # YAML config loading
│   ├── github/          # PR detection, diff fetching, review posting
│   ├── review/          # AI review engine (Copilot SDK integration)
│   └── cli/             # Cobra command definitions
├── go.mod
├── go.sum
└── README.md
```

## Dependencies

| Library | Purpose |
|---|---|
| `github/copilot-sdk/go` | AI review generation |
| `google/go-github/v68` | GitHub API (PRs, diffs, reviews) |
| `spf13/cobra` | CLI framework |
| `gopkg.in/yaml.v3` | Config file parsing |

## Out of Scope (Future)

- Webhook-based real-time triggers
- Web UI
- Multi-user support
- Auto-posting without human review
- Ollama integration (design supports it, don't build yet)
- Notification webhooks (Slack/Teams)
