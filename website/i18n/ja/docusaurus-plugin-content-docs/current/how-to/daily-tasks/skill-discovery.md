---
sidebar_position: 3
---

# Skill の発見

コミュニティから Skill を見つけ、評価し、インストールします。

## 概要

```mermaid
flowchart LR
    SEARCH["SEARCH"] --> BROWSE["BROWSE"] --> EVALUATE["EVALUATE"] --> INSTALL["INSTALL"] --> SYNC["SYNC"]
```

---

## ステップ 1: 検索する

キーワードで Skill を探すか、人気の Skill を閲覧します。

```bash
skillshare search              # 人気の Skill を閲覧
skillshare search pdf
skillshare search "code review"
skillshare search react
```

---

## ステップ 2: リポジトリを閲覧する

リポジトリ内の Skill を探索します。

```bash
# Anthropic 公式の Skill
skillshare install anthropics/skills

# コミュニティの Skill
skillshare install ComposioHQ/awesome-claude-skills
```

これにより**発見モード**に入ります — リポジトリ内で利用可能なすべての Skill が表示されます。

---

## ステップ 3: 評価する

インストール前に、以下を検討してください。

- **自分の問題を解決するか？** description を読む
- **よくメンテナンスされているか？** リポジトリの活動状況を確認する
- **衝突しないか？** 既存の Skill との名前の衝突を確認する

何がインストールされるかをプレビューします。
```bash
skillshare install anthropics/skills/skills/pdf --dry-run
```

---

## ステップ 4: インストールする

### 単一の Skill

```bash
skillshare install anthropics/skills/skills/pdf
```

### 1つのリポジトリから複数の Skill

```bash
# インタラクティブに閲覧
skillshare install anthropics/skills

# 特定の Skill を選択する（非インタラクティブ）
skillshare install anthropics/skills -s pdf,commit

# すべての Skill をインストールする
skillshare install anthropics/skills --all
```

### リポジトリ全体（チーム向け）

```bash
skillshare install github.com/team/skills --track
```

---

## ステップ 5: Sync する

インストール後に Sync するのを忘れないでください。

```bash
skillshare sync
```

---

## 人気の Skill ソース

| ソース | URL |
|--------|-----|
| Anthropic 公式 | `anthropics/skills` |
| Vercel Agent Skills | `vercel-labs/agent-skills` |
| コミュニティ | [skillsmp.com](https://skillsmp.com/) |

---

## 発見コマンド

| コマンド | 目的 |
|---------|------|
| `search` | 人気の Skill を閲覧する |
| `search <query>` | Skill を検索する |
| `check` | 利用可能な更新を確認する |
| `install <repo>` | リポジトリを閲覧する（発見モード） |
| `install <repo/path>` | 特定の Skill をインストールする |
| `list` | インストール済みの Skill を表示する |

---

## インストールオプション

```bash
# カスタム名
skillshare install anthropics/skills/skills/pdf --name my-pdf

# 強制上書き
skillshare install anthropics/skills/skills/pdf --force

# 既存のものを更新
skillshare install anthropics/skills/skills/pdf --update

# チーム共有用に Track する
skillshare install github.com/team/skills --track
```

`--name` はインストール対象が単一の Skill の場合にのみ有効です。
複数の Skill を返すリポジトリ発見モードで `--name` を使うとエラーになります。

---

## インストール後

### 確認する

```bash
skillshare list
skillshare status
```

### テストする

AI CLI で Skill を使い、期待通りに動作するか確認します。

### 更新を確認する

```bash
skillshare check              # 利用可能な更新を確認
```

### 後で更新する

```bash
# 単一の Skill（Source メタデータ付き）
skillshare install pdf --update

# Tracked repo
skillshare update _team-skills
```

---

## 関連項目

- [search](/docs/reference/commands/search) — Search コマンドリファレンス
- [install](/docs/reference/commands/install) — Install コマンドリファレンス
- [Hub Index](/docs/how-to/sharing/hub-index) — Skill Hub の管理
- [日々のワークフロー](./daily-workflow.md) — インストール後の日常的な使用
