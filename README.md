# Proof

> Let it rise before you bake it in.

AI-assisted PR review with human-in-the-loop. Proof pre-reviews GitHub PRs using AI
(via GitHub Copilot SDK), creates pending reviews for you to curate in GitHub's UI,
then lets you submit when ready.

## Install

```bash
go install -tags=copilot github.com/chaz8081/proof/cmd/proof@latest
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

## Building

The Copilot SDK integration requires a build tag:

```bash
# With Copilot SDK (full functionality)
go build -tags=copilot -o proof ./cmd/proof

# Without Copilot SDK (poll --dry-run, list, submit still work)
go build -o proof ./cmd/proof
```
