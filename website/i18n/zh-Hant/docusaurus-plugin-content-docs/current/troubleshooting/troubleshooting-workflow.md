---
sidebar_position: 2
---

# 疑難排解流程

系統性的問題診斷與修復方法。

## 總覽

```mermaid
flowchart LR
    DIAGNOSE["診斷"] --> IDENTIFY["找出問題"] --> FIX["修復"] --> VERIFY["驗證"]
```

---

## 步驟 1：診斷

執行 doctor 指令：

```bash
skillshare doctor
```

**它會檢查：**
- Source 目錄存在且有效
- 設定檔格式正確
- 所有 Target 都可存取
- Symlink 沒有損壞
- Git 儲存庫狀態（若已初始化）
- Skill 格式的有效性

---

## 步驟 2：找出問題

### 常見症狀與原因

| 症狀 | 可能原因 | 快速修復 |
|---------|--------------|-----------|
| Skill 沒有出現在 AI CLI 中 | 尚未同步 | `skillshare sync` |
| Symlink 損壞 | Source 被刪除 | 還原或重新安裝 |
| 設定錯誤 | YAML 無效 | `skillshare doctor` 會顯示詳情 |
| 無法 push/pull | Git 問題 | 手動檢查 git 狀態 |
| 權限被拒 | 擁有權錯誤 | 檢查檔案權限 |

---

## 步驟 3：修復

### Sync 問題

```bash
# 重新同步所有 Target
skillshare sync

# 強制同步（重新建立 symlink）
skillshare sync --force
```

### Symlink 損壞

```bash
# 檢查狀態
skillshare status

# 同步以重新建立
skillshare sync
```

### 設定問題

```bash
# 檢視目前設定
cat ~/.config/skillshare/config.yaml

# 重設設定
rm ~/.config/skillshare/config.yaml
skillshare init
```

### Git 問題

```bash
cd ~/.config/skillshare/skills

# 檢查狀態
git status

# Pull 失敗（有本地變更）
git stash
git pull
git stash pop

# Push 失敗（remote 領先）
git pull
git push
```

### Target 問題

```bash
# 移除並重新新增
skillshare target remove claude
skillshare target add claude ~/.claude/skills
skillshare sync
```

---

## 步驟 4：驗證

```bash
# 檢查狀態
skillshare status

# 再次執行 doctor
skillshare doctor

# 在 AI CLI 中測試
# （呼叫某個 Skill）
```

---

## 復原選項

### 輕度復原

```bash
# 只要重新同步
skillshare sync
```

### 中度復原

```bash
# 從備份還原
skillshare restore claude
skillshare sync
```

### 重度復原（從頭開始）

```bash
# 備份目前狀態
skillshare backup

# 移除設定（保留 Skill）
rm ~/.config/skillshare/config.yaml

# 重新初始化
skillshare init

# 同步
skillshare sync
```

---

## 取得協助

如果你無法解決問題：

1. **蒐集資訊：**
   ```bash
   skillshare doctor > doctor-output.txt
   skillshare status >> doctor-output.txt
   ```

2. **查閱 FAQ：** [常見錯誤](/docs/troubleshooting/common-errors)

3. **回報問題：** [GitHub Issues](https://github.com/runkids/skillshare/issues)
   - 附上 doctor 輸出結果
   - 附上錯誤訊息
   - 描述你當時想做什麼

---

## 相關文件

- [常見錯誤](/docs/troubleshooting/common-errors) — 錯誤訊息與解決方法
- [Windows 問題](/docs/troubleshooting/windows) — Windows 特有問題
- [FAQ](/docs/troubleshooting/faq) — 常見問題
- [指令：doctor](/docs/reference/commands/doctor) — Doctor 指令
