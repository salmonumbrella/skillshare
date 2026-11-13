---
sidebar_position: 3
---

# カスタム Target の追加

Skill ディレクトリを持つ任意のツールを skillshare に追加します。

## 概要

あなたの AI CLI が [対応リスト](./supported-targets.md) にない場合、手動で追加できます。

---

## Target を追加する

```bash
skillshare target add <name> <path>
```

### 例

```bash
skillshare target add aider ~/.aider/skills
skillshare sync
```

---

## 要件

### パスが存在している必要がある

必要であれば先にディレクトリを作成してください。

```bash
mkdir -p ~/.myapp/skills
skillshare target add myapp ~/.myapp/skills
```

### パスは `/skills` で終わることが望ましい

これは推奨ですが必須ではありません。

```bash
# 推奨
skillshare target add myapp ~/.myapp/skills

# これも動作する
skillshare target add myapp ~/.myapp/prompts
```

---

## 確認する

追加後:

```bash
# Target を確認する
skillshare target myapp

# 新しい Target に Sync する
skillshare sync

# 確認する
skillshare status
```

---

## よくあるシナリオ

### 新しい AI CLI ツールを追加する

```bash
# 1. ツールが Skill をどこに保存しているか調べる
# （ツールのドキュメントを確認する）

# 2. 必要ならディレクトリを作成する
mkdir -p ~/.newtool/skills

# 3. Target として追加する
skillshare target add newtool ~/.newtool/skills

# 4. Sync する
skillshare sync
```

### プロジェクト固有の Target を追加する

```bash
# 特定のプロジェクトに Skill を Sync する
skillshare target add myproject ~/projects/myapp/.ai/skills
skillshare sync
```

### 複数のツールを追加する

```bash
skillshare target add tool1 ~/.tool1/skills
skillshare target add tool2 ~/.tool2/skills
skillshare target add tool3 ~/.tool3/skills
skillshare sync
```

---

## Sync モードを変更する

追加後、Sync モードを変更できます。

```bash
# デフォルトは merge モード
skillshare target myapp --mode symlink
skillshare sync
```

詳細は [Sync モード](/docs/understand/sync-modes) を参照してください。

---

## Target を削除する

もう Target が不要な場合:

```bash
skillshare target remove myapp
```

これは:
1. バックアップを作成する
2. シンボリックリンクを実ファイルに置き換える（merge モードでは、Source が管理するシンボリックリンク
   のみが削除され、ローカルの Skill は保持される）
3. Config から削除する

---

## トラブルシューティング

### 「path does not exist」

先にディレクトリを作成してください。

```bash
mkdir -p ~/.myapp/skills
skillshare target add myapp ~/.myapp/skills
```

### Target が Sync されない

Target が有効になっているか確認してください。

```bash
skillshare target list
skillshare target myapp
```

### パスが間違っている

削除して再追加してください。

```bash
skillshare target remove myapp
skillshare target add myapp /correct/path/skills
```

---

## 関連項目

- [対応する Target](./supported-targets.md) — 組み込みの Target
- [Configuration](./configuration.md) — Config を直接編集する
- [Sync モード](/docs/understand/sync-modes) — merge、copy、symlink
