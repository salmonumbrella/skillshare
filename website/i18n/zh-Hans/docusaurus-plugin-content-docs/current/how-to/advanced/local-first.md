---
sidebar_position: 10
---

# 中心化 vs Local-First

Skill 管理工具通常遵循两种架构方式之一：**中心化平台**或 **Local-First**。两者并无绝对优劣 — 各有取舍。本页将带你了解这两种方式，帮助你判断哪种更适合自己的工作流程。

:::tip 这不是功能对比
关于功能层面的差异（安装流程、配置格式等），请参见 [Comparing Skill Management Approaches](/docs/understand/philosophy/comparison)。本页聚焦于**架构层面的取舍** — 数据存放在哪里、发现机制如何运作，以及你能掌控什么。
:::

## 两种方式

### 中心化平台

中心化平台托管一个统一的注册中心，Skill 在此发布、搜索并排名。安装活动会被汇总为下载量、热门排行等社区指标。

**优势**：
- 内置的发现机制 — 可在同一个地方浏览、搜索、比较 Skill
- 社区信号 — 下载量和热门排行有助于发现受欢迎的 Skill
- 低门槛 — 发现无需任何设置，搜索即可安装

**需要考量的地方**：
- 安装活动会被平台追踪
- 排名与统计规则由平台运营方管理

### Local-First（skillshare）

skillshare 将所有状态保存在你自己的机器上。Skill 通过 `git clone` 安装，并通过本地配置文件管理。不会向任何远程服务器发送数据。

**优势**：
- 零遥测 — 不追踪安装，不发送任何数据
- 完全掌控 — 你的 Skill 存放在你自己的文件系统中
- 初次安装后可离线使用
- 单一二进制文件，无运行时依赖

**需要考量的地方**：
- 没有内置的社区指标（下载量、热门排行）
- 发现机制需要设置或连接到 Hub

## 发现机制

Local-First 并不意味着没有发现机制。skillshare 提供三种发现渠道：

| 渠道 | 工作方式 |
|---------|-------------|
| **GitHub 搜索** | `skillshare search <query>` — 直接搜索公开的 GitHub 仓库 |
| **公共 Hub** | `skillshare search --hub` — 查询内置的[社区 Hub](https://github.com/runkids/skillshare-hub) |
| **自定义 Hub** | `skillshare search --hub <url>` — 查询你或你组织维护的任意 Hub |

### 什么是 Hub？

Hub 是一个静态 JSON 文件（`skillshare-hub.json`），列出各个 Skill 的名称、描述、来源与标签。它可以存放在任何地方 — Git 仓库、HTTP 服务器，或本地文件系统：

```bash
# 从已安装的 Skill 构建索引
skillshare hub index

# 搜索组织内部的 Hub
skillshare search --hub https://internal.corp/skills/hub.json

# 搜索本地索引文件
skillshare search --hub ./skillshare-hub.json
```

Hub 彼此独立 — 任何人都可以创建一个，用户也可以同时连接多个 Hub。这使其非常适合需要在维护私有 Skill 目录的同时也使用公共目录的组织。

详细操作步骤请参见 [Hub Index Guide](/docs/how-to/sharing/hub-index)。

### 自托管指标

skillshare 本身不会追踪安装情况，但如果你在自己的服务器上托管 Hub，可以自行添加任何合适的分析层：

1. 在你的服务器上托管 `skillshare-hub.json`
2. 添加请求日志或轻量级分析端点
3. 追踪搜索命中次数、安装引荐来源，或任何你关心的指标

这让 Skill 作者或组织可以按自己的标准衡量采用情况。

## 如何选择

**在以下情况下，中心化平台可能更适合：**
- 你希望开箱即用地获得内置的社区指标和热门排行
- 你更喜欢一个统一的浏览入口来发现 Skill
- 你只使用一种 AI CLI，不需要跨工具同步

**在以下情况下，Local-First 可能更适合：**
- 你使用多种 AI CLI，希望统一管理
- 你希望安装活动只留在你自己的机器上
- 你需要离线运行，或工作在受限的网络环境中
- 你是需要掌控哪些 Skill 可用、可被发现的组织

---

## 另请参阅

- [Comparing Skill Management Approaches](/docs/understand/philosophy/comparison) — 功能层面的对比
- [Hub Index Guide](/docs/how-to/sharing/hub-index) — 构建与使用 Skill Hub
- [hub command](/docs/reference/commands/hub) — Hub 命令参考
- [Security Guide](./security.md) — Skill 安全扫描
