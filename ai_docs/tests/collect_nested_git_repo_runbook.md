# Collect: Nested Git Repo Skill

## Scope

Verifies that `skillshare collect` correctly handles skills with nested directory
structures (e.g., a git-cloned multi-skill repo like `obra/superpowers` placed
directly in a target). Key behaviors:

- Nested files and directories are fully copied to source (not just empty dirs)
- `.git/` directory is skipped during collect (not copied to source)
- Flat file at root level is also collected

This targets a reported bug where `collect` produced only empty directories when
the skill contained nested subdirectories (and optionally a `.git/` dir).

## Environment

- Devcontainer with `ss` binary
- ssenv-isolated HOME
- No network required (uses local fixture data)

## Steps

### Step 1: Setup — create environment and target with a simulated git-cloned repo

```bash
# Clean any stale state
ss extras remove rules --force -g 2>/dev/null || true
rm -rf ~/.claude/rules 2>/dev/null || true

# Init skillshare with claude target only
ss init --no-copy --all-targets --no-git --no-skill --force 2>/dev/null || true

# Get source and target paths
SOURCE=$(ss status --json | jq -r '.source.path')
TARGET=$(ss status --json | jq -r '.targets[] | select(.name == "claude") | .path')

echo "SOURCE=$SOURCE"
echo "TARGET=$TARGET"

# Ensure target dir exists
mkdir -p "$TARGET"

# Create a simulated git-cloned multi-skill repo in the target
REPO="$TARGET/superpowers"
mkdir -p "$REPO/.git/objects/pack"
mkdir -p "$REPO/.git/refs/heads"
mkdir -p "$REPO/skills/brainstorming"
mkdir -p "$REPO/skills/debugging/prompts"
mkdir -p "$REPO/skills/tdd/templates/go"
mkdir -p "$REPO/commands/commit"
mkdir -p "$REPO/docs"

# .git metadata files
echo "ref: refs/heads/main" > "$REPO/.git/HEAD"
echo "[core]\n\trepositoryformatversion = 0" > "$REPO/.git/config"
echo "pack-data" > "$REPO/.git/objects/pack/pack-abc.pack"

# Skill files (nested 2-3 levels deep)
echo "# Brainstorming Skill" > "$REPO/skills/brainstorming/SKILL.md"
echo "---\nname: brainstorming\n---" > "$REPO/skills/brainstorming/SKILL.md"

echo "# Debugging Skill" > "$REPO/skills/debugging/SKILL.md"
echo "debug prompt" > "$REPO/skills/debugging/prompts/default.md"

echo "# TDD Skill" > "$REPO/skills/tdd/SKILL.md"
echo "go test template" > "$REPO/skills/tdd/templates/go/test.md"

echo "// commit command" > "$REPO/commands/commit/index.js"

echo "# Superpowers Docs" > "$REPO/docs/README.md"

# Root-level file
echo "// gemini extension" > "$REPO/gemini-extension.js"

# Verify structure
find "$REPO" -not -path '*/.git/*' -not -path '*/.git' | sort
```

Expected:
- exit_code: 0
- skills/brainstorming/SKILL.md
- skills/debugging/prompts/default.md
- skills/tdd/templates/go/test.md
- commands/commit/index.js
- docs/README.md
- gemini-extension.js

### Step 2: Collect — run collect and verify JSON output

```bash
SOURCE=$(ss status --json | jq -r '.source.path')
TARGET=$(ss status --json | jq -r '.targets[] | select(.name == "claude") | .path')

# Run collect targeting claude, with --json (implies --force)
ss collect claude --json
```

Expected:
- exit_code: 0
- jq: .pulled | length == 1
- jq: .pulled[0] == "superpowers"
- jq: .failed | length == 0

### Step 3: Verify — nested files exist in source

```bash
SOURCE=$(ss status --json | jq -r '.source.path')

# Check all nested files were copied
echo "=== Checking nested files ==="

test -f "$SOURCE/superpowers/skills/brainstorming/SKILL.md" && echo "FOUND brainstorming/SKILL.md" || echo "MISSING brainstorming/SKILL.md"
test -f "$SOURCE/superpowers/skills/debugging/SKILL.md" && echo "FOUND debugging/SKILL.md" || echo "MISSING debugging/SKILL.md"
test -f "$SOURCE/superpowers/skills/debugging/prompts/default.md" && echo "FOUND debugging/prompts/default.md" || echo "MISSING debugging/prompts/default.md"
test -f "$SOURCE/superpowers/skills/tdd/SKILL.md" && echo "FOUND tdd/SKILL.md" || echo "MISSING tdd/SKILL.md"
test -f "$SOURCE/superpowers/skills/tdd/templates/go/test.md" && echo "FOUND tdd/templates/go/test.md" || echo "MISSING tdd/templates/go/test.md"
test -f "$SOURCE/superpowers/commands/commit/index.js" && echo "FOUND commands/commit/index.js" || echo "MISSING commands/commit/index.js"
test -f "$SOURCE/superpowers/docs/README.md" && echo "FOUND docs/README.md" || echo "MISSING docs/README.md"
test -f "$SOURCE/superpowers/gemini-extension.js" && echo "FOUND gemini-extension.js" || echo "MISSING gemini-extension.js"

# Count total files (excluding .git)
TOTAL=$(find "$SOURCE/superpowers" -type f | wc -l | tr -d ' ')
echo "TOTAL_FILES=$TOTAL"
```

Expected:
- exit_code: 0
- FOUND brainstorming/SKILL.md
- FOUND debugging/SKILL.md
- FOUND debugging/prompts/default.md
- FOUND tdd/SKILL.md
- FOUND tdd/templates/go/test.md
- FOUND commands/commit/index.js
- FOUND docs/README.md
- FOUND gemini-extension.js
- Not MISSING

### Step 4: Verify — .git directory was NOT copied to source

```bash
SOURCE=$(ss status --json | jq -r '.source.path')

if [ -d "$SOURCE/superpowers/.git" ]; then
  echo "FAIL: .git directory was copied to source"
  ls -la "$SOURCE/superpowers/.git/"
  exit 1
else
  echo "OK: .git directory correctly skipped"
fi

# Double check no .git files leaked
GIT_FILES=$(find "$SOURCE/superpowers" -path '*/.git/*' 2>/dev/null | wc -l | tr -d ' ')
echo "GIT_FILES_IN_SOURCE=$GIT_FILES"
```

Expected:
- exit_code: 0
- OK: .git directory correctly skipped
- GIT_FILES_IN_SOURCE=0
- Not FAIL

### Step 5: Verify — file content integrity

```bash
SOURCE=$(ss status --json | jq -r '.source.path')

# Verify actual file content was preserved (not truncated or empty)
CONTENT=$(cat "$SOURCE/superpowers/skills/tdd/templates/go/test.md")
echo "DEEPEST_FILE_CONTENT=$CONTENT"

SIZE=$(wc -c < "$SOURCE/superpowers/skills/tdd/templates/go/test.md" | tr -d ' ')
echo "DEEPEST_FILE_SIZE=$SIZE"

# Verify root-level file
ROOT_CONTENT=$(cat "$SOURCE/superpowers/gemini-extension.js")
echo "ROOT_FILE_CONTENT=$ROOT_CONTENT"
```

Expected:
- exit_code: 0
- DEEPEST_FILE_CONTENT=go test template
- ROOT_FILE_CONTENT=// gemini extension

## Pass Criteria

- Steps 1-5 all pass with exit_code: 0
- All 8 nested files are present in source after collect (Step 3)
- `.git/` directory is NOT present in source (Step 4)
- File content is preserved, not empty (Step 5)
