---
sidebar_position: 2
---

# Troubleshooting Workflow

诊断与修复问题的系统化方法。

## Overview

```mermaid
flowchart LR
    DIAGNOSE["诊断"] --> IDENTIFY["定位"] --> FIX["修复"] --> VERIFY["验证"]
```

---

## Step 1: Diagnose

执行 doctor 命令：

```bash
skillshare doctor
```

**它会检查：**
- Source 目录存在且有效
- 配置文件格式正确
- 所有 Target 都可访问
- symlink 没有损坏
- Git 仓库状态（如果已初始化）
- Skill 格式是否有效

---

## Step 2: Identify the Issue

### 常见症状与原因

| 症状 | 可能原因 | 快速修复 |
|---------|--------------|-----------|
| Skill 没有出现在 AI CLI 中 | 未同步 | `skillshare sync` |
| symlink 损坏 | Source 被删除 | 还原或重新安装 |
| 配置出错 | YAML 无效 | `skillshare doctor` 会显示详情 |
| 无法 push/pull | Git 问题 | 手动检查 git 状态 |
| Permission denied | 权限归属不对 | 检查文件权限 |

---

## Step 3: Fix

### Sync 问题

```bash
# 重新同步所有 Target
skillshare sync

# 强制同步（重建 symlink）
skillshare sync --force
```

### symlink 损坏

```bash
# 检查状态
skillshare status

# 通过 sync 重建
skillshare sync
```

### 配置问题

```bash
# 查看当前配置
cat ~/.config/skillshare/config.yaml

# 重置配置
rm ~/.config/skillshare/config.yaml
skillshare init
```

### Git 问题

```bash
cd ~/.config/skillshare/skills

# 检查状态
git status

# pull 失败（有本地变更）
git stash
git pull
git stash pop

# push 失败（远程领先）
git pull
git push
```

### Target 问题

```bash
# 移除后重新添加
skillshare target remove claude
skillshare target add claude ~/.claude/skills
skillshare sync
```

---

## Step 4: Verify

```bash
# 检查状态
skillshare status

# 再次执行 doctor
skillshare doctor

# 在 AI CLI 中测试
# （调用某个 Skill）
```

---

## Recovery Options

### 轻度恢复

```bash
# 只需重新 sync
skillshare sync
```

### 中度恢复

```bash
# 从 backup 还原
skillshare restore claude
skillshare sync
```

### 重度恢复（从头开始）

```bash
# 备份当前状态
skillshare backup

# 移除配置（保留 Skill）
rm ~/.config/skillshare/config.yaml

# 重新初始化
skillshare init

# Sync
skillshare sync
```

---

## Getting Help

如果问题仍无法解决：

1. **收集信息：**
   ```bash
   skillshare doctor > doctor-output.txt
   skillshare status >> doctor-output.txt
   ```

2. **查阅 FAQ：** [Common Errors](/docs/troubleshooting/common-errors)

3. **报告问题：** [GitHub Issues](https://github.com/runkids/skillshare/issues)
   - 附上 doctor 输出
   - 附上错误信息
   - 描述你原本想做什么

---

## Related

- [Common Errors](/docs/troubleshooting/common-errors) — 错误信息与解决方式
- [Windows Issues](/docs/troubleshooting/windows) — Windows 专属问题
- [FAQ](/docs/troubleshooting/faq) — 常见问题
- [Commands: doctor](/docs/reference/commands/doctor) — doctor 命令
