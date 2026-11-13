# One-step setup for hands-on testing on Windows (a UTM guest or a real machine), then starts the
# dashboard. Everything lives in an isolated home under -Root, so the real profile is never touched.
# Expects in -Dir: ss.exe, ss-ui-dist.zip, ss-version.txt (scripts/windows/utm.sh build / push-manual).
# Usage (a normal, non-admin PowerShell tests the no-symlink fallback):
#   powershell -ExecutionPolicy Bypass -File C:\Users\Public\ss-setup.ps1
#   powershell -ExecutionPolicy Bypass -File C:\Users\Public\ss-setup.ps1 -Clean
param(
  [string]$Dir = 'C:\Users\Public',
  [string]$Root = 'C:\Users\Public\sstest\manual',
  [switch]$Clean
)
$ErrorActionPreference = 'Stop'

if ($Clean) {
  # rmdir does not follow junctions, so a junction left by a test is removed, not its target.
  if (Test-Path -LiteralPath $Root) { cmd /c "rmdir /s /q `"$Root`"" }
  Write-Host "Removed $Root"
  return
}

# The same overrides the E2E script uses: skillshare resolves home and config from these.
$env:USERPROFILE = $Root
$env:HOME = $Root
$env:APPDATA = "$Root\AppData\Roaming"
$env:LOCALAPPDATA = "$Root\AppData\Local"
New-Item -ItemType Directory -Force $env:APPDATA, $env:LOCALAPPDATA | Out-Null

$exe = "$Root\ss.exe"
Copy-Item "$Dir\ss.exe" $exe -Force
# The dashboard reads its UI from this cache, so it never needs to download it.
$version = (Get-Content "$Dir\ss-version.txt" -Raw).Trim()
$ui = "$env:APPDATA\skillshare\ui\$version"
New-Item -ItemType Directory -Force $ui | Out-Null
Expand-Archive "$Dir\ss-ui-dist.zip" $ui -Force

# Two tools with their own instruction files, so there is something to attach and restore.
New-Item -ItemType Directory -Force "$Root\.claude", "$Root\.codex" | Out-Null
if (-not (Test-Path "$Root\.claude\CLAUDE.md")) { Set-Content "$Root\.claude\CLAUDE.md" '# My own Claude rules' }
if (-not (Test-Path "$Root\.codex\AGENTS.md")) { Set-Content "$Root\.codex\AGENTS.md" '# My own Codex rules' }

Set-Location $Root
& $exe init --no-copy --no-git --no-skill -t claude,codex
Write-Host "Test home: $Root (files to inspect: $Root\.claude\CLAUDE.md, $Root\.codex\AGENTS.md)"
& $exe ui
