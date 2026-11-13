---
sidebar_position: 3
---

# Recipe：Pre-commit Hook

> 使用 [pre-commit](https://pre-commit.com/) 框架，在每次 commit 時自動執行 `skillshare audit`。

## 何時使用

pre-commit hook 在以下情況最有價值：

- **多位貢獻者編輯 skills** — 團隊成員可能不小心引入危險指令（`curl | bash`、`sudo rm -rf`）。這個 hook 會在它們進入版本控管前將其攔截。
- **Skills 來自外部來源** — 從 GitHub、社群 repo 或 AI 產生的內容複製 skills，讓人工審查變得困難。自動化掃描提供一道安全網。
- **你想要即時回饋** — CI 也能捕捉問題，但只有在 push 之後。這個 hook 能在幾秒內給開發者即時、本機的回饋。

以下情況可以跳過它：

- 你是唯一的作者，並信任你所有的 skills
- Skills 很少變更（此 hook 只在 `.skillshare/` 或 `skills/` 檔案被修改時才會執行）

## 設定

加入到你 project 的 `.pre-commit-config.yaml`：

```yaml
repos:
  - repo: https://github.com/runkids/skillshare
    rev: v0.16.8  # 使用最新的 release tag
    hooks:
      - id: skillshare-audit
```

接著安裝 hook：

```bash
pre-commit install
```

## 運作方式

每當你 commit 對符合 `.skillshare/` 或 `skills/` 目錄的檔案變更時，這個 hook 會執行 `skillshare audit -p`。若任何發現超過設定的門檻，該次 commit 就會被封鎖。

## 設定選項

這個 hook 會遵循你 project 的 `.skillshare/config.yaml` 設定：

```yaml
audit:
  block_threshold: high  # 在 HIGH 以上的發現時封鎖
```

## 跳過 Hook

一次性跳過：

```bash
SKIP=skillshare-audit git commit -m "your message"
```

## 需求

- `skillshare` CLI 必須已安裝並存在於 `PATH` 中
- Project 必須已使用 `skillshare init -p` 初始化

## 與 CI 結合

pre-commit hook 能在本機捕捉問題，而 [CI/CD 驗證](ci-cd-skill-validation.md) 則為整個團隊提供一道安全網。同時使用兩者，可獲得縱深防禦。
