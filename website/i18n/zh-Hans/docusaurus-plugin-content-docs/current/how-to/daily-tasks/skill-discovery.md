---
sidebar_position: 3
---

# Skill Discovery

从社区中寻找、评估并安装 Skill。

## 概览

```mermaid
flowchart LR
    SEARCH["搜索"] --> BROWSE["浏览"] --> EVALUATE["评估"] --> INSTALL["安装"] --> SYNC["同步"]
```

---

## 第一步：搜索

按关键字寻找 Skill，或浏览热门 Skill：

```bash
skillshare search              # 浏览热门 Skill
skillshare search pdf
skillshare search "code review"
skillshare search react
```

---

## 第二步：浏览仓库

探索某个仓库中的 Skill：

```bash
# 官方 Anthropic Skill
skillshare install anthropics/skills

# 社区 Skill
skillshare install ComposioHQ/awesome-claude-skills
```

这会进入 **discovery mode**——显示该仓库中所有可用的 Skill。

---

## 第三步：评估

在安装前，请考虑：

- **它能解决我的问题吗？** 阅读其说明
- **它维护得好吗？** 检查该仓库的活跃度
- **它会产生冲突吗？** 检查是否与现有 Skill 名称冲突

预览将会安装的内容：
```bash
skillshare install anthropics/skills/skills/pdf --dry-run
```

---

## 第四步：安装

### 单个 Skill

```bash
skillshare install anthropics/skills/skills/pdf
```

### 来自同一仓库的多个 Skill

```bash
# 交互式浏览
skillshare install anthropics/skills

# 选择特定的 Skill（非交互式）
skillshare install anthropics/skills -s pdf,commit

# 安装所有 Skill
skillshare install anthropics/skills --all
```

### 整个仓库（适用于团队）

```bash
skillshare install github.com/team/skills --track
```

---

## 第五步：同步

安装后别忘了同步：

```bash
skillshare sync
```

---

## 热门 Skill 来源

| 来源 | URL |
|--------|-----|
| Anthropic 官方 | `anthropics/skills` |
| Vercel Agent Skills | `vercel-labs/agent-skills` |
| 社区 | [skillsmp.com](https://skillsmp.com/) |

---

## Discovery 命令

| 命令 | 用途 |
|---------|-------|
| `search` | 浏览热门 Skill |
| `search <query>` | 搜索 Skill |
| `check` | 检查是否有可用的更新 |
| `install <repo>` | 浏览仓库（discovery mode） |
| `install <repo/path>` | 安装指定的 Skill |
| `list` | 显示已安装的 Skill |

---

## 安装选项

```bash
# 自定义名称
skillshare install anthropics/skills/skills/pdf --name my-pdf

# 强制覆盖
skillshare install anthropics/skills/skills/pdf --force

# 更新现有的
skillshare install anthropics/skills/skills/pdf --update

# 为团队共享而追踪
skillshare install github.com/team/skills --track
```

`--name` 只有在安装目标是单个 Skill 时才有效。  
若在仓库 discovery 返回多个 Skill 时使用 `--name`，会返回错误。

---

## 安装之后

### 验证

```bash
skillshare list
skillshare status
```

### 测试

在你的 AI CLI 中使用该 Skill，确认它的表现符合预期。

### 检查更新

```bash
skillshare check              # 查看有哪些可用更新
```

### 之后再更新

```bash
# 单个 Skill（带 Source 元数据）
skillshare install pdf --update

# 已追踪的仓库
skillshare update _team-skills
```

---

## 另请参阅

- [search](/docs/reference/commands/search) —— search 命令参考文档
- [install](/docs/reference/commands/install) —— install 命令参考文档
- [Hub Index](/docs/how-to/sharing/hub-index) —— 管理 Skill hub
- [Daily Workflow](./daily-workflow.md) —— 安装后的日常使用
