---
sidebar_position: 3
---

# Windows

Windows 特有的問題與解決方法。

## 安裝

### 我要如何在 Windows 上安裝？

**PowerShell：**
```powershell
irm https://raw.githubusercontent.com/runkids/skillshare/main/install.ps1 | iex
```

**或手動下載：**
1. 前往 [releases](https://github.com/runkids/skillshare/releases)
2. 下載 Windows 版的 `.zip`
3. 解壓縮並加入 PATH

---

## 權限

### skillshare 需要系統管理員權限嗎？

**不需要。** 資料夾（skills，以及 `symlink` 模式的 agents 或 extras）以 NTFS junction 連結。NTFS junction 對目錄的作用類似 symlink，不需要系統管理員權限。

單一檔案無法用 junction 連結。會連結檔案的模式——`merge` 模式的 agents、`merge` 模式下目錄型 extra 的檔案，以及單一檔案 extras（例如共用 `AGENTS.md`）——使用的是真正的 symlink，這需要[開發人員模式](#file-links-need-windows-developer-mode-copying-instead)（或系統管理員身分的 shell）。沒有開啟時，skillshare 會改為複製這些檔案。

---

## 檔案位置

### Windows 上的設定檔在哪裡？

```
%AppData%\skillshare\config.yaml
%AppData%\skillshare\skills\
%AppData%\skillshare\backups\
```

通常是：
```
C:\Users\YourName\AppData\Roaming\skillshare\
```

### Target 目錄在哪裡？

```
%USERPROFILE%\.claude\skills\
%USERPROFILE%\.cursor\skills\
%USERPROFILE%\.codex\skills\
```

---

## 環境變數

### 我要如何在 Windows 上設定 GITHUB_TOKEN？

**僅限目前的 session：**
```powershell
$env:GITHUB_TOKEN = "ghp_your_token"
```

**永久設定（使用者層級）：**
```powershell
[Environment]::SetEnvironmentVariable("GITHUB_TOKEN", "ghp_your_token", "User")
```

**接著重新啟動 PowerShell。**

### 我要如何設定 SKILLSHARE_CONFIG？

```powershell
$env:SKILLSHARE_CONFIG = "C:\path\to\custom\config.yaml"
skillshare status
```

---

## 常見問題

### `junction creation failed`

**原因：** Target 路徑已存在為檔案或不相容的類型。

**解決方法：**
```powershell
# 備份並移除既有內容
skillshare backup
Remove-Item -Path "$env:USERPROFILE\.claude\skills" -Recurse -Force
skillshare sync
```

### `path too long`

**原因：** Windows 預設有 260 字元的路徑長度限制。

**解決方法：** 啟用長路徑支援：
```powershell
# 以系統管理員身分執行
Set-ItemProperty -Path "HKLM:\SYSTEM\CurrentControlSet\Control\FileSystem" -Name "LongPathsEnabled" -Value 1
```

接著重新啟動。

### `access denied`

**原因：** 檔案或目錄正在使用中或受到保護。

**解決方法：**
1. 關閉任何正在使用這些檔案的程式
2. 檢查防毒軟體是否封鎖
3. 以系統管理員身分執行 PowerShell（很少需要）

### `file links need Windows Developer Mode; copying instead` {#file-links-need-windows-developer-mode-copying-instead}

**原因：** Windows 只有在開啟開發人員模式時（或從系統管理員身分的 shell）才允許建立檔案 symlink。`merge` 模式的 agents、`merge` 模式的目錄型 extras，以及單一檔案 extras（例如共用 `AGENTS.md`）都會連結單一檔案，所以沒有開發人員模式時，`sync` 會改為複製它們，並顯示這一行。`status`、`doctor` 與 `extras list` 會把這些 target 顯示為 `copy`。

這些副本會被追蹤，所以對 sync 來說，它們的行為仍然和連結一樣：

- Source 變更時會更新它們，source 被移除時也會移除它們。
- 你自己在 target 中建立的檔案不會被動到，與 `merge` 模式相同。
- 檔案連結可用後，下一次 sync 會把副本換成連結。

**解決方法：** 不需要做任何事；副本會繼續正常運作。若想改用連結，請開啟開發人員模式（Windows 11：**設定 → 系統 → 開發人員專用 → 開發人員模式**；Windows 10：**設定 → 更新與安全性 → 開發人員專用**），然後再執行一次 `skillshare sync --all`。如果 `skillshare ui` 正在執行，請重新啟動它。

內容相同的本機檔案會顯示為 `local preserved`；`sync extras` 不會為它們建議使用 `--force`。它們仍是本機檔案，不是受管理的連結。

### Agent 檔案或 AGENTS.md 顯示為資料夾圖示且無法讀取 {#agent-files-or-agentsmd-show-a-folder-icon-and-cant-be-read}

**原因：** 舊版本用目錄 junction 連結單一檔案。檔案總管會把該檔案顯示成資料夾，工具也讀不到它。

**解決方法：** 執行 `skillshare sync --all`（或 `skillshare sync agents` / `skillshare sync extras`）。Sync 會把這些損壞的連結換成檔案 symlink；關閉開發人員模式時則換成副本。在 dashboard 的 **AGENTS.md** 分頁上，受影響的 target 會顯示警告；把它切換成 `copy` 也能修正。

### `symlinks not working`

**注意：** 在 Windows 上，skillshare 以 NTFS junction 連結資料夾，以 symlink 連結單一檔案。如果你看到 symlink 相關的錯誤，請確認你使用的是 Windows 版本的 skillshare。

### Antigravity 出現 `Incorrect function`

Antigravity 無法遍歷 junction。請參閱 [Antigravity 無法載入已同步的 Skill](./common-errors.md#antigravity-does-not-load-synced-skills)。

---

## PowerShell 提示

### 別名

加入你的 PowerShell profile（`$PROFILE`）：
```powershell
Set-Alias -Name ss -Value skillshare
function sss { skillshare sync }
function ssp { param($m) skillshare push -m $m }
function ssl { skillshare pull }
```

### 檢查 PowerShell 版本

skillshare 可搭配 PowerShell 5.1+ 與 PowerShell Core 7+ 使用：
```powershell
$PSVersionTable.PSVersion
```

---

## WSL 相容性

如果你使用 Windows Subsystem for Linux：

### 分開安裝

為 Windows 與 WSL 保留各自獨立的 skillshare 安裝：
- Windows：`%AppData%\skillshare\`
- WSL：`~/.config/skillshare/`

### 透過 git 共享

使用同一個 git remote 在兩者之間同步：
```bash
# Windows
skillshare push -m "From Windows"

# WSL
skillshare pull
```

---

## 取得協助

在錯誤回報中附上：
- Windows 版本：`winver`
- PowerShell 版本：`$PSVersionTable.PSVersion`
- skillshare 版本：`skillshare --version`
- 完整錯誤訊息

---

## 相關文件

- [常見錯誤](./common-errors.md) — 一般錯誤的解決方法
- [設定](/docs/reference/targets/configuration) — 設定檔參考
