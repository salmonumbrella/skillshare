# Windows Runbook: File Links and Copy Fallback

Manual runbook (not mdproof): it runs on a real Windows machine, driven from a macOS
host through UTM's `utmctl`. It checks that file-based link modes never produce a
junction to a file, that they fall back to copies without file-symlink rights, and
that skills stay junctions recognised as links.

## Scope

- Skills (merge): directory junction, tag `0xa0000003`, not re-linked on resync.
- Agents (merge), directory extra (merge), single-file extra `AGENTS.md` (merge):
  - with file-symlink rights: symlink, tag `0xa000000c`, readable;
  - without: regular file with source content and the warning
    `file links need Windows Developer Mode; copying instead`.
- `-Extended`: two source edits propagate without drift backups or a "backed up"
  line, removed source agent is pruned while an
  untracked agent stays, a junction-to-file `AGENTS.md` is replaced, and
  `status`/`doctor`/`list`/`extras list` output is captured.

## Environment

- Windows guest in UTM with the guest agent (runs as SYSTEM). Everything lives under
  `C:\Users\Public\sstest\`. The script overrides `USERPROFILE`, `HOME`, `APPDATA`,
  `LOCALAPPDATA` and `TEMP`, and aborts unless `doctor` reports the config under the
  test root.
- The desktop user must be logged in: the `-LogonType Interactive` task in step 3
  only runs in an interactive session, and restarting the VM logs the user out. Do
  not substitute `S4U`; its token can differ from what a real user gets.
- `utmctl exec` is asynchronous and returns no output. The script writes a report
  and ends it with `DONE`; poll it with `utmctl file pull`.
- SYSTEM and administrators (including an admin with UAC off, where
  `-RunLevel Limited` still yields a full token) can create file symlinks. For the
  fallback path run through `runas /trustlevel:0x20000` (basic-user token); the
  report's `SeCreateSymbolicLinkPrivilege: absent` and `file symlink probe: FAILED`
  lines confirm it.

## Steps

1. Build in the devcontainer and copy out (`GOARCH=amd64` for x64 guests):

   ```bash
   docker exec -w /workspace "$CONTAINER" bash -lc \
     'GOOS=windows GOARCH=arm64 go build -o /tmp/ss.exe ./cmd/skillshare'
   docker cp "$CONTAINER":/tmp/ss.exe ./ss.exe
   ```

   For a baseline, build `git archive HEAD` extracted to a temp dir the same way.

2. Push files (create `C:\Users\Public\sstest` first and grant the desktop user
   access, e.g. `icacls C:\Users\Public\sstest /grant '<user>:(OI)(CI)F'`):

   ```bash
   U=~/Applications/UTM.app/Contents/MacOS/utmctl
   $U file push Windows 'C:\Users\Public\sstest\ss.exe' < ss.exe
   $U file push Windows 'C:\Users\Public\sstest\e2e-file-links.ps1' < scripts/windows/e2e-file-links.ps1
   ```

3. Run as the desktop user through a scheduled task registered from SYSTEM (push
   this as `launch.ps1` and `utmctl exec` it with `powershell.exe -File`):

   ```powershell
   $ps = 'powershell.exe -NoProfile -ExecutionPolicy Bypass -WindowStyle Hidden -File C:\Users\Public\sstest\e2e-file-links.ps1 -Exe C:\Users\Public\sstest\ss.exe -Root C:\Users\Public\sstest\run -Out C:\Users\Public\sstest\out.txt -Extended'
   $a = New-ScheduledTaskAction -Execute 'runas.exe' -Argument "/trustlevel:0x20000 `"$ps`""   # or -Execute powershell.exe for a full token
   $p = New-ScheduledTaskPrincipal -UserId '<user>' -LogonType Interactive -RunLevel Limited
   Register-ScheduledTask -TaskName sstest-run -Action $a -Principal $p -Force
   Start-ScheduledTask -TaskName sstest-run
   ```

4. Poll until the last line is `DONE`:

   ```bash
   $U file pull Windows 'C:\Users\Public\sstest\out.txt'
   ```

5. Clean up from SYSTEM: `Unregister-ScheduledTask -TaskName sstest-* -Confirm:$false`
   and `cmd /c rmdir /s /q C:\Users\Public\sstest` (`rmdir` does not follow junctions).

## Pass Criteria

- Baseline (before the fix) reproduces the bug: agent, rule and `AGENTS.md` entries
  are `LinkType=Junction`, tag `0xa0000003`, `read=NO`.
- Fixed, full token: those entries are `LinkType=SymbolicLink`, tag `0xa000000c`,
  `read=yes` with the source content.
- Fixed, basic-user token: those entries have `reparseTag=none`, `read=yes`; the
  warning line appears for agents and each file extra; `.skillshare-manifest.json`
  exists in the agents and rules targets.
- Both fixed runs: `demo-skill` is a junction and `skill junction recreated: False`;
  (a) target reads `shared v2`, `drift backups before/after source edits: 0/0` and
  `backed-up message after source edits: False`; (b) `demo-agent.md : MISSING`,
  `my-own.md` kept; (c) the junction becomes a readable file or symlink after sync.
- Fixed, basic-user token: `status agents line` shows `copied … [copy] 1/1 linked` and
  `doctor agents line` shows `[copy] synced (1/1 linked)`.
- Note: `utmctl file pull` exits 0 even when the file is missing; check its output
  for `failed to open` when verifying cleanup.
