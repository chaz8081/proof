# Agent Team Design

**Date**: 2026-04-09
**Status**: Approved

## Overview

Define 7 reusable agent definitions in `.claude/agents/` covering the full SDLC lifecycle. Agents are self-contained (no skill dependencies), use beads for issue tracking, and can be dispatched individually or composed into runtime teams.

## Agents

### architect (Opus)
- Design & architecture decisions
- API surface design, data modeling, trade-off analysis
- Produces design docs, not code
- Consults existing code and docs before proposing changes

### implementer (Sonnet)
- Feature implementation following TDD
- Claims a beads issue, writes failing tests, implements, commits
- Stays within task scope — no speculative refactoring
- Reports what was built, tested, and any concerns

### reviewer (Sonnet)
- Code review for bugs, security, quality
- Reviews diffs against requirements (beads issue or plan)
- Checks OWASP top 10, error handling, naming, test quality
- Reports findings with file:line references, does NOT fix issues

### tester (Sonnet)
- Test strategy, coverage gaps, edge case identification
- Writes unit/integration tests for existing code
- Validates test quality (tests behavior, not implementation)
- Runs tests and reports results

### fixer (Sonnet)
- Bug diagnosis and repair
- Reads the bug report, reproduces, diagnoses root cause, fixes
- Follows systematic debugging: reproduce → hypothesize → verify → fix → test
- Minimal changes — fix the bug, don't refactor the neighborhood

### ops (Haiku)
- Build validation, linting, dependency checks
- Runs go build, go vet, go test, checks for issues
- Mechanical tasks: formatting, import organization, changelog generation
- Fast and cheap — use for pre-commit/pre-PR validation

### planner (Sonnet)
- Backlog triage, task breakdown, prioritization
- Reads beads issues, analyzes codebase, breaks epics into tasks
- Creates beads issues with descriptions, priorities, dependencies
- Does NOT implement — produces plans and issues only

## Shared Context (All Agents)

All agents receive:
- Project awareness: Go CLI tool, Copilot SDK, go-github, cobra
- Beads workflow: `bd ready`, `bd show`, `bd update --claim`, `bd close`
- Git discipline: commit early, descriptive messages, don't amend
- Scope discipline: do what was asked, no more

## Model Selection Rationale

- **Opus** (architect): Design mistakes compound. Worth the cost for decisions that shape the codebase.
- **Sonnet** (implementer, reviewer, tester, fixer, planner): Strong reasoning for code generation, review, and analysis. Best cost/quality balance.
- **Haiku** (ops): Mechanical validation tasks. Speed and cost matter more than depth.

## File Structure

```
.claude/agents/
├── architect.md
├── implementer.md
├── reviewer.md
├── tester.md
├── fixer.md
├── ops.md
└── planner.md
```

## Usage Patterns

**Solo**: `"Have the fixer tackle proof-0gh"`
**Pair**: `"Have the implementer build proof-sia, then the reviewer check it"`
**Team**: `"Create a team with planner, implementer, tester, and reviewer to clear the P1 backlog"`
