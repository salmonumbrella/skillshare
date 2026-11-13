---
sidebar_position: 3
---

# Windows

Windows 관련 문제와 해결 방법입니다.

## Installation

### How do I install on Windows?

**PowerShell:**
```powershell
irm https://raw.githubusercontent.com/runkids/skillshare/main/install.ps1 | iex
```

**Or download manually:**
1. [releases](https://github.com/runkids/skillshare/releases)로 이동
2. Windows용 `.zip` 다운로드
3. 압축 해제 후 PATH에 추가

---

## Permissions

### Does skillshare need admin privileges?

**아니요.** 폴더(skills, 그리고 `symlink` 모드의 agents 또는 extras)는 NTFS junction으로 링크됩니다. junction은 디렉터리에 대해 symlink처럼 동작하며 관리자 권한이 필요하지 않습니다.

단일 파일은 junction으로 링크할 수 없습니다. 파일을 링크하는 모드 — `merge` 모드의 agents, `merge` 모드 디렉터리 extra의 파일, 그리고 공유 `AGENTS.md` 같은 single-file extra — 는 실제 symlink를 사용하며, 이를 만들려면 [Developer Mode](#file-links-need-windows-developer-mode-copying-instead)(또는 관리자 셸)가 필요합니다. 없으면 skillshare가 해당 파일을 복사합니다.

---

## File Locations

### Where are config files on Windows?

```
%AppData%\skillshare\config.yaml
%AppData%\skillshare\skills\
%AppData%\skillshare\backups\
```

일반적으로:
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

**현재 세션에만:**
```powershell
$env:GITHUB_TOKEN = "ghp_your_token"
```

**영구 설정 (사용자 수준):**
```powershell
[Environment]::SetEnvironmentVariable("GITHUB_TOKEN", "ghp_your_token", "User")
```

**그런 다음 PowerShell을 재시작하세요.**

### How do I set SKILLSHARE_CONFIG?

```powershell
$env:SKILLSHARE_CONFIG = "C:\path\to\custom\config.yaml"
skillshare status
```

---

## Common Issues

### `junction creation failed`

**Cause:** Target 경로가 이미 파일 또는 호환되지 않는 유형으로 존재합니다.

**Solution:**
```powershell
# 기존 항목을 백업하고 제거
skillshare backup
Remove-Item -Path "$env:USERPROFILE\.claude\skills" -Recurse -Force
skillshare sync
```

### `path too long`

**Cause:** Windows는 기본적으로 260자 경로 제한이 있습니다.

**Solution:** 긴 경로를 활성화하세요.
```powershell
# 관리자 권한으로 실행
Set-ItemProperty -Path "HKLM:\SYSTEM\CurrentControlSet\Control\FileSystem" -Name "LongPathsEnabled" -Value 1
```

그런 다음 재시작하세요.

### `access denied`

**Cause:** 파일 또는 디렉터리가 사용 중이거나 보호되어 있습니다.

**Solutions:**
1. 해당 파일을 사용 중인 프로그램 종료
2. 백신 프로그램이 차단하고 있지 않은지 확인
3. PowerShell을 관리자 권한으로 실행 (거의 필요 없음)

### `file links need Windows Developer Mode; copying instead`

**Cause:** Windows에서는 Developer Mode가 켜져 있을 때(또는 관리자 셸에서)만 파일 symlink를 만들 수 있습니다. `merge` 모드의 agents, `merge` 모드의 디렉터리 extras, 그리고 single-file extras(공유 `AGENTS.md` 등)는 단일 파일을 링크하므로, Developer Mode가 없으면 `sync`가 대신 복사하고 이 줄을 표시합니다. `status`, `doctor`, `extras list`는 이런 target을 `copy`로 표시합니다.

복사본은 추적되므로 sync 입장에서는 여전히 링크처럼 동작합니다:

- 소스가 바뀌면 업데이트되고, 소스가 제거되면 함께 제거됩니다.
- target에 직접 만든 파일은 `merge` 모드와 마찬가지로 건드리지 않습니다.
- 파일 링크를 쓸 수 있게 되면 다음 sync에서 복사본을 링크로 교체합니다.

**Solution:** 따로 할 일은 없으며 복사본은 계속 정상적으로 동작합니다. 링크를 쓰려면 Developer Mode를 켜고(Windows 11: **Settings → System → For developers → Developer Mode**, Windows 10: **Settings → Update & Security → For developers**) `skillshare sync --all`을 다시 실행하세요. `skillshare ui`가 실행 중이면 재시작하세요.

내용이 같은 로컬 파일은 `local preserved`로 표시되며, `sync extras`는 해당 파일에 `--force`를 권하지 않습니다. 관리되는 링크가 아닌 로컬 파일로 유지됩니다.

### Agent files or AGENTS.md show a folder icon and can't be read

**Cause:** 이전 버전은 단일 파일을 디렉터리 junction으로 링크했습니다. 그래서 탐색기에는 파일이 폴더로 보이고, 도구가 읽을 수 없습니다.

**Solution:** `skillshare sync --all`(또는 `skillshare sync agents` / `skillshare sync extras`)을 실행하세요. sync가 이 깨진 링크를 파일 symlink로, Developer Mode가 꺼져 있으면 복사본으로 교체합니다. 대시보드의 **AGENTS.md** 탭에서는 해당 target에 경고가 표시되며, `copy`로 전환해도 해결됩니다.

### `symlinks not working`

**Note:** Windows에서 skillshare는 폴더를 NTFS junction으로, 단일 파일을 symlink로 링크합니다. symlink 오류가 보인다면, Windows 버전의 skillshare를 사용하고 있는지 확인하세요.

### `Incorrect function` in Antigravity

Antigravity는 junction을 탐색할 수 없습니다. [Antigravity does not load synced skills](./common-errors.md#antigravity-does-not-load-synced-skills)를 참고하세요.

---

## PowerShell Tips

### Aliases

PowerShell 프로필(`$PROFILE`)에 추가하세요.
```powershell
Set-Alias -Name ss -Value skillshare
function sss { skillshare sync }
function ssp { param($m) skillshare push -m $m }
function ssl { skillshare pull }
```

### Check PowerShell version

skillshare는 PowerShell 5.1+와 PowerShell Core 7+에서 동작합니다.
```powershell
$PSVersionTable.PSVersion
```

---

## WSL Compatibility

Windows Subsystem for Linux를 사용한다면:

### Separate installations

Windows와 WSL을 위한 skillshare 설치를 분리해서 유지하세요.
- Windows: `%AppData%\skillshare\`
- WSL: `~/.config/skillshare/`

### Share via git

동일한 git remote를 사용해 둘 사이를 동기화하세요.
```bash
# Windows
skillshare push -m "From Windows"

# WSL
skillshare pull
```

---

## Getting Help

버그 리포트에 다음을 포함하세요.
- Windows 버전: `winver`
- PowerShell 버전: `$PSVersionTable.PSVersion`
- skillshare 버전: `skillshare --version`
- 전체 오류 메시지

---

## Related

- [Common Errors](./common-errors.md) — 일반적인 오류 해결 방법
- [Configuration](/docs/reference/targets/configuration) — Config 파일 참조
