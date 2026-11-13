---
sidebar_position: 7
---

# AI 輔助開發

> 使用 AI coding agent，搭配預先建立好的 project skills 為 skillshare 做出貢獻。

## 先決條件

- 一個 AI coding agent（Claude Code、Codex 等）
- 已在本機 clone 的 skillshare 儲存庫

## 設定

此 repo 在 `.skillshare/skills/` 中內建了 project mode 的 skills。將它們 sync 到你的 agent：

```bash
skillshare sync -p
```

你的 AI agent 現在可以使用專門用於處理此程式碼庫的 skills。

## 可用的 Skills

| Skill | 功能 |
|-------|-------------|
| `skillshare-implement-feature` | 使用 TDD 工作流程，根據 spec 檔案或描述實作功能 |
| `skillshare-update-docs` | 更新網站文件以符合最新的程式碼變更，並交叉驗證每個 flag 是否與原始碼一致 |
| `skillshare-code-review` | 以平行檢查（正確性、儲存庫規則、規格、使用處、清理）審查變更，並逐條驗證發現的問題 |
| `skillshare-cli-e2e-test` | 根據 runbook 在 devcontainer 中執行隔離的 E2E 測試 |
| `skillshare-changelog` | 以 conventional commit 格式，根據最近的 commits 產生 CHANGELOG.md 項目 |

## 典型工作流程

1. **開始一項功能** — 請你的 agent 搭配 spec 使用 `skillshare-implement-feature`
2. **更新文件** — 程式碼變更後，呼叫 `skillshare-update-docs` 以同步網站文件
3. **審查變更** — 提交或開 PR 前執行 `skillshare-code-review`
4. **執行 E2E 測試** — 使用 `skillshare-cli-e2e-test` 在 sandbox 中驗證
5. **撰寫 changelog** — 發布前呼叫 `skillshare-changelog`

## 接下來呢？

- [Dev Containers 設定 →](/docs/learn/with-devcontainer)
- [互動式 Playground →](/docs/learn/with-playground)
- [貢獻指南 →](https://github.com/runkids/skillshare/blob/main/CONTRIBUTING.md)
