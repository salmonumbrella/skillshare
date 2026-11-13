---
sidebar_position: 7
---

# AI 支援開発

> ビルド済みの Project mode Skill を使って、AI コーディング Agent で skillshare にコントリビュートする。

## 前提条件

- AI コーディング Agent（Claude Code、Codex など）
- ローカルに clone された skillshare リポジトリ

## セットアップ

このリポジトリには `.skillshare/skills/` に Project mode の Skill が同梱されています。Agent に
Sync してください。

```bash
skillshare sync -p
```

これで、あなたの AI Agent はこのコードベースで作業するための専用の Skill にアクセスできます。

## 利用可能な Skill

| Skill | 何をするか |
|-------|-------------|
| `skillshare-implement-feature` | TDD ワークフローを使い、spec ファイルまたは説明から機能を実装する |
| `skillshare-update-docs` | すべてのフラグを Source と照合しながら、Web サイトのドキュメントを最近のコード変更に合わせて更新する |
| `skillshare-code-review` | 変更を並列パス（正確性、リポジトリのルール、仕様、利用箇所、クリーンアップ）でレビューし、各指摘を検証する |
| `skillshare-cli-e2e-test` | runbook から devcontainer 内で隔離された E2E テストを実行する |
| `skillshare-changelog` | 最近のコミットから conventional な形式で CHANGELOG.md のエントリを生成する |

## 典型的なワークフロー

1. **機能を始める** — Agent に spec を使って `skillshare-implement-feature` を使うよう依頼する
2. **ドキュメントを更新する** — コード変更後、`skillshare-update-docs` を呼び出して Web サイトのドキュメントを Sync する
3. **変更をレビューする** — コミットや PR の前に `skillshare-code-review` を実行する
4. **E2E テストを実行する** — サンドボックスで検証するために `skillshare-cli-e2e-test` を使う
5. **changelog を書く** — リリース前に `skillshare-changelog` を呼び出す

## 次のステップ

- [Dev Containers セットアップ →](/docs/learn/with-devcontainer)
- [インタラクティブプレイグラウンド →](/docs/learn/with-playground)
- [コントリビューションガイド →](https://github.com/runkids/skillshare/blob/main/CONTRIBUTING.md)
