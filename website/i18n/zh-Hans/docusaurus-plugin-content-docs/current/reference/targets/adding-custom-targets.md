---
sidebar_position: 3
---

# 添加自定义 Target

将任何拥有 Skill 目录的工具添加到 skillshare。

## 概览

如果你的 AI CLI 不在[支持列表](./supported-targets.md)中，你可以手动添加它。

---

## 添加 Target

```bash
skillshare target add <name> <path>
```

### 示例

```bash
skillshare target add aider ~/.aider/skills
skillshare sync
```

---

## 要求

### 路径必须存在

如有需要先创建目录：

```bash
mkdir -p ~/.myapp/skills
skillshare target add myapp ~/.myapp/skills
```

### 路径应以 `/skills` 结尾

这是推荐做法，但非强制要求：

```bash
# 推荐
skillshare target add myapp ~/.myapp/skills

# 也可以
skillshare target add myapp ~/.myapp/prompts
```

---

## 验证

添加之后：

```bash
# 检查 Target
skillshare target myapp

# Sync 到新 Target
skillshare sync

# 验证
skillshare status
```

---

## 常见场景

### 添加新的 AI CLI 工具

```bash
# 1. 找到该工具存储 Skill 的位置
# （查阅该工具的文档）

# 2. 如有需要创建目录
mkdir -p ~/.newtool/skills

# 3. 添加为 Target
skillshare target add newtool ~/.newtool/skills

# 4. Sync
skillshare sync
```

### 添加项目专属 Target

```bash
# 将 Skill sync 到特定项目
skillshare target add myproject ~/projects/myapp/.ai/skills
skillshare sync
```

### 添加多个工具

```bash
skillshare target add tool1 ~/.tool1/skills
skillshare target add tool2 ~/.tool2/skills
skillshare target add tool3 ~/.tool3/skills
skillshare sync
```

---

## 更改 Sync 模式

添加之后，你可以更改 Sync 模式：

```bash
# 默认是 merge 模式
skillshare target myapp --mode symlink
skillshare sync
```

详见 [Sync Modes](/docs/understand/sync-modes)。

---

## 移除 Target

如果不再需要某个 Target：

```bash
skillshare target remove myapp
```

此操作会：
1. 创建备份
2. 将符号链接替换为真实文件（在 merge 模式下，只移除由 Source 管理的符号链接；本地 Skill 会被保留）
3. 从配置中移除

---

## 疑难排解

### "path does not exist"

先创建目录：

```bash
mkdir -p ~/.myapp/skills
skillshare target add myapp ~/.myapp/skills
```

### Target 未 sync

检查该 Target 是否已启用：

```bash
skillshare target list
skillshare target myapp
```

### 路径错误

移除后重新添加：

```bash
skillshare target remove myapp
skillshare target add myapp /correct/path/skills
```

---

## 相关内容

- [Supported Targets](./supported-targets.md) — 内置 Target
- [Configuration](./configuration.md) — 直接编辑配置
- [Sync Modes](/docs/understand/sync-modes) — Merge、copy、symlink
