# Proof

> Let it rise before you bake it in.

AI-assisted PR review with human-in-the-loop. Proof pre-reviews GitHub PRs using AI,
creates pending reviews for you to curate, then directs you to GitHub to submit when ready.

## Philosophy

Proof is a **review preparation** tool, not a review publishing tool. The CLI creates
pending reviews visible only to you. To submit a review, you must visit GitHub's UI
and explicitly approve, request changes, or comment. This ensures every published
review has a human behind it — the AI assists, you decide.

## Install

### From Release (recommended)

**Supported platforms:** macOS (arm64, amd64), Linux (amd64, arm64)

Download the latest binary from [Releases](https://github.com/chaz8081/proof/releases):

```bash
# macOS (Apple Silicon)
curl -sL $(gh release view --repo chaz8081/proof --json assets -q '.assets[] | select(.name | contains("darwin_arm64")) | .url') | tar xz
sudo mv proof /usr/local/bin/

# macOS (Intel)
curl -sL $(gh release view --repo chaz8081/proof --json assets -q '.assets[] | select(.name | contains("darwin_amd64")) | .url') | tar xz
sudo mv proof /usr/local/bin/

# Linux (amd64)
curl -sL $(gh release view --repo chaz8081/proof --json assets -q '.assets[] | select(.name | contains("linux_amd64")) | .url') | tar xz
sudo mv proof /usr/local/bin/
```

Or download directly from https://github.com/chaz8081/proof/releases/latest

### From Source

```bash
git clone https://github.com/chaz8081/proof.git
cd proof
make install
```

## Setup

```bash
# Guided setup wizard (recommended)
proof setup

# Or create a default config manually
proof config init
```

## Usage

```bash
# Review a specific PR directly
proof poll owner/repo#123

# Scan all configured repos for PRs needing review
proof poll

# Watch mode — poll every 5 minutes
proof poll --every 5m

# List only — don't generate reviews yet
proof poll --dry-run

# Show your pending reviews
proof list
proof list -o json              # machine-readable output

# Preview a pending review
proof show owner/repo#123
proof show owner/repo#123 -o json

# Delete a pending review
proof dismiss owner/repo#123

# Force re-review (delete existing + create fresh)
proof poll owner/repo#123 --re-review

# Curate a pending review in the terminal (keep/delete/skip each comment)
proof curate owner/repo#123

# Review history
proof log
proof log --pr owner/repo#42 --since 7d
proof log -o json

# Review metrics
proof stats
proof stats --since 30d

# Compare two reviews of the same PR
proof diff owner/repo#42

# Review with a profile
proof poll --profile quick           # bugs/blockers only
proof poll --profile thorough        # comprehensive review

# Include your own PRs
proof poll --include-own

# Batch mode (skip interactive selection)
proof poll --batch
```

## How It Works

1. `proof poll` finds PRs and generates AI reviews as **pending drafts**
2. You curate the review — edit, delete, or keep comments
3. Visit GitHub to review the pending comments and submit when ready

## Configuration

`~/.proof/config.yaml`:

```yaml
repos:
  - owner/repo-a
  - myorg/* # all repos in an org

teams:
  - myorg/my-team

poll:
  ignore_drafts: true
  ignore_wip: true
  max_files: 50

review:
  default_verdict: COMMENT
```

## Configuration Guide

This section walks through every configuration option in `~/.proof/config.yaml`.

### Quick Start

The only required field is `repos`. Everything else has a sensible default.

```yaml
# Minimal config — just add your repos
repos:
  - owner/repo
```

Run `proof config init` to generate a starter file, then open it in your editor.

---

### Authentication

By default, proof resolves credentials automatically via `gh auth token` — no `auth` block required for single-account use.

**Credential resolution order:**

| Purpose         | Sources checked (in order)                                                                      |
| --------------- | ----------------------------------------------------------------------------------------------- |
| Posting reviews | `GITHUB_TOKEN` env var → `gh auth token --user <reviewer>` → `gh auth token`                    |
| Copilot / AI    | `PROOF_COPILOT_TOKEN` env var → `gh auth token --user <copilot>` → falls back to reviewer token |

```yaml
# Single account (default — no auth block needed)
# Uses the active gh account for everything

# Dual-account setup
auth:
  copilot: work-account # Account with Copilot subscription
  reviewer: personal-account # Account that posts reviews
```

Tokens are resolved at runtime via `gh auth token` — no secrets stored in config. You can also override with environment variables:

```bash
export GITHUB_TOKEN=ghp_yyy    # override reviewer token
```

> **Note:** `GITHUB_TOKEN` env var takes precedence over account-based resolution.

---

### Repos

Repos can be listed in two formats: **simple string** or **extended map**.

```yaml
repos:
  # Simple — owner/repo string.
  # Automatically picks up .github/PULL_REQUEST_TEMPLATE.md or
  # .github/copilot-instructions.md from the repo as review context.
  - owner/repo-a

  # Wildcard — watch all repos in an org where you're a requested reviewer
  - myorg/*

  # Extended — add per-repo review instructions
  - name: owner/repo-b
    instructions: |
      This is a financial services repo.
      Flag any hardcoded credentials or PII exposure.
      Prefer structured logging over fmt.Println.
```

Both formats can be mixed freely in the same list.

---

### Poll Settings

Controls which PRs proof considers when scanning.

```yaml
poll:
  ignore_drafts: true # Skip draft PRs (default: true)
  ignore_wip: true # Skip PRs with "WIP" in the title (default: false)
  include_own: false # Include your own PRs in batch scan (default: false)
  max_files: 50 # Skip PRs that touch more than N files (0 = no limit)
  max_diff_bytes: 500000 # Skip PRs whose diff exceeds N bytes (0 = no limit)
```

**Field reference:**

| Field            | Type | Default | Description                                        |
| ---------------- | ---- | ------- | -------------------------------------------------- |
| `ignore_drafts`  | bool | `true`  | Skip PRs marked as draft                           |
| `ignore_wip`     | bool | `false` | Skip PRs with "WIP" anywhere in the title          |
| `include_own`    | bool | `false` | Include PRs you authored in batch scans            |
| `max_files`      | int  | `0`     | Max changed-file count; PRs above this are skipped |
| `max_diff_bytes` | int  | `0`     | Max diff size in bytes; PRs above this are skipped |

---

### Review Settings

Controls what the AI produces and how proof presents it.

```yaml
review:
  default_verdict: COMMENT # APPROVE, REQUEST_CHANGES, or COMMENT (default: COMMENT)
  model: gpt-4.1 # AI model to use (default: gpt-4.1)
  instructions: | # Global instructions appended to the AI prompt
    Prefer table-driven tests over individual test functions.
    Flag any use of fmt.Println in production code.
    Always check for missing error handling.
```

**Field reference:**

| Field             | Default   | Description                                                                       |
| ----------------- | --------- | --------------------------------------------------------------------------------- |
| `default_verdict` | `COMMENT` | Verdict applied when submitting. Options: `APPROVE`, `REQUEST_CHANGES`, `COMMENT` |
| `model`           | `gpt-4.1` | AI model. Supported: `gpt-4.1`, `gpt-4.1-mini`, `gpt-5-mini`, `claude-haiku-4.5`  |
| `instructions`    | _(none)_  | Free-form text appended to the AI prompt for every review                         |

> **Tip:** Per-repo `instructions` (under `repos`) are merged with the global `review.instructions` for that repo's reviews.

---

### Teams

Monitor PRs that request a review from a GitHub team — not just your individual account.

```yaml
teams:
  - myorg/backend-team # Any PR requesting this team's review will be picked up
  - myorg/security-team
```

---

### Full Annotated Config

```yaml
# ~/.proof/config.yaml

# ── Repos ───────────────────────────────────────────────────────────────────
repos:
  - owner/repo-a # simple format
  - myorg/* # all repos in an org
  - name: owner/repo-b # extended format with per-repo instructions
    instructions: |
      Security-sensitive service. Flag PII, hardcoded secrets, and
      missing input validation on all external inputs.

# ── Teams ───────────────────────────────────────────────────────────────────
teams:
  - myorg/backend-team

# ── Poll ────────────────────────────────────────────────────────────────────
poll:
  ignore_drafts: true # skip draft PRs
  ignore_wip: true # skip PRs with WIP in title
  include_own: false # don't include your own PRs in batch scans
  max_files: 50 # skip PRs touching > 50 files
  max_diff_bytes: 500000 # skip PRs with diffs > ~500 KB

# ── Review ──────────────────────────────────────────────────────────────────
review:
  default_verdict: COMMENT # safe default — you decide before submitting
  model: gpt-4.1
  instructions: |
    Prefer table-driven tests.
    Flag any use of fmt.Println in production code.
    Check for missing context propagation in Go code.

# ── Auth (optional) ─────────────────────────────────────────────────────────
# Authentication (optional — uses active gh account by default)
# auth:
#   reviewer: personal-account  # Account that posts reviews
#   copilot: work-account        # Account with Copilot subscription
```

---

### Common Scenarios

#### Solo Developer

You're reviewing your own work or want to preview AI feedback on your PRs before merging.

```yaml
repos:
  - yourname/my-project

poll:
  include_own: true # include PRs you authored

review:
  default_verdict: COMMENT
```

```bash
proof poll --dry-run          # see which PRs would be picked up
proof poll yourname/my-project#42   # review a specific PR
```

#### Team Member

You're on a team and want proof to pick up all PRs where your team — or you directly — is a requested reviewer.

```yaml
repos:
  - myorg/*

teams:
  - myorg/backend-team

poll:
  ignore_drafts: true
  ignore_wip: true

review:
  default_verdict: COMMENT
  instructions: |
    Follow our internal Go style guide.
    Prefer errors.Is/As over direct comparison.
```

#### Dual-Account Setup (Work Copilot + Personal Reviewer)

Your Copilot subscription is on a work GitHub account, but you want reviews posted from your personal account.

```yaml
repos:
  - myorg/backend

auth:
  copilot: work-account # has Copilot subscription
  reviewer: personal-account # posts reviews as you

review:
  default_verdict: COMMENT
```

Tokens are resolved at runtime via `gh auth token --user <name>` — no secrets stored in config. Make sure both accounts are logged in via `gh auth login`.

#### Security-Focused Repo

Use per-repo instructions to give the AI targeted security guidance for a sensitive codebase.

```yaml
repos:
  - name: myorg/payments-service
    instructions: |
      This service handles payment processing and PCI-scoped data.
      Flag any: hardcoded credentials or API keys, PII logged to stdout,
      missing input validation on external inputs, SQL queries built
      with string concatenation, and any use of math/rand instead of
      crypto/rand for security-sensitive operations.

  - myorg/other-repo # regular repos can coexist

review:
  default_verdict: REQUEST_CHANGES # be conservative for this setup
  model: gpt-4.1
```

## Building

The Copilot SDK integration requires a build tag:

```bash
# With Copilot SDK (full functionality)
go build -tags=copilot -o proof ./cmd/proof

# Without Copilot SDK (poll --dry-run, list still work)
go build -o proof ./cmd/proof

# Build with version info
go build -tags=copilot -ldflags "-X github.com/chaz8081/proof/internal/cli.Version=v1.0.0 -X github.com/chaz8081/proof/internal/cli.Commit=$(git rev-parse --short HEAD) -X github.com/chaz8081/proof/internal/cli.BuildDate=$(date -u +%Y-%m-%dT%H:%M:%SZ)" -o proof ./cmd/proof
```

# Troubleshooting

➜ shop-notice git:(CR-12084) ✗ proof poll
Found 4 PR(s) requesting your review:

1. [NEW] getbread/kyc#287 — CR-11949 - support callback endpoint (by @hamster-bread)
2. [NEW] getbread/fraud#1130 — CR-11949 - support interface to accept id upload results from KYC (by @hamster-bread)
3. [NEW] getbread/fraud#1129 — JUST: 11832 fix wrong order (by @john-slota-bread)
4. [NEW] getbread/fraud#1092 — remove unused go mod (by @john-slota-bread)

Select PRs to review (e.g., 1,3 or 1-4 or all) [default: new only]: 2

Reviewing getbread/fraud#1130...
Warning: error during AI review: copilot review request: waiting for session.idle: context deadline exceeded
