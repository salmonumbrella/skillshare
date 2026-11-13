---
sidebar_position: 4
---

# Recipe：Project Mode 工作流程

> 管理與你程式碼庫一同移動的 project 範圍 skills。

## 情境

你希望將特定的 skills commit 到你的 project repository，讓：
- 每位貢獻者都獲得相同的 AI 指示
- Skills 與程式碼一同進行版本控管
- 除了 clone repo 之外不需要任何手動設定

## 解決方案

### 步驟 1：初始化 project mode

```bash
cd your-project
skillshare init -p
```

這會在你的 project 根目錄建立 `.skillshare/config.yaml`。

### 步驟 2：安裝 project 範圍的 skills

```bash
skillshare install anthropics/courses/prompt-eng -p
skillshare install your-org/team-skills --skill code-review -p
```

Skills 會放置於 `.skillshare/skills/`。

### 步驟 3：同步到 project targets

```bash
skillshare sync -p
```

這會從 `.skillshare/skills/` 建立 symlinks 到 project 層級的 target 目錄（例如 `.claude/skills/`、`.cursor/skills/`）。

### 步驟 4：Commit 到版本控管

```bash
git add .skillshare/
git commit -m "Add project skills"
```

### 步驟 5：團隊成員設定

當團隊成員 clone 這個 repo 時：

```bash
git clone your-org/your-project
cd your-project
skillshare sync -p
```

一個指令即可將所有 project skills 同步到他們本機的 AI 工具。

## 驗證

- Project 根目錄中存在 `.skillshare/config.yaml`
- `.skillshare/skills/` 包含已安裝的 skills
- `skillshare list -p` 顯示 project skills
- `sync -p` 之後，target 目錄包含 symlinks

## 變化

- **Dev container 自動同步**：在 `.devcontainer/devcontainer.json` 的 `postCreateCommand` 加入 `skillshare sync -p`
- **混合模式**：對個人偏好使用 global skills，對團隊標準使用 project skills
- **CI 驗證**：在 CI pipeline 中加入 `skillshare audit -p`，以驗證 project skills

## 相關

- [Project 設定指南](/docs/how-to/sharing/project-setup)
- [理解 project skills](/docs/understand/project-skills)
- [Dev container 指南](/docs/learn/with-devcontainer)
