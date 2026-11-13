---
sidebar_position: 1
---

# トラブルシューティング

問題が発生していますか？まずはここから。

## クイック診断

doctor コマンドを実行してください。

```bash
skillshare doctor
```

これは以下をチェックします。
- Source ディレクトリ
- Config ファイル
- Target のアクセス可能性
- シンボリックリンクの健全性
- Git のステータス

---

## 何が起きていますか？

| 問題 | 参照先 |
|---------|-------|
| エラーメッセージが表示される | [よくあるエラー](./common-errors.md) |
| Windows で何かが動作しない | [Windows](./windows.md) |
| 段階的なデバッグ手順が必要 | [トラブルシューティングワークフロー](./troubleshooting-workflow.md) |
| 一般的な質問がある | [FAQ](./faq.md) |

---

## クイックフィックス

### Skill が表示されない

```bash
skillshare sync
```

### 壊れたシンボリックリンク

```bash
skillshare sync --force
```

### Config の問題

```bash
skillshare doctor
```

### 最初からやり直す

```bash
rm ~/.config/skillshare/config.yaml
skillshare init
```

---

## ヘルプを得る

問題が解決しない場合:

1. **情報を集める:**
   ```bash
   skillshare doctor
   skillshare status
   ```

2. **既存の Issue を検索する:** [GitHub Issues](https://github.com/runkids/skillshare/issues)

3. 以下を含めて **新しい Issue を開く**:
   - doctor の出力
   - エラーメッセージ
   - 何をしようとしていたか
   - オペレーティングシステム

---

## 関連項目

- [トラブルシューティングワークフロー](./troubleshooting-workflow.md) — 段階的なデバッグ
- [Commands: doctor](/docs/reference/commands/doctor) — doctor コマンドの詳細
