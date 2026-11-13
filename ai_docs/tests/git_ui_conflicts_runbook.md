# Git Sync Conflict Resolution Runbook

Verify updates from two computers through the dashboard's Git API and UI.

## Scope

- Diverged commits merge without discarding either history.
- Conflict previews and cancelled reviews leave the local tree and HEAD unchanged.
- Every conflict needs an explicit whole-file choice, including modify/delete conflicts.
- Stale, incomplete, invalid, or extra choices cannot alter the merge.
- Commit-and-pull ordering, dry runs, and translation parity remain correct.

## Environment

Run automated steps inside the Linux devcontainer. Handler tests create isolated temporary bare remotes and local repositories; they do not use configured user remotes. Each step is self-contained.

## Steps

### 1. Verify real Git operations through the API

```bash
cd /workspace
go test ./internal/git ./internal/server -run 'TestPullWithProgress|TestHandlePull|TestHandlePush_RemoteAheadReportsPushRejected' -count=1
```

**Expected**:
- exit_code: 0
- regex: ok\s+skillshare/internal/git
- regex: ok\s+skillshare/internal/server

### 2. Verify dashboard interactions and translations

```bash
cd /workspace/ui
pnpm exec vitest run src/pages/GitSyncPage.test.tsx src/i18n/i18n.test.ts --reporter=dot
```

**Expected**:
- exit_code: 0
- regex: 2 passed

## Visual Verification

Build the dashboard inside the devcontainer with `pnpm run build` in `/workspace/ui`. Launch `bash /workspace/scripts/git/preview-pull-conflicts.sh` in a throwaway container from the devcontainer image, with `/workspace` and its dependency/cache volumes mounted and container port 49421 forwarded to a free host port. The script creates temporary Git repos, isolates all XDG paths, and serves the built UI without changing user configuration. Its temporary data disappears when the throwaway container is removed.

Open `/git` on that server. Check Clean and Playful in light and dark at desktop width, and the conflict dialog at a narrow viewport:

1. Confirm that one local and one remote commit are shown, with **Pull and merge** available before pushing.
2. Pull, compare both versions of each file, and cancel. Local HEAD and files must remain unchanged.
3. Reopen **Review conflicts**, choose different sides for the two files, and apply. The button stays disabled until every file has a choice.
4. Confirm that the dialog closes, the working tree is clean, both prior commits are ancestors of HEAD, and **Push** is available.
5. Push and confirm that the isolated remote receives the merge.

## Pass Criteria

- Both automated steps pass.
- The visual review covers all four themes and the narrow dialog.
- The complete UI flow uses the isolated fixture's real API and Git repos.
- Record any unavailable previews or environmental limitations; preserve unrelated workspace changes.
