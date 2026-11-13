---
sidebar_position: 5
---

# Sync 模式详解

> 深入剖析三种 sync 模式——merge、copy 和 symlink——分别应在何时使用，以及各自的权衡取舍。

## 三种模式

skillshare 提供三种 sync 模式，用于控制 Skills 如何从你的 source 目录交付到 AI 工具的 Target 目录。

### Merge 模式（默认）

```
Source: ~/.config/skillshare/skills/
├── code-review/SKILL.md
├── testing/SKILL.md
└── debugging/SKILL.md

Target: ~/.claude/skills/
├── code-review → ~/.config/skillshare/skills/code-review  (symlink)
├── testing → ~/.config/skillshare/skills/testing           (symlink)
├── debugging → ~/.config/skillshare/skills/debugging       (symlink)
└── my-local-skill/SKILL.md                                 (untouched)
```

**工作方式**：为每个 Skill 创建一个符号链接。Target 中的每个 Skill 目录都指回 source。

**关键特性**：**非破坏性**。Target 目录中的本地 Skills（如上面的 `my-local-skill`）会被保留。skillshare 只管理它自己创建的符号链接。

### Copy 模式

```
Source: ~/.config/skillshare/skills/
├── code-review/SKILL.md
├── testing/SKILL.md
└── debugging/SKILL.md

Target: ~/.cursor/skills/
├── code-review/SKILL.md                (physical copy)
├── testing/SKILL.md                    (physical copy)
├── debugging/SKILL.md                  (physical copy)
├── .skillshare-manifest.json           (tracks managed files)
└── my-local-skill/SKILL.md             (untouched)
```

**工作方式**：将每个 Skill 物理复制到 Target 中。`.skillshare-manifest.json` 文件会记录哪些 Skills 受管理及其 SHA-256 校验和。在后续的 sync 中，只有发生变化的 Skills 才会被重新复制。

**关键特性**：**最大兼容性**。在任何环境下都能工作——不需要符号链接支持。本地 Skills 也会像 merge 模式一样被保留。

### Symlink 模式

```
Source: ~/.config/skillshare/skills/
├── code-review/SKILL.md
├── testing/SKILL.md
└── debugging/SKILL.md

Target: ~/.claude/skills → ~/.config/skillshare/skills/  (single symlink)
```

**工作方式**：用单个符号链接替换整个 Target 目录，该链接指向 source。

**关键特性**：**完全掌控**。Target 与 source 完全一致，Target 中不可能存在任何本地 Skill。

## 该用哪一种

| 因素 | Merge | Copy | Symlink |
|--------|-------|------|---------|
| 保留本地 Skills | 是 | 是 | 否 |
| 跨平台支持 | 可能有问题 | 到处都能用 | 可能有问题 |
| Source 变更的反映速度 | 即时 | 执行 `sync` 之后 | 即时 |
| 处理嵌套路径 | 扁平化（`a/b/c` → `a__b__c`） | 扁平化 | 保留原生结构 |
| 孤立项清理 | 自动 | 自动 | 无需清理 |
| 磁盘占用 | 极小（符号链接） | 完整复制 | 极小（单个符号链接） |
| 推荐场景 | 大多数用户 | WSL、Docker、CI | 单一来源的环境 |

### 何时选择 Merge

- 你的 AI 工具中有一些本地 Skills，不希望交由 skillshare 管理
- 你使用多个带有各自本地定制的 AI 工具
- 你正在逐步采用 skillshare（部分 Skills 受管理，部分不受管理）

### 何时选择 Copy

- 你的平台对符号链接支持不稳定（WSL、部分 Docker 环境）
- 该 AI 工具无法正确跟随符号链接
- 你处于 CI/CD 流水线或容器化环境中
- 你希望 Target 能独立于 source 目录正常工作

### 何时选择 Symlink

- skillshare 是某个 Target 唯一的 Skill 来源
- 你希望 Target 里的内容完全没有歧义
- 你正在搭建一个全新的环境

## 嵌套路径处理

在 merge 和 copy 模式下，嵌套的 source 路径会用双下划线扁平化：

```
Source: skills/frontend/react-patterns/SKILL.md
Target: ~/.claude/skills/frontend__react-patterns → skills/frontend/react-patterns
```

这样可以避免在期望扁平 Skill 结构的 Target 中创建目录。而在 symlink 模式下，目录结构会原样保留。

## 孤立项清理

Merge 和 copy 模式会在 `skillshare sync` 期间自动移除孤立的条目。如果你从 source 中卸载了某个 Skill，Target 中对应的符号链接（或复制的目录）会在下一次 sync 时被清理掉。

```bash
skillshare uninstall old-skill
skillshare sync
# → Pruned orphan: old-skill
```

## 按 Target 覆盖模式

你可以为不同的 Target 设置不同的模式。全局配置使用映射（map）格式：

```yaml
targets:
  claude:
    path: ~/.claude/skills
    mode: merge
  cursor:
    path: ~/.cursor/skills
    mode: copy
```

项目配置使用列表（list）格式：

```yaml
targets:
  - name: claude
    mode: merge
  - name: cursor
    mode: copy
```

或者通过 CLI 更改模式：

```bash
skillshare target claude --mode copy
```

## 相关内容

- [Sync 模式概念页](/docs/understand/sync-modes)
- [`sync` 命令参考](/docs/reference/commands/sync)
- [Source 与 Target](/docs/understand/source-and-targets)
