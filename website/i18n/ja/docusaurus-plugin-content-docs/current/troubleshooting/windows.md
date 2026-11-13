---
sidebar_position: 3
---

# Windows

Windows 固有の問題と解決方法です。

## インストール

### Windows にはどうやってインストールしますか？

**PowerShell:**
```powershell
irm https://raw.githubusercontent.com/runkids/skillshare/main/install.ps1 | iex
```

**または手動でダウンロードする:**
1. [リリース](https://github.com/runkids/skillshare/releases) にアクセスする
2. Windows 用の `.zip` をダウンロードする
3. 展開して PATH に追加する

---

## 権限

### skillshare には管理者権限が必要ですか？

**いいえ。** フォルダー（skills、および `symlink` モードの agents や extras）は NTFS ジャンクションで
リンクされます。ジャンクションはディレクトリに対してシンボリックリンクのように機能し、管理者権限は不要です。

単一のファイルはジャンクションでリンクできません。ファイルをリンクするモード（`merge` モードの agents、
`merge` モードのディレクトリ Extras 内のファイル、共有 `AGENTS.md` などの単一ファイルの Extras）は
本物のシンボリックリンクを使うため、[Developer Mode](#file-links-need-windows-developer-mode-copying-instead)
（または管理者シェル）が必要です。Developer Mode がない場合、skillshare はそれらのファイルを代わりに
コピーします。

---

## ファイルの場所

### Windows での Config ファイルはどこにありますか？

```
%AppData%\skillshare\config.yaml
%AppData%\skillshare\skills\
%AppData%\skillshare\backups\
```

通常は次の場所です。
```
C:\Users\YourName\AppData\Roaming\skillshare\
```

### Target ディレクトリはどこにありますか？

```
%USERPROFILE%\.claude\skills\
%USERPROFILE%\.cursor\skills\
%USERPROFILE%\.codex\skills\
```

---

## 環境変数

### Windows で GITHUB_TOKEN を設定するには？

**現在のセッションのみ:**
```powershell
$env:GITHUB_TOKEN = "ghp_your_token"
```

**永続化（ユーザーレベル）:**
```powershell
[Environment]::SetEnvironmentVariable("GITHUB_TOKEN", "ghp_your_token", "User")
```

**その後 PowerShell を再起動してください。**

### SKILLSHARE_CONFIG を設定するには？

```powershell
$env:SKILLSHARE_CONFIG = "C:\path\to\custom\config.yaml"
skillshare status
```

---

## よくある問題

### `junction creation failed`

**原因:** Target のパスがすでにファイルまたは互換性のない種類として存在している。

**解決策:**
```powershell
# バックアップして既存のものを削除する
skillshare backup
Remove-Item -Path "$env:USERPROFILE\.claude\skills" -Recurse -Force
skillshare sync
```

### `path too long`

**原因:** Windows はデフォルトで260文字のパス長制限がある。

**解決策:** 長いパスを有効にしてください。
```powershell
# 管理者として実行する
Set-ItemProperty -Path "HKLM:\SYSTEM\CurrentControlSet\Control\FileSystem" -Name "LongPathsEnabled" -Value 1
```

その後再起動してください。

### `access denied`

**原因:** ファイルまたはディレクトリが使用中、または保護されている。

**解決策:**
1. ファイルを使用しているプログラムを閉じる
2. アンチウイルスがブロックしていないか確認する
3. PowerShell を管理者として実行する（ほとんど必要ない）

### `file links need Windows Developer Mode; copying instead` {#file-links-need-windows-developer-mode-copying-instead}

**原因:** Windows でファイルのシンボリックリンクを作成できるのは、Developer Mode がオンのとき（または
管理者シェルから実行したとき）だけです。`merge` モードの agents、`merge` モードのディレクトリ Extras、
単一ファイルの Extras（共有 `AGENTS.md` など）は単一のファイルをリンクするため、Developer Mode が
ない場合 `sync` は代わりにそれらをコピーし、この行を表示します。`status`、`doctor`、`extras list` では
これらの Target が `copy` と表示されます。

コピーは追跡されるため、sync に関してはリンクと同じように動作します。

- ソースが変更されると更新され、ソースが削除されると削除されます。
- Target 内で自分で作成したファイルは、`merge` モードと同様にそのまま残ります。
- ファイルのリンクが使えるようになると、次の sync でコピーがリンクに置き換えられます。

**解決策:** 何もする必要はありません。コピーはそのまま機能します。リンクにしたい場合は Developer Mode を
オンにし（Windows 11: **設定 → システム → 開発者向け → 開発者モード**、Windows 10: **設定 → 更新と
セキュリティ → 開発者向け**）、もう一度 `skillshare sync --all` を実行してください。`skillshare ui` が
起動している場合は再起動してください。

内容が同じローカルファイルは `local preserved` と表示され、`sync extras` はそれらに `--force` を提案しません。管理対象のリンクにはならず、ローカルファイルのままです。

### Agent ファイルや AGENTS.md がフォルダーのアイコンで表示され、読み込めない {#agent-files-or-agentsmd-show-a-folder-icon-and-cant-be-read}

**原因:** 以前のバージョンは、単一のファイルをディレクトリジャンクションでリンクしていました。エクスプローラーは
そのファイルをフォルダーとして表示し、ツールはそれを読み込めません。

**解決策:** `skillshare sync --all`（または `skillshare sync agents` / `skillshare sync extras`）を
実行してください。sync はこの壊れたリンクを、ファイルのシンボリックリンク、または Developer Mode が
オフの場合はコピーに置き換えます。ダッシュボードの **AGENTS.md** タブでは、影響を受ける Target に警告が
表示され、`copy` に切り替えることでも直ります。

### `symlinks not working`

**注記:** Windows では、skillshare はフォルダーを NTFS ジャンクションで、単一のファイルをシンボリック
リンクでリンクします。シンボリックリンクのエラーが表示される場合は、Windows 版の skillshare を使用していることを
確認してください。

### Antigravity での `Incorrect function`

Antigravity はジャンクションをたどれません。
[Antigravity が Sync された Skill を読み込まない](./common-errors.md#antigravity-does-not-load-synced-skills)
を参照してください。

---

## PowerShell のヒント

### エイリアス

PowerShell プロファイル（`$PROFILE`）に追加してください。
```powershell
Set-Alias -Name ss -Value skillshare
function sss { skillshare sync }
function ssp { param($m) skillshare push -m $m }
function ssl { skillshare pull }
```

### PowerShell のバージョンを確認する

skillshare は PowerShell 5.1 以降および PowerShell Core 7 以降で動作します。
```powershell
$PSVersionTable.PSVersion
```

---

## WSL との互換性

Windows Subsystem for Linux を使用している場合:

### インストールを分離する

Windows と WSL 用に別々の skillshare インストールを維持してください。
- Windows: `%AppData%\skillshare\`
- WSL: `~/.config/skillshare/`

### git 経由で共有する

同じ git リモートを使ってそれらの間で Sync してください。
```bash
# Windows
skillshare push -m "From Windows"

# WSL
skillshare pull
```

---

## ヘルプを得る

バグ報告には以下を含めてください。
- Windows のバージョン: `winver`
- PowerShell のバージョン: `$PSVersionTable.PSVersion`
- skillshare のバージョン: `skillshare --version`
- 完全なエラーメッセージ

---

## 関連項目

- [よくあるエラー](./common-errors.md) — 一般的なエラーの解決方法
- [Configuration](/docs/reference/targets/configuration) — Config ファイルリファレンス
