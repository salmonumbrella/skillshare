# CLI E2E Runbook: Install Pinned to Commit SHA or Tag

Validates that `--branch` accepts a tag or commit SHA (issue #281), installs exactly that revision, and that `check`/`update` treat a SHA pin as fixed.

**Origin**: v0.x — `--branch` previously accepted branch names only; clone used `--branch <name>` which rejects SHAs.

## Scope

- `ss install <repo> --branch <tag>` installs the tagged revision
- `ss install <repo> --branch <sha>` installs that commit (shallow clone + targeted fetch)
- Abbreviated SHA (7+ hex) also resolves (falls back to full fetch)
- `ss check` reports a SHA-pinned skill as up to date without network
- `--track` combined with a SHA or tag is rejected with a clear error (CLI and Web UI share the guard)

## Environment

Run inside devcontainer with `ssenv` isolation. Network access required (clones from `github.com`).

## Steps

### 1. Install pinned to a tag

```bash
ss install https://github.com/JuliusBrussee/caveman.git/skills/caveman --branch v2.6.0 --name caveman-tag --skip-audit -y
```

**Expected**: exit_code 0, output contains `Installed`.

### 2. Verify metadata records the tag

```bash
grep -A3 '"caveman-tag"' ~/.config/skillshare/skills/.skillshare-metadata.json | grep branch
```

**Expected**: regex `"branch": "v2.6.0"`

### 3. Install pinned to a full commit SHA

```bash
ss install https://github.com/JuliusBrussee/caveman.git/skills/caveman --branch 09715751c1b30bb7142304086a6375d50d3c4cee --name caveman-sha --skip-audit -y
```

**Expected**: exit_code 0, output contains `Installed`.

### 4. Verify installed version matches the pin

```bash
grep -A6 '"caveman-sha"' ~/.config/skillshare/skills/.skillshare-metadata.json | grep -E 'version|branch'
```

**Expected**: regex `"version": "0971575"` and `"branch": "09715751c1b30bb7142304086a6375d50d3c4cee"`

### 5. Install pinned to an abbreviated SHA

```bash
ss install https://github.com/JuliusBrussee/caveman.git/skills/caveman --branch 0971575 --name caveman-short --skip-audit -y
```

**Expected**: exit_code 0, output contains `Installed`.

### 6. Check reports pinned skills as up to date

```bash
ss check --no-tui
```

**Expected**: exit_code 0; `caveman-sha` and `caveman-short` are NOT listed as having updates.

### 7. Tracked install rejects a SHA pin

```bash
ss install https://github.com/JuliusBrussee/caveman.git --track --branch 09715751c1b30bb7142304086a6375d50d3c4cee --name caveman-tracked
```

**Expected**: exit_code non-zero, output contains `--track cannot pin a commit SHA`.

### 8. Cleanup

```bash
ss uninstall caveman-tag --force && ss uninstall caveman-sha --force && ss uninstall caveman-short --force
```

**Expected**: exit_code 0.

## Pass Criteria

- All steps marked PASS
- Steps 3 and 5 install the same content (same `tree_hash` in metadata)
