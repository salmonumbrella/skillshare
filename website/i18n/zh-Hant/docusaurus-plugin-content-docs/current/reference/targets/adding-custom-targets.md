---
sidebar_position: 3
---

# 新增自訂 Targets

將任何有 Skill 目錄的工具加入 skillshare。

## 總覽

如果你的 AI CLI 不在[支援清單](./supported-targets.md)中，你可以手動新增。

---

## 新增 Target

```bash
skillshare target add <name> <path>
```

### 範例

```bash
skillshare target add aider ~/.aider/skills
skillshare sync
```

---

## 需求

### 路徑必須存在

如有需要，先建立目錄：

```bash
mkdir -p ~/.myapp/skills
skillshare target add myapp ~/.myapp/skills
```

### 路徑應以 `/skills` 結尾

這是建議做法，但非強制要求：

```bash
# 建議
skillshare target add myapp ~/.myapp/skills

# 也可以
skillshare target add myapp ~/.myapp/prompts
```

---

## 驗證

新增後：

```bash
# 檢查 Target
skillshare target myapp

# 同步到新 Target
skillshare sync

# 驗證
skillshare status
```

---

## 常見情境

### 新增 AI CLI 工具

```bash
# 1. 找出該工具儲存 Skill 的位置
# （查閱工具文件）

# 2. 如有需要，建立目錄
mkdir -p ~/.newtool/skills

# 3. 新增為 Target
skillshare target add newtool ~/.newtool/skills

# 4. 同步
skillshare sync
```

### 新增專案專屬的 Target

```bash
# 將 Skill 同步到特定專案
skillshare target add myproject ~/projects/myapp/.ai/skills
skillshare sync
```

### 新增多個工具

```bash
skillshare target add tool1 ~/.tool1/skills
skillshare target add tool2 ~/.tool2/skills
skillshare target add tool3 ~/.tool3/skills
skillshare sync
```

---

## 變更 Sync 模式

新增後，你可以變更 Sync 模式：

```bash
# 預設為 merge 模式
skillshare target myapp --mode symlink
skillshare sync
```

詳情請參閱 [Sync 模式](/docs/understand/sync-modes)。

---

## 移除 Target

如果你不再需要某個 Target：

```bash
skillshare target remove myapp
```

這會：
1. 建立備份
2. 將 symlink 替換為真實檔案（在 merge 模式下，只會移除由 source 管理的 symlink；本地 Skill 會被保留）
3. 從設定中移除

---

## 疑難排解

### "path does not exist"

先建立目錄：

```bash
mkdir -p ~/.myapp/skills
skillshare target add myapp ~/.myapp/skills
```

### Target 沒有同步

檢查該 Target 是否已啟用：

```bash
skillshare target list
skillshare target myapp
```

### 路徑錯誤

移除後重新新增：

```bash
skillshare target remove myapp
skillshare target add myapp /correct/path/skills
```

---

## 相關文件

- [支援的 Targets](./supported-targets.md) — 內建的 Targets
- [設定](./configuration.md) — 直接編輯設定檔
- [Sync 模式](/docs/understand/sync-modes) — Merge、copy、symlink
