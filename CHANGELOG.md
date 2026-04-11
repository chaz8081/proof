# Changelog

All notable changes to Proof are documented here.

## [Unreleased]

## [1.2.0] — 2026-04-10

### Added
- **`proof curate` command** — interactive terminal-based review curation. Walk through each comment with keep/delete/skip, then submit — no browser needed.
- **Repo-level `.proof.yaml` config** — teams check a `.proof.yaml` into their repo to define review standards. Discovered via GitHub Contents API and merged with user config (user config takes precedence).
- **Curation learning** — tracks deltas between AI-generated reviews and what users actually submit in `~/.proof/learning.jsonl`. Records comments kept vs deleted and verdict changes for quality analysis.
- **Diff-aware re-review** — `--re-review` now sends only the incremental diff (changes since last review) with previous summary as context, producing focused delta reviews instead of full re-reviews.

### Fixed
- `FetchRepoConfig` now distinguishes 404 (no file) from real errors (auth, network) instead of silently swallowing all failures
- `proof curate` validates verdict input before submitting to prevent GitHub 422 errors
- `saveLearningDelta` creates `~/.proof/` directory if it doesn't exist
- SHA display uses `shortSHA()` helper to prevent panic on malformed pending records
- Learning deltas are now recorded from both `proof submit` and `proof curate` (was only submit)

## [1.1.0] — 2026-04-10

### Added
- **Dual-account support** — separate tokens for Copilot SDK auth (`auth.copilot_token` / `PROOF_COPILOT_TOKEN`) and GitHub API reviewer identity (`auth.github_token`). Use one GitHub account for AI access and another for posting reviews.
- **Interactive PR selection** — `proof poll` now shows a numbered list of PRs with status tags (`[NEW]` / `[PENDING]`) and lets you select which to review. Default behavior for multi-PR polling. Use `--batch` for the old non-interactive mode.
- **Self-review mode** — `proof poll --include-own` includes your own PRs in the batch scan. Also configurable via `poll.include_own` in config. Single-PR mode (`proof poll owner/repo#123`) already worked for self-review.
- **Per-repo instruction overrides** — repos config now supports extended format with per-repo review instructions that override global `review.instructions`:
  ```yaml
  repos:
    - owner/repo-a                    # uses .github/ instructions from repo
    - name: owner/repo-b              # with custom instructions
      instructions: |
        Focus on security in this repo.
    - myorg/*                          # wildcard
  ```
- **Per-repo instructions wired into review flow** — `cfg.RepoInstructions(owner, repo)` overrides global instructions in both single-PR and batch poll paths.

### Architecture Decisions
- **Interactive as default** — interactive PR selection is the default for batch polls. Watch mode (`--every`), `--dry-run`, `--batch`, and single-PR mode bypass it automatically. Keeps the simple case simple while giving power users control.
- **Per-repo override > global** — per-repo `instructions` in config take precedence over `review.instructions`. Both are still augmented by `.github/` repo instruction files fetched from GitHub.
- **Dual-account is opt-in** — if not configured, both paths use the same token (backward compatible). Separate tokens only matter when the Copilot subscriber and reviewer are different accounts.

## [1.0.0] — 2026-04-10

### Added
- **Agent team** — 7 SDLC agent definitions (architect, implementer, reviewer, tester, fixer, planner, ops) for parallel development workflows
- **Pending review store** — `~/.proof/pending.json` tracks reviews created by `proof poll` so `proof list` finds them even after GitHub removes the review-requested status. Store interface allows future backends (beads, SQLite, etc.)
- **Copilot SDK integration** — end-to-end AI review working with `gpt-4.1` model via GitHub Copilot SDK v0.2.1
- **Team review support** — `FindReviewRequests` now searches for `team-review-requested:` in addition to `review-requested:@me`, with deduplication
- **Shorthand verdict flags** — `proof submit --approve` and `--request-changes` as alternatives to `--verdict`
- **Config-driven default verdict** — `review.default_verdict` in config is used when no `--verdict` flag is passed
- **Custom review instructions** — `review.instructions` config field appends custom guidance to the AI system prompt (e.g., project-specific conventions)
- **Model selection** — `review.model` config field + `proof poll --model` flag; defaults to `gpt-4.1`, overridable per-run
- **Auto-detect GITHUB_TOKEN** — falls back to `gh auth token` when env var is not set
- **Verdict validation** — rejects invalid verdict values before sending to GitHub API
- **Diff size guardrail** — `poll.max_diff_bytes` config skips oversized PRs to prevent timeouts
- **`proof dismiss` command** — delete a pending review from GitHub and clean up local store
- **Out-of-hunk line filtering** — comments with invalid line numbers are filtered before review creation, preventing GitHub 422 errors
- **`proof poll --re-review`** — force fresh AI review on PRs that already have a pending review (deletes existing, creates new)
- **`proof config validate`** — validates config file and reports issues (empty repos, invalid verdicts, negative values)
- **Integration test harness** — shared `testutil` package with `NewTestClient`, fixture loading, and mock helpers for cross-package testing
- **Repo instruction discovery** — automatically fetches `.github/copilot-instructions.md`, `.github/instructions/*.instructions.md` (with glob matching), and `AGENTS.md` from target repos and injects into the AI review prompt
- **`proof show` command** — terminal preview of pending review with summary and inline comments before submitting
- **`--output json` flag** — machine-readable JSON output on `proof list` and `proof show` for scripting
- **E2E smoke test** — full workflow integration test (search → fetch → review → create) with httptest mocks
- **Single-PR mode** — `proof poll owner/repo#123` reviews a specific PR directly, bypassing search
- **Watch mode** — `proof poll --every 5m` polls repeatedly at a configurable interval (Ctrl+C to stop)
- **`proof version` command** — prints version, commit hash, and build date (set via ldflags)
- **Rate limit awareness** — checks GitHub API limits before polling; warns at low remaining, waits at zero
- **Improved `proof config init`** — detects GitHub username via `gh`, generates commented YAML with helpful defaults and next-steps
- **Concurrent `proof list`** — goroutine-per-PR with semaphore (max 5 concurrent), local store as fast path
- **Release automation** — Goreleaser config + GitHub Actions workflow for tagged releases (linux/darwin, amd64/arm64)

### Fixed
- `proof poll` no longer creates duplicate pending reviews when run multiple times
- `IgnoreDrafts` config now uses `*bool` so `ignore_drafts: false` is respected
- `truncate` function is rune-safe for multi-byte UTF-8 characters
- Replaced custom `containsStr` test helpers with `strings.Contains`
- Search results capped at `PerPage: 100` to avoid silent truncation
- Improved error messages across all commands with actionable guidance
- Removed dead code from ratelimit.go (125→43 lines)

### Architecture Decisions
- **GitHub-native reviews over local drafts** — pending reviews on GitHub eliminate local markdown storage, parsing contracts, and the need for an editor integration. Trade-off: requires browser for curation. Decision validated by end-to-end testing.
- **Copilot SDK as AI transport** — uses `SystemMessage.Mode: "replace"` with our own review prompt. SDK provides model access and auth; we control the review logic entirely.
- **Store interface for pending tracking** — `Store` with `Add/List/Remove` allows swapping from JSON file to beads or SQLite. Cleanup is passive (during `list` and `submit`), no background GC.

## [0.1.0] — 2026-04-09

### Added
- Initial MVP: `proof poll`, `proof list`, `proof submit`, `proof config`
- GitHub client with PR detection, diff fetching, pending review CRUD
- Copilot SDK reviewer with JSON response parsing
- YAML configuration with repo/team/poll/review settings
- Cobra CLI framework
