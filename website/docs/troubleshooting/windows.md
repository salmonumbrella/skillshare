---
sidebar_position: 3
---

# Windows

Windows-specific issues and solutions.

## Installation

### How do I install on Windows?

**PowerShell:**
```powershell
irm https://raw.githubusercontent.com/runkids/skillshare/main/install.ps1 | iex
```

**Or download manually:**
1. Go to [releases](https://github.com/runkids/skillshare/releases)
2. Download the `.zip` for Windows
3. Extract and add to PATH

---

## Permissions

### Does skillshare need admin privileges?

**No.** Folders (skills, and agents or extras in `symlink` mode) are linked with NTFS junctions, which work like symlinks for directories and don't require admin privileges.

Single files can't be linked with a junction. Modes that link files — agents in `merge` mode, the files of a directory extra in `merge` mode, and single-file extras such as a shared `AGENTS.md` — use real symlinks, which need [Developer Mode](#file-links-need-windows-developer-mode-copying-instead) (or an Administrator shell). Without it, skillshare copies those files instead.

---

## File Locations

### Where are config files on Windows?

```
%AppData%\skillshare\config.yaml
%AppData%\skillshare\skills\
%AppData%\skillshare\backups\
```

Typically:
```
C:\Users\YourName\AppData\Roaming\skillshare\
```

### Where are target directories?

```
%USERPROFILE%\.claude\skills\
%USERPROFILE%\.cursor\skills\
%USERPROFILE%\.codex\skills\
```

---

## Environment Variables

### How do I set GITHUB_TOKEN on Windows?

**Current session only:**
```powershell
$env:GITHUB_TOKEN = "ghp_your_token"
```

**Permanent (user-level):**
```powershell
[Environment]::SetEnvironmentVariable("GITHUB_TOKEN", "ghp_your_token", "User")
```

**Then restart PowerShell.**

### How do I set SKILLSHARE_CONFIG?

```powershell
$env:SKILLSHARE_CONFIG = "C:\path\to\custom\config.yaml"
skillshare status
```

---

## Common Issues

### `junction creation failed`

**Cause:** Target path already exists as a file or incompatible type.

**Solution:**
```powershell
# Backup and remove existing
skillshare backup
Remove-Item -Path "$env:USERPROFILE\.claude\skills" -Recurse -Force
skillshare sync
```

### `path too long`

**Cause:** Windows has a 260 character path limit by default.

**Solution:** Enable long paths:
```powershell
# Run as Administrator
Set-ItemProperty -Path "HKLM:\SYSTEM\CurrentControlSet\Control\FileSystem" -Name "LongPathsEnabled" -Value 1
```

Then restart.

### `access denied`

**Cause:** File or directory is in use or protected.

**Solutions:**
1. Close any programs using the files
2. Check antivirus isn't blocking
3. Run PowerShell as Administrator (rarely needed)

### `file links need Windows Developer Mode; copying instead`

**Cause:** Windows only lets you create file symlinks with Developer Mode on (or from an Administrator shell). Agents in `merge` mode, directory extras in `merge` mode, and single-file extras (such as a shared `AGENTS.md`) link single files, so without Developer Mode `sync` copies them instead and shows this line. `status`, `doctor`, and `extras list` show these targets as `copy`.

The copies are tracked, so they still behave like links as far as sync is concerned:

- They are updated when the source changes, and removed when the source is removed.
- Files you created yourself in the target are left alone, as in `merge` mode.
- Once file links work, the next sync replaces the copies with links.

**Solution:** Nothing is required; the copies keep working. To get links instead, turn on Developer Mode (Windows 11: **Settings → System → For developers → Developer Mode**; Windows 10: **Settings → Update & Security → For developers**), then run `skillshare sync --all` again. Restart `skillshare ui` if it is running.

Identical local files are reported as `local preserved`; `sync extras` does not suggest `--force` for them. They remain local files, not managed links.

### Agent files or AGENTS.md show a folder icon and can't be read

**Cause:** Older versions linked single files with a directory junction. Explorer shows the file as a folder, and tools can't read it.

**Solution:** Run `skillshare sync --all` (or `skillshare sync agents` / `skillshare sync extras`). Sync replaces these broken links with a file symlink, or with a copy when Developer Mode is off. On the dashboard's **AGENTS.md** tab, an affected target shows a warning; switching it to `copy` also fixes it.

### `symlinks not working`

**Note:** On Windows, skillshare links folders with NTFS junctions and single files with symlinks. If you see symlink errors, ensure you're using the Windows version of skillshare.

### `Incorrect function` in Antigravity

Antigravity cannot traverse junctions. See [Antigravity does not load synced skills](./common-errors.md#antigravity-does-not-load-synced-skills).

---

## PowerShell Tips

### Aliases

Add to your PowerShell profile (`$PROFILE`):
```powershell
Set-Alias -Name ss -Value skillshare
function sss { skillshare sync }
function ssp { param($m) skillshare push -m $m }
function ssl { skillshare pull }
```

### Check PowerShell version

skillshare works with PowerShell 5.1+ and PowerShell Core 7+:
```powershell
$PSVersionTable.PSVersion
```

---

## WSL Compatibility

If you use Windows Subsystem for Linux:

### Separate installations

Keep separate skillshare installations for Windows and WSL:
- Windows: `%AppData%\skillshare\`
- WSL: `~/.config/skillshare/`

### Share via git

Use the same git remote to sync between them:
```bash
# Windows
skillshare push -m "From Windows"

# WSL
skillshare pull
```

---

## Getting Help

Include in bug reports:
- Windows version: `winver`
- PowerShell version: `$PSVersionTable.PSVersion`
- skillshare version: `skillshare --version`
- Full error message

---

## Related

- [Common Errors](./common-errors.md) — General error solutions
- [Configuration](/docs/reference/targets/configuration) — Config file reference
