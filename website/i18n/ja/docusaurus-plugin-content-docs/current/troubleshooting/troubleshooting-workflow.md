---
sidebar_position: 2
---

# トラブルシューティングワークフロー

問題を診断・修正するための体系的なアプローチです。

## 概要

```mermaid
flowchart LR
    DIAGNOSE["診断"] --> IDENTIFY["特定"] --> FIX["修正"] --> VERIFY["検証"]
```

---

## ステップ1: 診断する

doctor コマンドを実行してください。

```bash
skillshare doctor
```

**チェックされる内容:**
- Source ディレクトリが存在し、有効であること
- Config ファイルが正しくフォーマットされていること
- すべての Target にアクセス可能であること
- シンボリックリンクが壊れていないこと
- Git リポジトリのステータス（初期化されている場合）
- Skill フォーマットの妥当性

---

## ステップ2: 問題を特定する

### よくある症状と原因

| 症状 | 考えられる原因 | クイックフィックス |
|---------|--------------|-----------|
| AI CLI に Skill が表示されない | Sync されていない | `skillshare sync` |
| シンボリックリンクが壊れている | Source が削除された | 復元または再インストール |
| Config エラー | 無効な YAML | `skillshare doctor` が詳細を表示する |
| push/pull ができない | Git の問題 | 手動で git のステータスを確認する |
| Permission denied | 所有権が間違っている | ファイルの権限を確認する |

---

## ステップ3: 修正する

### Sync の問題

```bash
# すべての Target を再 Sync する
skillshare sync

# 強制的に Sync する（シンボリックリンクを再作成する）
skillshare sync --force
```

### 壊れたシンボリックリンク

```bash
# ステータスを確認する
skillshare status

# 再作成するために Sync する
skillshare sync
```

### Config の問題

```bash
# 現在の Config を表示する
cat ~/.config/skillshare/config.yaml

# Config をリセットする
rm ~/.config/skillshare/config.yaml
skillshare init
```

### Git の問題

```bash
cd ~/.config/skillshare/skills

# ステータスを確認する
git status

# pull が失敗する（ローカルに変更がある）
git stash
git pull
git stash pop

# push が失敗する（リモートが先行している）
git pull
git push
```

### Target の問題

```bash
# 削除して再追加する
skillshare target remove claude
skillshare target add claude ~/.claude/skills
skillshare sync
```

---

## ステップ4: 検証する

```bash
# ステータスを確認する
skillshare status

# もう一度 doctor を実行する
skillshare doctor

# AI CLI でテストする
# （Skill を呼び出す）
```

---

## 復旧オプション

### 軽度の復旧

```bash
# 再 Sync するだけ
skillshare sync
```

### 中程度の復旧

```bash
# バックアップから復元する
skillshare restore claude
skillshare sync
```

### 重度の復旧（最初からやり直す）

```bash
# 現在の状態をバックアップする
skillshare backup

# Config を削除する（Skill は保持される）
rm ~/.config/skillshare/config.yaml

# 再初期化する
skillshare init

# Sync する
skillshare sync
```

---

## ヘルプを得る

問題が解決しない場合:

1. **情報を集める:**
   ```bash
   skillshare doctor > doctor-output.txt
   skillshare status >> doctor-output.txt
   ```

2. **FAQ を確認する:** [よくあるエラー](/docs/troubleshooting/common-errors)

3. **Issue を報告する:** [GitHub Issues](https://github.com/runkids/skillshare/issues)
   - doctor の出力を含める
   - エラーメッセージを含める
   - 何をしようとしていたかを記述する

---

## 関連項目

- [よくあるエラー](/docs/troubleshooting/common-errors) — エラーメッセージと解決方法
- [Windows の問題](/docs/troubleshooting/windows) — Windows 固有の問題
- [FAQ](/docs/troubleshooting/faq) — よくある質問
- [Commands: doctor](/docs/reference/commands/doctor) — doctor コマンド
