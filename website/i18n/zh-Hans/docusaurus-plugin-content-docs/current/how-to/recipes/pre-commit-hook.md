---
sidebar_position: 3
---

# Recipe: Pre-commit Hook

> 使用 [pre-commit](https://pre-commit.com/) 框架，在每次提交时自动运行 `skillshare audit`。

## When to Use

在以下情况中，pre-commit hook 最有价值：

- **多位贡献者编辑 Skill** — 团队成员可能无意中引入危险命令（`curl | bash`、`sudo rm -rf`）。该 hook 会在它们进入版本控制之前将其捕获。
- **Skill 来自外部来源** — 从 GitHub、社区仓库或 AI 生成内容复制 Skill 使得人工审查很困难。自动扫描提供了一道安全网。
- **你想要即时反馈** — CI 也能捕获问题，但只在推送之后。该 hook 能在几秒内为开发者提供即时的本地反馈。

在以下情况可以跳过它：

- 你是唯一作者，并信任所有 Skill
- Skill 很少变动（该 hook 只在 `.skillshare/` 或 `skills/` 文件被修改时运行）

## Setup

添加到你 Project 的 `.pre-commit-config.yaml`：

```yaml
repos:
  - repo: https://github.com/runkids/skillshare
    rev: v0.16.8  # 使用最新的 release tag
    hooks:
      - id: skillshare-audit
```

然后安装该 hook：

```bash
pre-commit install
```

## How It Works

只要你提交的变更涉及 `.skillshare/` 或 `skills/` 目录下的文件，该 hook 就会运行 `skillshare audit -p`。如果任何 findings 超过配置的阈值，提交将被阻止。

## Configuration

该 hook 会遵循你 Project 的 `.skillshare/config.yaml` 设置：

```yaml
audit:
  block_threshold: high  # 在 HIGH 及以上时阻止
```

## Skipping the Hook

一次性跳过：

```bash
SKIP=skillshare-audit git commit -m "your message"
```

## Requirements

- `skillshare` CLI 必须已安装并存在于 `PATH` 中
- Project 必须已通过 `skillshare init -p` 初始化

## Combining with CI

pre-commit hook 在本地捕获问题，而 [CI/CD validation](ci-cd-skill-validation.md) 为整个团队提供安全网。两者结合使用可实现纵深防御。
