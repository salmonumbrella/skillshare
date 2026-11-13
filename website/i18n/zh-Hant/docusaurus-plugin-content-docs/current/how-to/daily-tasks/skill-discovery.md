---
sidebar_position: 3
---

# Skill 探索

從社群尋找、評估並安裝 Skills。

## 總覽

```mermaid
flowchart LR
    SEARCH["搜尋"] --> BROWSE["瀏覽"] --> EVALUATE["評估"] --> INSTALL["安裝"] --> SYNC["Sync"]
```

---

## 步驟 1：搜尋

依關鍵字尋找 Skills，或瀏覽熱門 Skills：

```bash
skillshare search              # 瀏覽熱門 Skills
skillshare search pdf
skillshare search "code review"
skillshare search react
```

---

## 步驟 2：瀏覽 Repositories

探索一個 repository 中的 Skills：

```bash
# 官方 Anthropic Skills
skillshare install anthropics/skills

# 社群 Skills
skillshare install ComposioHQ/awesome-claude-skills
```

這會進入 **discovery mode** — 顯示該 repo 中所有可用的 Skills。

---

## 步驟 3：評估

安裝之前，請考慮：

- **它能解決我的問題嗎？** 閱讀說明
- **維護狀況良好嗎？** 檢查該 repo 的活躍度
- **會不會衝突？** 檢查是否與既有 Skills 有名稱衝突

預覽將會安裝的內容：
```bash
skillshare install anthropics/skills/skills/pdf --dry-run
```

---

## 步驟 4：安裝

### 單一 Skill

```bash
skillshare install anthropics/skills/skills/pdf
```

### 從單一 repo 安裝多個 Skills

```bash
# 互動式瀏覽
skillshare install anthropics/skills

# 選取特定 Skills（非互動式）
skillshare install anthropics/skills -s pdf,commit

# 安裝所有 Skills
skillshare install anthropics/skills --all
```

### 整個 repo（適合團隊使用）

```bash
skillshare install github.com/team/skills --track
```

---

## 步驟 5：同步

安裝後別忘了同步：

```bash
skillshare sync
```

---

## 熱門 Skill 來源

| 來源 | URL |
|--------|-----|
| Anthropic 官方 | `anthropics/skills` |
| Vercel Agent Skills | `vercel-labs/agent-skills` |
| 社群 | [skillsmp.com](https://skillsmp.com/) |

---

## Discovery 指令

| 指令 | 用途 |
|---------|-------|
| `search` | 瀏覽熱門 Skills |
| `search <query>` | 搜尋 Skills |
| `check` | 檢查是否有可用更新 |
| `install <repo>` | 瀏覽 repo（discovery mode） |
| `install <repo/path>` | 安裝特定 Skill |
| `list` | 顯示已安裝的 Skills |

---

## 安裝選項

```bash
# 自訂名稱
skillshare install anthropics/skills/skills/pdf --name my-pdf

# 強制覆寫
skillshare install anthropics/skills/skills/pdf --force

# 更新既有的
skillshare install anthropics/skills/skills/pdf --update

# 標記為 track，供團隊共用
skillshare install github.com/team/skills --track
```

`--name` 只有在安裝目標為單一 Skill 時才有效。
若對會回傳多個 Skills 的 repo discovery 使用 `--name`，將會回傳錯誤。

---

## 安裝之後

### 驗證

```bash
skillshare list
skillshare status
```

### 測試

在你的 AI CLI 中使用該 Skill，確認其運作符合預期。

### 檢查更新

```bash
skillshare check              # 查看有哪些可用的更新
```

### 之後更新

```bash
# 單一 Skill（含來源 metadata）
skillshare install pdf --update

# Tracked repo
skillshare update _team-skills
```

---

## 另請參閱

- [search](/docs/reference/commands/search) — search 指令參考
- [install](/docs/reference/commands/install) — install 指令參考
- [Hub Index](/docs/how-to/sharing/hub-index) — 管理 Skill hub
- [日常工作流程](./daily-workflow.md) — 安裝後，開始日常使用
