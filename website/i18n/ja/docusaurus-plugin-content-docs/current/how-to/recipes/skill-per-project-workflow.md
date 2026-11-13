---
sidebar_position: 4
---

# レシピ: Project Mode ワークフロー

> コードベースと共に移動するプロジェクトスコープの Skill を管理する。

## シナリオ

特定の Skill をプロジェクトリポジトリにコミットしておきたい理由は次の通りです。
- すべてのコントリビューターが同じ AI 指示を得られる
- Skill がコードと一緒にバージョン管理される
- リポジトリを clone する以外の手動セットアップが不要になる

## 解決策

### ステップ 1: Project mode を初期化する

```bash
cd your-project
skillshare init -p
```

これによりプロジェクトのルートに `.skillshare/config.yaml` が作成されます。

### ステップ 2: プロジェクトスコープの Skill をインストールする

```bash
skillshare install anthropics/courses/prompt-eng -p
skillshare install your-org/team-skills --skill code-review -p
```

Skill は `.skillshare/skills/` に配置されます。

### ステップ 3: プロジェクトの Target に Sync する

```bash
skillshare sync -p
```

これにより `.skillshare/skills/` からプロジェクトレベルの Target ディレクトリ（例: `.claude/skills/`、
`.cursor/skills/`）へのシンボリックリンクが作成されます。

### ステップ 4: バージョン管理にコミットする

```bash
git add .skillshare/
git commit -m "Add project skills"
```

### ステップ 5: チームメンバーのセットアップ

チームメンバーがリポジトリを clone すると:

```bash
git clone your-org/your-project
cd your-project
skillshare sync -p
```

1つのコマンドで、すべてのプロジェクトの Skill が彼らのローカルの AI ツールに Sync されます。

## 確認

- プロジェクトのルートに `.skillshare/config.yaml` が存在する
- `.skillshare/skills/` にインストールされた Skill が含まれる
- `skillshare list -p` がプロジェクトの Skill を表示する
- `sync -p` の後、Target ディレクトリにシンボリックリンクが含まれる

## バリエーション

- **Dev Container の自動 Sync**: `.devcontainer/devcontainer.json` の `postCreateCommand` に
  `skillshare sync -p` を追加する
- **混合モード**: 個人の好みには Global の Skill を、チーム標準にはプロジェクトの Skill を使う
- **CI 検証**: プロジェクトの Skill を検証するため、CI パイプラインに `skillshare audit -p` を追加する

## 関連項目

- [プロジェクトセットアップガイド](/docs/how-to/sharing/project-setup)
- [プロジェクトの Skill を理解する](/docs/understand/project-skills)
- [Dev Container ガイド](/docs/learn/with-devcontainer)
