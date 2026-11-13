---
sidebar_position: 3
---

# Windows

Windows 专属的问题与解决方式。

## Installation

### 如何在 Windows 上安装？

**PowerShell：**
```powershell
irm https://raw.githubusercontent.com/runkids/skillshare/main/install.ps1 | iex
```

**或手动下载：**
1. 前往 [releases](https://github.com/runkids/skillshare/releases)
2. 下载 Windows 版的 `.zip`
3. 解压并添加到 PATH

---

## Permissions

### skillshare 需要管理员权限吗？

**不需要。** 文件夹（skills，以及 `symlink` mode 下的 agents 或 extras）通过 NTFS junction 链接；junction 在目录层面上的行为类似 symlink，而且不需要管理员权限。

单个文件无法用 junction 链接。会链接文件的 mode——`merge` mode 下的 agents、`merge` mode 下目录类 extra 的文件，以及单文件 extras（例如共享 `AGENTS.md`）——使用真正的 symlink，需要开启 [Developer Mode](#file-links-need-windows-developer-mode-copying-instead)（或使用管理员 shell）。没有开启时，skillshare 会改为复制这些文件。

---

## File Locations

### Windows 上的配置文件在哪里？

```
%AppData%\skillshare\config.yaml
%AppData%\skillshare\skills\
%AppData%\skillshare\backups\
```

通常是：
```
C:\Users\YourName\AppData\Roaming\skillshare\
```

### Target 目录在哪里？

```
%USERPROFILE%\.claude\skills\
%USERPROFILE%\.cursor\skills\
%USERPROFILE%\.codex\skills\
```

---

## Environment Variables

### 如何在 Windows 上设置 GITHUB_TOKEN？

**仅当前会话有效：**
```powershell
$env:GITHUB_TOKEN = "ghp_your_token"
```

**永久生效（用户层级）：**
```powershell
[Environment]::SetEnvironmentVariable("GITHUB_TOKEN", "ghp_your_token", "User")
```

**然后重新启动 PowerShell。**

### 如何设置 SKILLSHARE_CONFIG？

```powershell
$env:SKILLSHARE_CONFIG = "C:\path\to\custom\config.yaml"
skillshare status
```

---

## Common Issues

### `junction creation failed`

**原因：** 目标路径已经以文件形式存在，或类型不兼容。

**解决方式：**
```powershell
# 备份并移除既有内容
skillshare backup
Remove-Item -Path "$env:USERPROFILE\.claude\skills" -Recurse -Force
skillshare sync
```

### `path too long`

**原因：** Windows 默认的路径长度限制为 260 个字符。

**解决方式：** 启用长路径支持：
```powershell
# 以管理员身份运行
Set-ItemProperty -Path "HKLM:\SYSTEM\CurrentControlSet\Control\FileSystem" -Name "LongPathsEnabled" -Value 1
```

然后重新启动。

### `access denied`

**原因：** 文件或目录正被占用，或受到保护。

**解决方式：**
1. 关闭所有正在使用这些文件的程序
2. 确认防病毒软件没有拦截
3. 以管理员身份运行 PowerShell（很少需要）

### `file links need Windows Developer Mode; copying instead` {#file-links-need-windows-developer-mode-copying-instead}

**原因：** Windows 只有在开启 Developer Mode（或从管理员 shell）时才允许创建文件 symlink。`merge` mode 下的 agents、`merge` mode 下的目录类 extras，以及单文件 extras（例如共享 `AGENTS.md`）链接的是单个文件，因此在没有 Developer Mode 时，`sync` 会改为复制它们并显示这一行。`status`、`doctor` 和 `extras list` 会把这些 target 显示为 `copy`。

这些副本会被追踪，因此在 sync 看来，它们的行为仍然和链接一样：

- source 变更时会更新，source 移除时会删除。
- 你自己在 target 中创建的文件不会被动到，和 `merge` mode 一样。
- 一旦文件链接可用，下次 sync 会把副本替换为链接。

**解决方式：** 不需要做任何事；副本会继续正常工作。如果想改用链接，请开启 Developer Mode（Windows 11：**Settings → System → For developers → Developer Mode**；Windows 10：**Settings → Update & Security → For developers**），然后再次运行 `skillshare sync --all`。如果 `skillshare ui` 正在运行，请重新启动它。

内容相同的本地文件会显示为 `local preserved`；`sync extras` 不会为它们建议使用 `--force`。它们仍是本地文件，不是受管理的链接。

### Agent 文件或 AGENTS.md 显示为文件夹图标且无法读取 {#agent-files-or-agentsmd-show-a-folder-icon-and-cant-be-read}

**原因：** 旧版本用目录 junction 链接单个文件。资源管理器会把该文件显示为文件夹，工具也无法读取它。

**解决方式：** 运行 `skillshare sync --all`（或 `skillshare sync agents` / `skillshare sync extras`）。sync 会把这些损坏的链接替换为文件 symlink；如果 Developer Mode 未开启，则替换为副本。在控制台的 **AGENTS.md** 标签页上，受影响的 target 会显示警告；把它切换为 `copy` 也能修复。

### `symlinks not working`

**说明：** 在 Windows 上，skillshare 用 NTFS junction 链接文件夹，用 symlink 链接单个文件。如果你看到 symlink 相关的错误，请确认使用的是 Windows 版的 skillshare。

### Antigravity 中出现 `Incorrect function`

Antigravity 无法遍历 junction。参见 [Antigravity does not load synced skills](./common-errors.md#antigravity-does-not-load-synced-skills)。

---

## PowerShell Tips

### 别名

添加到你的 PowerShell profile（`$PROFILE`）：
```powershell
Set-Alias -Name ss -Value skillshare
function sss { skillshare sync }
function ssp { param($m) skillshare push -m $m }
function ssl { skillshare pull }
```

### 检查 PowerShell 版本

skillshare 支持 PowerShell 5.1+ 与 PowerShell Core 7+：
```powershell
$PSVersionTable.PSVersion
```

---

## WSL Compatibility

如果你使用 Windows Subsystem for Linux：

### 各自独立安装

为 Windows 和 WSL 分别保留独立的 skillshare 安装：
- Windows：`%AppData%\skillshare\`
- WSL：`~/.config/skillshare/`

### 通过 git 共享

用同一个 git remote 在两者之间同步：
```bash
# Windows
skillshare push -m "From Windows"

# WSL
skillshare pull
```

---

## Getting Help

提交 bug 报告时请附上：
- Windows 版本：`winver`
- PowerShell 版本：`$PSVersionTable.PSVersion`
- skillshare 版本：`skillshare --version`
- 完整的错误信息

---

## Related

- [Common Errors](./common-errors.md) — 一般错误解决方式
- [Configuration](/docs/reference/targets/configuration) — 配置文件参考
