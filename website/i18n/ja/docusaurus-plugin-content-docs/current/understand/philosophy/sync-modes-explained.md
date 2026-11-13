---
sidebar_position: 5
---

# Sync モード詳解

> 3 つの Sync モード——merge、copy、symlink——を深掘りし、それぞれをいつ使うべきか、どのようなトレードオフがあるかを解説します。

## 3 つのモード

skillshare には、Skill を Source ディレクトリから AI ツールの Target ディレクトリへどのように配信するかを制御する 3 つの Sync モードがあります。

### Merge モード（デフォルト）

```
Source: ~/.config/skillshare/skills/
├── code-review/SKILL.md
├── testing/SKILL.md
└── debugging/SKILL.md

Target: ~/.claude/skills/
├── code-review → ~/.config/skillshare/skills/code-review  (symlink)
├── testing → ~/.config/skillshare/skills/testing           (symlink)
├── debugging → ~/.config/skillshare/skills/debugging       (symlink)
└── my-local-skill/SKILL.md                                 (untouched)
```

**仕組み**: Skill ごとに 1 つのシンボリックリンクを作成します。Target 内の各 Skill ディレクトリは Source を指します。

**重要な性質**: **非破壊的**であることです。Target ディレクトリ内のローカル Skill（上記の `my-local-skill` など）はそのまま保持されます。skillshare は自身が作成したシンボリックリンクのみを管理します。

### Copy モード

```
Source: ~/.config/skillshare/skills/
├── code-review/SKILL.md
├── testing/SKILL.md
└── debugging/SKILL.md

Target: ~/.cursor/skills/
├── code-review/SKILL.md                (physical copy)
├── testing/SKILL.md                    (physical copy)
├── debugging/SKILL.md                  (physical copy)
├── .skillshare-manifest.json           (tracks managed files)
└── my-local-skill/SKILL.md             (untouched)
```

**仕組み**: 各 Skill を物理的に Target にコピーします。`.skillshare-manifest.json` ファイルが、管理対象の Skill とその SHA-256 チェックサムを記録します。以降の Sync では、変更された Skill のみが再コピーされます。

**重要な性質**: **最大限の互換性**です。シンボリックリンクのサポートが不要なため、どこでも動作します。ローカル Skill は merge モードと同様に保持されます。

### Symlink モード

```
Source: ~/.config/skillshare/skills/
├── code-review/SKILL.md
├── testing/SKILL.md
└── debugging/SKILL.md

Target: ~/.claude/skills → ~/.config/skillshare/skills/  (single symlink)
```

**仕組み**: Target ディレクトリ全体を、Source を指す単一のシンボリックリンクに置き換えます。

**重要な性質**: **完全な制御**です。Target は Source そのものになります。Target 内にローカル Skill は存在できません。

## どれを使うべきか

| 要因 | Merge | Copy | Symlink |
|--------|-------|------|---------|
| ローカル Skill を保持する | Yes | Yes | No |
| クロスプラットフォーム対応 | 問題が生じる場合がある | どこでも動作する | 問題が生じる場合がある |
| Source の変更が反映されるタイミング | 即座に | `sync` 実行後に | 即座に |
| ネストしたパスの扱い | フラット化される（`a/b/c` → `a__b__c`） | フラット化される | ネイティブな構造のまま |
| 孤立ファイルの自動削除 | 自動 | 自動 | 不要 |
| ディスク使用量 | 最小（シンボリックリンク） | フルコピー | 最小（1 つのシンボリックリンク） |
| 推奨対象 | ほとんどのユーザー | WSL、Docker、CI | 単一ソースのセットアップ |

### Merge を選ぶ場合

- AI ツール内に、skillshare で管理したくないローカル Skill がある
- 異なるローカルカスタマイズを持つ複数の AI ツールを使用している
- skillshare を段階的に導入している（一部の Skill は管理し、一部は管理しない）

### Copy を選ぶ場合

- プラットフォームのシンボリックリンクサポートが不安定である（WSL、一部の Docker 環境）
- AI ツールがシンボリックリンクを正しく辿らない
- CI/CD パイプラインやコンテナ環境で使用している
- Target が Source ディレクトリから独立して機能することを望んでいる

### Symlink を選ぶ場合

- skillshare がその Target における唯一の Skill ソースである
- Target の中身について曖昧さをゼロにしたい
- 新しい環境をセットアップしている最中である

## ネストしたパスの扱い

merge モードと copy モードでは、ネストした Source のパスは二重アンダースコアでフラット化されます。

```
Source: skills/frontend/react-patterns/SKILL.md
Target: ~/.claude/skills/frontend__react-patterns → skills/frontend/react-patterns
```

これにより、フラットな Skill 構造を前提とする Target でのディレクトリ作成を回避できます。symlink モードでは、ディレクトリ構造はそのまま保持されます。

## 孤立ファイルの自動削除

merge モードと copy モードは、`skillshare sync` 実行時に孤立したエントリを自動的に削除します。Source から Skill をアンインストールすると、Target 内の対応するシンボリックリンク（またはコピーされたディレクトリ）は次回の Sync 時にクリーンアップされます。

```bash
skillshare uninstall old-skill
skillshare sync
# → Pruned orphan: old-skill
```

## Target ごとのモード上書き

Target ごとに異なるモードを設定できます。グローバル設定では map 形式を使用します。

```yaml
targets:
  claude:
    path: ~/.claude/skills
    mode: merge
  cursor:
    path: ~/.cursor/skills
    mode: copy
```

プロジェクト設定では list 形式を使用します。

```yaml
targets:
  - name: claude
    mode: merge
  - name: cursor
    mode: copy
```

または CLI 経由でモードを変更します。

```bash
skillshare target claude --mode copy
```

## 関連項目

- [Sync モードのコンセプトページ](/docs/understand/sync-modes)
- [`sync` コマンドリファレンス](/docs/reference/commands/sync)
- [Source と Target](/docs/understand/source-and-targets)
