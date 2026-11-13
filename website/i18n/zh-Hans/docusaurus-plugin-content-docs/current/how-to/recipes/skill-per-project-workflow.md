---
sidebar_position: 4
---

# Recipe: Project Mode Workflow

> 管理随代码库一起流转的 Project 范围 Skill。

## Scenario

你希望将特定 Skill 提交到你的 Project 仓库，以便：
- 每位贡献者都获得相同的 AI 指令
- Skill 与代码一起被版本化
- 除了克隆仓库之外无需其他手动设置

## Solution

### Step 1: 初始化 Project mode

```bash
cd your-project
skillshare init -p
```

这会在你的 Project 根目录创建 `.skillshare/config.yaml`。

### Step 2: 安装 Project 范围的 Skill

```bash
skillshare install anthropics/courses/prompt-eng -p
skillshare install your-org/team-skills --skill code-review -p
```

Skill 会被放置在 `.skillshare/skills/` 中。

### Step 3: Sync 到 Project Targets

```bash
skillshare sync -p
```

这会从 `.skillshare/skills/` 创建 symlink 到 Project 级别的 Target 目录（例如 `.claude/skills/`、`.cursor/skills/`）。

### Step 4: 提交到版本控制

```bash
git add .skillshare/
git commit -m "Add project skills"
```

### Step 5: 队友设置

当队友克隆该仓库时：

```bash
git clone your-org/your-project
cd your-project
skillshare sync -p
```

一条命令即可将所有 Project Skill Sync 到他们本地的 AI 工具。

## Verification

- `.skillshare/config.yaml` 存在于 Project 根目录
- `.skillshare/skills/` 包含已安装的 Skill
- `skillshare list -p` 显示 Project Skill
- 执行 `sync -p` 后，Target 目录中包含 symlink

## Variations

- **Dev container 自动 Sync**：在 `.devcontainer/devcontainer.json` 的 `postCreateCommand` 中添加 `skillshare sync -p`
- **混合模式**：使用 Global Skill 满足个人偏好 + Project Skill 用于团队标准
- **CI 验证**：在 CI 流水线中添加 `skillshare audit -p` 以验证 Project Skill

## Related

- [Project setup guide](/docs/how-to/sharing/project-setup)
- [Understanding project skills](/docs/understand/project-skills)
- [Dev container guide](/docs/learn/with-devcontainer)
