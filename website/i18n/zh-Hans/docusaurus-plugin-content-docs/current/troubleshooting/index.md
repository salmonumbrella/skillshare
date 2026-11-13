---
sidebar_position: 1
---

# Troubleshooting

遇到问题了？从这里开始。

## Quick Diagnosis

执行 doctor 命令：

```bash
skillshare doctor
```

它会检查：
- Source 目录
- 配置文件
- Target 是否可访问
- symlink 健康状态
- Git 状态

---

## What's happening?

| 问题 | 前往 |
|---------|-------|
| 我看到一条错误信息 | [Common Errors](./common-errors.md) |
| Windows 上有些东西不能用 | [Windows](./windows.md) |
| 我需要一步步的排查流程 | [Troubleshooting Workflow](./troubleshooting-workflow.md) |
| 我有一个一般性的问题 | [FAQ](./faq.md) |

---

## Quick Fixes

### Skill 没有出现

```bash
skillshare sync
```

### symlink 损坏

```bash
skillshare sync --force
```

### 配置问题

```bash
skillshare doctor
```

### 从头开始

```bash
rm ~/.config/skillshare/config.yaml
skillshare init
```

---

## Getting Help

如果问题仍无法解决：

1. **收集信息：**
   ```bash
   skillshare doctor
   skillshare status
   ```

2. **搜索现有 issue：** [GitHub Issues](https://github.com/runkids/skillshare/issues)

3. **提交新 issue**，并附上：
   - doctor 的输出
   - 错误信息
   - 你原本想做什么
   - 操作系统

---

## Related

- [Troubleshooting Workflow](./troubleshooting-workflow.md) — 一步步排查问题
- [Commands: doctor](/docs/reference/commands/doctor) — doctor 命令详情
