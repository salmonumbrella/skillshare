---
sidebar_position: 7
---

# AI-Assisted Development

> Use AI coding agents to contribute to skillshare with pre-built project skills.

## Prerequisites

- An AI coding agent (Claude Code, Codex, etc.)
- The skillshare repository cloned locally

## Setup

The repo ships project-mode skills in `.skillshare/skills/`. Sync them to your agent:

```bash
skillshare sync -p
```

Your AI agent now has access to specialized skills for working on this codebase.

## Available Skills

| Skill | What it does |
|-------|-------------|
| `skillshare-implement-feature` | Implement a feature from a spec file or description using TDD workflow |
| `skillshare-update-docs` | Update website docs to match recent code changes, cross-validating every flag against source |
| `skillshare-code-review` | Review a change in parallel passes (correctness, repo rules, spec, consumers, cleanup) and verify each finding |
| `skillshare-cli-e2e-test` | Run isolated E2E tests in devcontainer from runbooks |
| `skillshare-changelog` | Generate a CHANGELOG.md entry from recent commits in conventional format |

## Typical Workflow

1. **Start a feature** — ask your agent to use `skillshare-implement-feature` with a spec
2. **Update docs** — after code changes, invoke `skillshare-update-docs` to sync website docs
3. **Review the change** — run `skillshare-code-review` before committing or opening a PR
4. **Run E2E tests** — use `skillshare-cli-e2e-test` to verify in a sandbox
5. **Write changelog** — invoke `skillshare-changelog` before release

## What's Next?

- [Dev Containers setup →](/docs/learn/with-devcontainer)
- [Interactive Playground →](/docs/learn/with-playground)
- [Contributing guide →](https://github.com/runkids/skillshare/blob/main/CONTRIBUTING.md)
