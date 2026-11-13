# Runs a PowerShell script as the logged-in desktop user through a scheduled task.
# utmctl exec runs as SYSTEM, which can always create symlinks; this gives the token a real user gets.
# Usage: launch-task.ps1 -Name <task> -User <name> -Script <ps1> [-ScriptArgs <string>] [-Basic]
# -Basic wraps the run in `runas /trustlevel:0x20000` (basic-user token: no symlink right).
# Interactive tasks only start while -User is logged in; S4U gives a different token, so it is not used.
param(
  [Parameter(Mandatory)] [string]$Name,
  [Parameter(Mandatory)] [string]$User,
  [Parameter(Mandatory)] [string]$Script,
  [string]$ScriptArgs = '',
  [switch]$Basic
)
$ps = "powershell.exe -NoProfile -ExecutionPolicy Bypass -WindowStyle Hidden -File $Script $ScriptArgs"
if ($Basic) { $a = New-ScheduledTaskAction -Execute 'runas.exe' -Argument "/trustlevel:0x20000 `"$ps`"" }
else { $a = New-ScheduledTaskAction -Execute 'powershell.exe' -Argument ($ps -replace '^powershell\.exe ', '') }
$p = New-ScheduledTaskPrincipal -UserId $User -LogonType Interactive -RunLevel Limited
Register-ScheduledTask -TaskName $Name -Action $a -Principal $p -Force | Out-Null
Start-ScheduledTask -TaskName $Name
