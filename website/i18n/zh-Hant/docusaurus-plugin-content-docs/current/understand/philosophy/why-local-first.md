---
sidebar_position: 1
---

# Why Local-First

> skillshare 是一個沒有執行期相依性的單一 binary。以下是原因。

## 這個決定

skillshare 以單一 Go binary 的形式發行。沒有 Node.js、沒有 Python、沒有套件管理員、沒有 daemon。安裝、執行、完成。

這並不是最省力的路徑 — 而是由三項原則驅動的刻意選擇。

## 原則一：零相依性鏈

每一項相依性都是一個攻擊面，也是一項維護負擔。

如果 skillshare 需要 Node.js，你就得管理 Node 版本、處理 `node_modules`、應付平台特定的 native modules，還得信任整個 npm 供應鏈。對於一個管理 AI skills 的工具而言 — 而這些 skills 本身就是不受信任的內容 — 再加上一條不受信任的相依性鏈是無法接受的。

Go 編譯成靜態 binary。相依性鏈在編譯時就結束了。你下載的就是你執行的。

## 原則二：到處都以同樣的方式運作

skillshare 可在以下環境執行：
- macOS（Intel 與 Apple Silicon）
- Linux（amd64 與 arm64）
- Windows（amd64）
- Docker 容器（不需特殊設定）
- CI/CD pipelines（不需要語言執行環境）
- Dev containers 與 Codespaces

單一 binary 意味著在所有平台上都有相同的行為。不會有「在我機器上可以跑」的除錯困擾，也不會有 CI 環境漂移的問題。

## 原則三：預設離線可用

skillshare 的核心操作 — `sync`、`list`、`status`、`backup`、`restore` — 不需要網路連線就能運作。只有明確需要遠端的操作（`install`、`search`、`check`、`update`、`push`、`pull`）才需要連線。

這一點對以下情境很重要：
- **與外界隔離的環境**：國防、醫療、金融機構通常會限制網路存取
- **不穩定的連線**：火車、飛機、研討會 WiFi
- **速度**：本機操作在毫秒內完成，而非以秒計

## 為什麼不做成套件管理員的 Plugin？

我們曾考慮發行為 npm package、Homebrew formula（現在我們支援它作為額外的發行管道）或 pip package。每一種都有相同的問題：它們都會加上一項執行期相依性，而 skillshare 的使用者可能沒有、也不想要它。

一位在 Windows 上使用 Cursor 的開發者不應該被要求安裝 Homebrew。一個跑在 Alpine Linux 上的 CI pipeline 不應該需要 Node.js。工具應該去適應使用者的環境，而不是反過來。

## 取捨

單一 binary 的做法有其代價：

- **建置複雜度**：需要為 6 個以上的目標進行交叉編譯，並停用 CGO
- **更新機制**：沒有 `npm update` — skillshare 有自己的 `upgrade` 指令
- **UI 交付**：網頁儀表板無法與 binary 一起打包（太大了），所以會在執行期下載並快取

我們願意接受這些取捨，因為它們讓使用者體驗保持簡單：下載、執行、完成。

## 相關文件

- [Security-First Design](/docs/understand/philosophy/security-first)
- [Comparison with other tools](/docs/understand/philosophy/comparison)
