# Changelog

All notable changes to Proof are documented here.

## [Unreleased]

### Added
- **Agent team** — 7 SDLC agent definitions (architect, implementer, reviewer, tester, fixer, planner, ops) for parallel development workflows
- **Pending review store** — `~/.proof/pending.json` tracks reviews created by `proof poll` so `proof list` finds them even after GitHub removes the review-requested status. Store interface allows future backends (beads, SQLite, etc.)
- **Copilot SDK integration** — end-to-end AI review working with `gpt-4.1` model via GitHub Copilot SDK v0.2.1
- **Team review support** — `FindReviewRequests` now searches for `team-review-requested:` in addition to `review-requested:@me`, with deduplication
- **Shorthand verdict flags** — `proof submit --approve` and `--request-changes` as alternatives to `--verdict`
- **Config-driven default verdict** — `review.default_verdict` in config is used when no `--verdict` flag is passed

### Fixed
- `proof poll` no longer creates duplicate pending reviews when run multiple times
- `IgnoreDrafts` config now uses `*bool` so `ignore_drafts: false` is respected
- `truncate` function is rune-safe for multi-byte UTF-8 characters
- Replaced custom `containsStr` test helpers with `strings.Contains`
- Search results capped at `PerPage: 100` to avoid silent truncation

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
