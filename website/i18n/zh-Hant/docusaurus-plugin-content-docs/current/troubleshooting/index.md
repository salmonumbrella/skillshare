---
sidebar_position: 1
---

# 疑難排解

遇到問題了嗎？從這裡開始。

## 快速診斷

執行 doctor 指令：

```bash
skillshare doctor
```

這會檢查：
- Source 目錄
- 設定檔
- Target 可存取性
- Symlink 健康狀態
- Git 狀態

---

## 遇到什麼狀況？

| 問題 | 前往 |
|---------|-------|
| 我看到錯誤訊息 | [常見錯誤](./common-errors.md) |
| 在 Windows 上有東西無法運作 | [Windows](./windows.md) |
| 我需要一步步的除錯流程 | [疑難排解流程](./troubleshooting-workflow.md) |
| 我有一般性的問題 | [FAQ](./faq.md) |

---

## 快速修復

### Skill 沒有出現

```bash
skillshare sync
```

### Symlink 損壞

```bash
skillshare sync --force
```

### 設定問題

```bash
skillshare doctor
```

### 從頭開始

```bash
rm ~/.config/skillshare/config.yaml
skillshare init
```

---

## 取得協助

如果你無法解決問題：

1. **蒐集資訊：**
   ```bash
   skillshare doctor
   skillshare status
   ```

2. **搜尋既有 issue：** [GitHub Issues](https://github.com/runkids/skillshare/issues)

3. **開一個新 issue**，並附上：
   - Doctor 輸出結果
   - 錯誤訊息
   - 你當時想做什麼
   - 作業系統

---

## 相關文件

- [疑難排解流程](./troubleshooting-workflow.md) — 一步步除錯
- [指令：doctor](/docs/reference/commands/doctor) — Doctor 指令細節
