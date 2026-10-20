---
sidebar_position: 21
---

# project

Manage project folders in the global config and sync skills and agents into them, without creating a `.skillshare/` configuration in each folder.

`project` always uses the global config, even when run inside a project. This differs from `-p` / `--project`, which selects a repo's own configuration and is rejected by this command. Use [project mode](/docs/understand/project-skills) when teammates should receive the setup with the repo.

## Usage {#usage}

```bash
skillshare project                          # Same as project list
skillshare project list --json
skillshare project show <project> --json
skillshare project add <root> --target T [options]
skillshare project edit <project> [options]
skillshare project sync <project> [--dry-run] [--json]
skillshare project remove <project> [--yes] [--dry-run] [--json]
```

`add` requires an existing folder and at least one target. Other commands accept the effective project name (custom name or folder basename) or its root path. Prefix relative paths with `./` or `../` to select a root explicitly. If a bare token matches one project's name and another project's root, the command reports both candidates instead of choosing one. Symlink aliases stay separate registrations.

## Flags {#flags}

| Flag | Applies to | Description |
|------|------------|-------------|
| `--target T` | add, edit | Target tools; repeat or use a comma list. Required for add; replaces the entire list on edit |
| `--name N` | add, edit | Custom name; `--name=` restores the folder basename |
| `--skills`, `--no-skills` | add, edit | Enable or disable skills; enabled by default on add |
| `--agents`, `--no-agents` | add, edit | Enable or disable agents; disabled by default on add |
| `--skills-mode MODE`, `--agents-mode MODE` | add, edit | `merge`, `symlink`, or `copy`; empty value clears the override |
| `--skills-naming NAMING` | add, edit | `flat`, `standard`, or `prefixed`; empty value inherits |
| `--skills-include P`, `--skills-exclude P` | add, edit | Repeat for multiple glob patterns; replaces the corresponding list; empty value clears it |
| `--agents-include P`, `--agents-exclude P` | add, edit | Same replacement semantics for agents; filters require merge or copy mode |
| `--sync` | add, edit | Save, then sync this project's skills and agents; force is off |
| `--dry-run`, `-n` | add, edit, sync, remove | Preview without writing or prompting |
| `--yes` | remove | Skip confirmation; required for removal with JSON unless previewing |
| `--json` | all | Structured output; check the process exit code as well |
| `--global`, `-g` | all | Explicit global scope (also the default for this command) |
| `--help`, `-h` | all | Show help |

Resource settings also enable a disabled resource. Editing changes only supplied settings; enabling an existing resource keeps its settings. Opposing toggles such as `--skills --no-skills` are rejected. There is no agents naming flag. Unlike [`target`](/docs/reference/commands/target), these include/exclude flags replace lists rather than add or remove individual patterns.

## Examples {#examples}

### Register, preview, and sync {#register-and-sync}

```bash
skillshare project add ~/work/shop-web --target claude,codex \
  --skills-mode copy --skills-include 'frontend-*' --agents --dry-run
skillshare project add ~/work/shop-web --target claude,codex \
  --skills-mode copy --skills-include 'frontend-*' --agents --sync
skillshare project show shop-web --json
```

The saved entry belongs to [`projects`](/docs/reference/targets/configuration#projects). Shared tool folders are grouped into one sync target. A normal `skillshare sync` also syncs registered projects.

### Replace filters or reset overrides {#edit-settings}

```bash
skillshare project edit shop-web --skills-include 'frontend-*' --skills-include 'core-*'
skillshare project edit shop-web --skills-exclude= --skills-mode= --name=
skillshare project sync shop-web --dry-run --json
skillshare project sync shop-web --json
```

Add and edit validate the full effective configuration before writing. YAML anchors can affect other entries; mutation results list every affected root. A no-op edit returns `applied: false` and `unchanged: true`.

### Remove a registration {#remove}

```bash
skillshare project remove shop-web --dry-run --json
skillshare project remove shop-web --yes --json
```

Removal detaches skills before removing the registration. A whole-folder link is removed; merge-mode links become local copies while their source content exists; copy-mode files remain and their manifest is removed. Agent files remain in place. The preview and result list removed, copied-back, and retained paths.

## JSON and failures {#json-and-failures}

| Command | Main fields |
|---------|-------------|
| list | `projects` array (empty when none are registered) |
| show | `project` object |
| add, edit, remove | `action`, `project`, `applied`, `affected`, `warnings`; optional `sync` or `detach` |
| sync | `action`, `project`, `sync`; previews also have `applied: false` |

Project objects include `root` (stored spelling), `path` (expanded path), `name`, `targets`, `skills`, `agents`, `groups`, `missing`, and `hasOwnConfig`. Disabled resources are `null`. The sync result includes `results`, `failed`, `warnings`, and discovery/filter diagnostics.

A failed target makes sync exit 1 while reporting completed targets. If add/edit saves successfully but its sync fails, the registration stays saved; fix the target and retry with `project sync`. A missing project folder can still be edited, but cannot be synced.

Removal is not transactional: a later detach or save failure can leave earlier filesystem changes in place. Its JSON result reports `detach`, `registration` (`kept` or `removed`), and `error`; inspect those fields before retrying. Failures before any changes use an error payload. List, show, and previews avoid startup migrations and other maintenance writes; JSON output stays free of migration notices.

This command manages skills and agents only. MCP project settings and hooks use their own commands; it does not convert targets or edit a project's own config.

## Related {#related}

- [Many Projects, One Config](/docs/how-to/recipes/many-projects-one-config)
- [Project configuration fields](/docs/reference/targets/configuration#projects)
- [MCP project settings](/docs/reference/commands/mcp#manage-several-projects-from-the-global-config)
