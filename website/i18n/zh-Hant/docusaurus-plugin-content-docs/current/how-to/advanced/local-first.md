---
sidebar_position: 10
---

# 集中式平台 vs Local-First

Skill 管理工具通常遵循兩種架構方式之一：**集中式平台（centralized platforms）**或 **local-first（本機優先）**。沒有哪一種絕對更好 — 每種都有各自的權衡。本頁會介紹這兩種方式，幫助你判斷哪一種適合你的工作流程。

:::tip 這不是功能比較
關於功能層級的差異（安裝流程、設定檔格式等），請參閱 [Comparing Skill Management Approaches](/docs/understand/philosophy/comparison)。本頁專注於**架構層面的權衡** — 你的資料存放在哪裡、探索機制如何運作，以及你能掌控什麼。
:::

## 兩種方式

### 集中式平台

集中式平台會提供一個統一的註冊中心，Skill 在其中發布、搜尋與排名。安裝活動會被彙整成社群指標，例如下載次數與熱門排行。

**優勢**：
- 內建探索功能 — 可以在同一處瀏覽、搜尋、比較 Skill
- 社群訊號 — 下載次數與熱門排行有助於發現受歡迎的 Skill
- 低門檻 — 探索不需要任何設定，直接搜尋並安裝即可

**須留意之處**：
- 安裝活動會被平台追蹤
- 排名與計數規則由平台營運方管理

### Local-First（skillshare）

skillshare 將所有狀態保存在你的機器上。Skill 透過 `git clone` 安裝，並透過本機設定檔管理。不會有任何資料傳送到遠端伺服器。

**優勢**：
- 零遙測 — 沒有安裝追蹤，也不會傳送任何資料
- 完全擁有權 — 你的 Skill 存放在你自己的檔案系統中
- 初次安裝後可離線使用
- 單一執行檔，沒有執行期依賴

**須留意之處**：
- 沒有內建的社群指標（下載次數、熱門排行）
- 探索功能需要自行建立或連接 Hub

## 探索機制

Local-first 並不代表沒有探索功能。skillshare 提供三種探索管道：

| 管道 | 運作方式 |
|---------|-------------|
| **GitHub search** | `skillshare search <query>` — 直接搜尋公開的 GitHub repos |
| **Public hub** | `skillshare search --hub` — 查詢內建的[社群 Hub](https://github.com/runkids/skillshare-hub) |
| **Custom hub** | `skillshare search --hub <url>` — 查詢你或你的組織維護的任何 Hub |

### 什麼是 Hub？

Hub 是一個靜態 JSON 檔案（`skillshare-hub.json`），列出 Skill 的名稱、描述、Source 與標籤。它可以存放在任何地方 — Git repo、HTTP 伺服器，或本機檔案系統：

```bash
# 從你已安裝的 Skill 建立索引
skillshare hub index

# 搜尋某組織的內部 Hub
skillshare search --hub https://internal.corp/skills/hub.json

# 搜尋本機的索引檔
skillshare search --hub ./skillshare-hub.json
```

Hub 彼此獨立 — 任何人都可以建立一個，使用者也可以同時連接多個 Hub。這讓 Hub 非常適合需要在維護公開目錄的同時，也維護私有 Skill 目錄的組織。

詳細操作步驟請參閱 [Hub Index Guide](/docs/how-to/sharing/hub-index)。

### 自架指標

skillshare 本身不會追蹤安裝次數，但如果你在自己的伺服器上架設 Hub，可以自行加上任何合適的分析層：

1. 在你的伺服器上架設 `skillshare-hub.json`
2. 加入請求日誌記錄或輕量的分析端點
3. 追蹤搜尋命中次數、安裝來源，或任何你在意的指標

這讓 Skill 作者或組織可以按照自己的方式衡量採用情況。

## 選擇適合的方式

**如果符合以下情況，集中式平台可能更適合你：**
- 你想要開箱即用的內建社群指標與熱門排行
- 你偏好單一瀏覽入口來探索 Skill
- 你只使用一個 AI CLI，不需要跨工具 Sync

**如果符合以下情況，local-first 可能更適合你：**
- 你使用多個 AI CLI，並想要統一管理
- 你希望安裝活動保留在自己的機器上
- 你需要離線運作，或在受限的網路環境中工作
- 你是需要掌控哪些 Skill 可用、可被探索的組織

---

## 參見

- [Comparing Skill Management Approaches](/docs/understand/philosophy/comparison) — 功能層級的比較
- [Hub Index Guide](/docs/how-to/sharing/hub-index) — 建立與使用 Skill Hub
- [hub command](/docs/reference/commands/hub) — hub 指令參考
- [Security Guide](./security.md) — Skill 安全性掃描
