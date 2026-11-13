---
sidebar_position: 5
---

# Sync Modes Explained

> 深入探討三種 sync 模式 — merge、copy 和 symlink — 各自適用的時機，以及取捨。

## 三種模式

skillshare 提供三種 sync 模式，控制 skills 如何從你的 source 目錄交付到 AI 工具的 target 目錄。

### Merge Mode（預設）

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

**運作方式**：為每個 skill 建立一個 symlink。target 目錄中的每個 skill 目錄都指回 source。

**關鍵特性**：**非破壞性**。target 目錄中的本機 skills（如上方的 `my-local-skill`）會被保留。skillshare 只管理它自己建立的 symlinks。

### Copy Mode

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

**運作方式**：把每個 skill 實際複製進 target。一個 `.skillshare-manifest.json` 檔案會追蹤哪些 skills 受管理，以及它們的 SHA-256 checksums。之後的 sync 只會重新複製有變更的 skills。

**關鍵特性**：**最大相容性**。到處都能運作 — 不需要 symlink 支援。本機 skills 也會像 merge mode 一樣被保留。

### Symlink Mode

```
Source: ~/.config/skillshare/skills/
├── code-review/SKILL.md
├── testing/SKILL.md
└── debugging/SKILL.md

Target: ~/.claude/skills → ~/.config/skillshare/skills/  (single symlink)
```

**運作方式**：把整個 target 目錄替換成一個指向 source 的單一 symlink。

**關鍵特性**：**完全控制**。target 就等於 source。target 中不可能存在任何本機 skills。

## 各模式的使用時機

| 因素 | Merge | Copy | Symlink |
|--------|-------|------|---------|
| 保留本機 skills | 是 | 是 | 否 |
| 跨平台支援 | 可能有問題 | 到處都能運作 | 可能有問題 |
| Source 變更反映速度 | 立即 | `sync` 之後 | 立即 |
| 處理巢狀路徑 | 攤平（`a/b/c` → `a__b__c`） | 攤平 | 保留原生結構 |
| Orphan 清理 | 自動 | 自動 | 不需要 |
| 磁碟用量 | 最小（symlinks） | 完整複製 | 最小（單一 symlink） |
| 建議用於 | 多數使用者 | WSL、Docker、CI | 單一 source 設定 |

### 選擇 Merge 的時機

- 你在 AI 工具中有本機 skills，不想讓 skillshare 管理它們
- 你使用多個 AI 工具，各自有不同的本機客製化
- 你正在逐步導入 skillshare（部分 skills 受管理，部分不受管理）

### 選擇 Copy 的時機

- 你的平台對 symlink 支援不穩定（WSL、部分 Docker 設定）
- AI 工具無法正確跟隨 symlink
- 你身處 CI/CD pipeline 或容器化環境
- 你希望 target 能獨立於 source 目錄運作

### 選擇 Symlink 的時機

- skillshare 是某個 target 唯一的 skills 來源
- 你希望對 target 中有什麼內容毫無疑義
- 你正在設定一個全新的環境

## 巢狀路徑處理

在 merge 和 copy 模式中，巢狀的 source 路徑會用雙底線攤平：

```
Source: skills/frontend/react-patterns/SKILL.md
Target: ~/.claude/skills/frontend__react-patterns → skills/frontend/react-patterns
```

這能避免在預期扁平 skill 結構的 target 中建立目錄。在 symlink mode 中，目錄結構則會原封不動地保留。

## Orphan 清理

Merge 和 copy 模式會在 `skillshare sync` 期間自動移除孤兒項目。如果你從 source 中移除某個 skill，target 中對應的 symlink（或已複製的目錄）會在下一次 sync 時被清理掉。

```bash
skillshare uninstall old-skill
skillshare sync
# → Pruned orphan: old-skill
```

## 每個 Target 各自覆寫模式

你可以為不同的 target 設定不同的模式。Global 設定使用 map 格式：

```yaml
targets:
  claude:
    path: ~/.claude/skills
    mode: merge
  cursor:
    path: ~/.cursor/skills
    mode: copy
```

Project 設定使用 list 格式：

```yaml
targets:
  - name: claude
    mode: merge
  - name: cursor
    mode: copy
```

或透過 CLI 變更模式：

```bash
skillshare target claude --mode copy
```

## 相關文件

- [Sync modes concept page](/docs/understand/sync-modes)
- [`sync` command reference](/docs/reference/commands/sync)
- [Source and targets](/docs/understand/source-and-targets)
