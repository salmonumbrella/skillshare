---
sidebar_position: 3
---

# Skill Design

如何撰寫穩定可靠的 skills — 選擇合適的複雜度層級、最大化決定性（determinism），並運用漸進式揭露（progressive disclosure）。

:::tip 什麼時候需要在意這個？
如果你的 skills 表現不一致、便宜的模型在你的 skills 上頻頻失敗，或是你正在為團隊打造 skills — 這篇指南能幫你寫出**可靠、安全且高效**的 skills。
:::

## Skill 光譜

不是所有 skills 都生而平等。了解你的 skill 落在複雜度光譜的哪個位置，能幫你做出正確的設計選擇：

| 層級 | 風格 | Determinism | Model Cost | 最適合 |
|-------|-------|-------------|------------|----------|
| **Passive** | 純上下文 | N/A | 最低 | 背景知識、程式碼規範 |
| **Instructional** | 規則 + 準則 | 中等 | 低 | Code review、風格指南 |
| **CLI Wrapper** | 呼叫已編譯的 binary | **高** | **低** | 自動化、整合、資料處理 |
| **Workflow** | 多步驟並附驗證 | 中等 | 中等 | 部署流程、遷移 |
| **Generative** | 要求 agent 撰寫程式碼 | 低 | 高 | Scaffolding、程式碼生成 |

**關鍵洞察：盡可能往光譜左側移動。** 越簡單的 skill 越可靠、執行成本越低，也能在更多模型上運作。

---

## 原則一：Determinism 優先

一個設計良好的 skill，最重要的特質就是 **determinism（決定性）** — 相同的輸入應該每次都產生相同的輸出。

### 為什麼 determinism 很重要

- **便宜的模型也能執行決定性的 skill。** 一個說「執行 `eslint --fix`」的 skill 在任何模型上都能運作。一個說「分析程式碼並提出改善建議」的 skill 則需要昂貴的推理能力。
- **決定性的 skill 不會壞。** CLI 指令要嘛成功、要嘛帶著明確錯誤失敗。模稜兩可的指示則會靜默失敗或產生不一致的結果。
- **團隊需要可預測性。** 如果同一個 skill 對不同團隊成員產生不同結果，就會造成困惑。

### 如何提高 determinism

**指令優先於描述：**

```markdown
# ✅ Deterministic — any model can run this
Run the formatter:
`prettier --write "src/**/*.{ts,tsx}"`

# ❌ Non-deterministic — model must reason about formatting rules
Format the code following the project's style conventions.
Ensure consistent indentation, trailing commas, and import ordering.
```

**腳本優先於指示：**

```markdown
# ✅ Deterministic — execute a script
Run `./scripts/deploy.sh staging` to deploy.

# ❌ Non-deterministic — model must reconstruct the deploy flow
Deploy to staging:
1. Build the project
2. Run tests
3. Push to the staging branch
4. Wait for CI
5. Verify the deployment
```

**明確數值優先於判斷：**

```markdown
# ✅ Deterministic
Block any file larger than 100KB.

# ❌ Non-deterministic
Block files that are too large.
```

---

## 原則二：CLI Wrapper Pattern

打造可靠 skill 最強大的技巧：**把邏輯包進已編譯的 CLI binary，再讓 skill 去呼叫它。**

### 這個 pattern

```
my-tool/                  # Compiled binary (Go, Rust, Swift, Bun)
├── main.go
└── ...

my-skill/                 # Skill just calls the binary
└── SKILL.md
```

```markdown title="SKILL.md"
---
name: my-tool
description: Processes data files with my-tool CLI
---

# My Tool

Use the `my-tool` CLI for data processing tasks.

## Commands

- `my-tool convert <input> <output>` — Convert between formats
- `my-tool validate <file>` — Check file integrity
- `my-tool analyze <file> --json` — Output analysis as JSON
```

### 為什麼這樣有效

1. **零執行期相依性。** Go 或 Rust binary 沒有 `node_modules`、不需要 `pip install`、也沒有版本衝突。
2. **Binary 行為是固定的。** 同一個 binary 版本在每台機器上都會產生相同結果。
3. **安全性。** 沒有來自 transitive dependencies 的供應鏈風險，binary 本身是自我完備的。
4. **對便宜的模型也有效。** 即使是最小的模型也能執行 `my-tool convert a.csv b.json`。

### 真實案例

[Peter Steinberger](https://github.com/steipete)（PSPDFKit 創辦人）為他的 AI agents 需要的每一件事都打造了已編譯的 CLI：

| CLI | 語言 | 用途 |
|-----|----------|---------|
| `gogcli` | Go | Google Suite（Gmail、Calendar、Drive） |
| `peekaboo` | Swift | macOS 螢幕截圖，供 AI vision 使用 |
| `imsg` | Swift | 收發 iMessage |
| `mcporter` | Bun | 把 MCP servers 轉換成 CLI binaries |

他的做法：**SKILL.md 只是一行指示，所有工作都由 binary 完成。**

> "Agents are really, really good at calling CLIs — actually much better than calling MCPs. You don't have to clutter up your context and you can use all the features on demand."
> — [Peekaboo 2.0](https://steipete.me/posts/2025/peekaboo-2-freeing-the-cli-from-its-mcp-shackles)

### 什麼時候該用這個 pattern

- 你有複雜邏輯，不該寫在 prompt 裡
- 你需要在團隊成員之間有可重現的行為
- 你正在整合外部服務（API、資料庫、雲端）
- 安全性很重要（不想有相依性供應鏈風險）

### 什麼時候不該用這個 pattern

- 單純的知識或慣例（改用 instructional skills）
- 邏輯每次都真的不一樣（改用 generative skills）
- 你沒時間打造 CLI（先從指示開始，之後再重構）

---

## 原則三：Progressive Disclosure

不要把所有東西都塞進 SKILL.md。把內容分層，讓 AI 只載入它需要的部分。

### 三個層次

```
my-skill/
├── SKILL.md           # Layer 1: Always loaded (~100 tokens in description)
├── references/        # Layer 2: Loaded on demand
│   ├── api-guide.md
│   └── patterns.md
├── scripts/           # Layer 3: Executed, not loaded into context
│   └── validate.sh
└── examples/          # Layer 3: Referenced by path
    └── sample.json
```

**Layer 1 — Metadata**（永遠在 context 中）：
frontmatter 裡的 `name` + `description`。維持在 200 字元以內。這是 AI 用來決定是否啟用該 skill 的依據。

**Layer 2 — Body + References**（skill 啟用時才載入）：
SKILL.md 的主文以及任何被引用的檔案。SKILL.md 維持在 500 行以內，詳細文件放進 `references/`。

**Layer 3 — Scripts + Assets**（執行或依路徑引用，從不載入）：
透過 Bash 執行的腳本、複製到輸出的樣板。這些不會消耗 context tokens。

### Context window 是共享資源

你的 skill 裡每一個 token 都在跟使用者的程式碼、對話歷史，以及其他 skills 競爭。問問自己：

> 「這一行值得花這些 context tokens 嗎？」

**修改前：**
```markdown
## Background

PDF (Portable Document Format) was developed by Adobe in 1993. It's widely used
for document exchange because it preserves formatting across platforms. PDFs can
contain text, images, forms, and multimedia. The PDF specification is maintained
by ISO as ISO 32000...

## Instructions

Use pdfplumber to extract text from PDF files.
```

**修改後：**
```markdown
Use `pdfplumber` for text extraction:

    import pdfplumber
    with pdfplumber.open("file.pdf") as pdf:
        text = pdf.pages[0].extract_text()
```

AI 早就知道什麼是 PDF 了。只需要加上它不知道的東西。

---

## 原則四：讓複雜度匹配風險

使用「窄橋 vs 開放場地」這個判斷法則：

| 情境 | 風險 | 自由度 | 做法 |
|----------|------|---------|----------|
| 資料庫遷移 | 高 | 低 | 明確的指令、驗證步驟、回滾計畫 |
| Code review | 低 | 高 | 一般性準則，讓 AI 自行判斷 |
| 部署到 production | 高 | 低 | 附明確步驟與檢查的腳本 |
| 撰寫文件 | 低 | 高 | 風格指南 + 範例 |

**高風險操作需要低自由度的 skill：**

```markdown
## Database Migration

⚠️ Follow these steps EXACTLY in order:

1. Create backup: `pg_dump -Fc mydb > backup_$(date +%Y%m%d).dump`
2. Run migration: `psql mydb < migrations/0042_add_index.sql`
3. Verify: `psql mydb -c "SELECT count(*) FROM pg_indexes WHERE indexname = 'idx_users_email'"`
4. If verification fails, rollback: `pg_restore -d mydb backup_*.dump`
```

**低風險操作可以是高自由度：**

```markdown
## Code Review Guidelines

When reviewing code, consider:
- Are there obvious bugs or edge cases?
- Is the code readable and well-structured?
- Are there performance concerns?

Adapt your review depth to the change size.
```

---

## 原則五：先設計介面

在撰寫 skill 之前，先定義它的契約 — 什麼會觸發它、它做什麼，以及它產出什麼。

### 五個要回答的問題

1. **這個 skill 什麼時候該啟用？** 把 `description` 欄位寫得像是在教一位新團隊成員何時該用這個工具。
2. **它需要什麼輸入？** 參數、檔案、環境狀態？
3. **成功長什麼樣子？** 特定的輸出格式、建立的檔案、執行的指令？
4. **它不該做什麼？** 明確的排除項目能避免範圍蔓延。
5. **怎麼驗證它有效運作？** 要包含一個驗證步驟。

### 範本

```markdown
---
name: {name}
description: {what it does}. Use when {trigger condition}.
---

# {Name}

{One sentence: what this does.}

## When to Use

{Specific trigger conditions — be precise}

## Instructions

{Steps — ordered, concrete, verifiable}

## Verify

{How to confirm it worked}

## When NOT to Use

{Explicit exclusions}
```

---

## Anti-Patterns

讓 skill 變得不可靠的常見錯誤：

### 1. 大雜燴

```markdown
# ❌ Too many responsibilities
This skill handles code review, testing, deployment,
documentation updates, and changelog generation.
```

**修正方式：** 一個 skill = 一個目的。拆成多個獨立的 skills。

### 2. 模稜兩可的指示

```markdown
# ❌ Agent must guess what "properly" means
Ensure the code is properly formatted and follows best practices.
```

**修正方式：** 指名具體的工具和規則。

```markdown
# ✅ Specific and actionable
Run `prettier --write .` to format. Run `eslint --fix .` to lint.
```

### 3. 解釋 AI 早就知道的東西

```markdown
# ❌ Wasting context tokens
React is a JavaScript library for building user interfaces.
Components are reusable pieces of UI. Props are passed from
parent to child components...
```

**修正方式：** 只加上 AI 不知道的東西 — 你專案特有的慣例、內部 API、領域規則。

### 4. 選項太多

```markdown
# ❌ Choice paralysis
You can use pdfplumber, PyMuPDF, pdfminer, tabula-py, or camelot
depending on the use case...
```

**修正方式：** 給一個預設選項，只在需要時才提及替代方案。

```markdown
# ✅ Clear default
Use `pdfplumber` for text extraction. For scanned PDFs, fall back to `pytesseract`.
```

### 5. 沒有驗證步驟

```markdown
# ❌ No way to confirm success
Deploy the application to staging.
```

**修正方式：** 永遠附上如何驗證。

```markdown
# ✅ Verifiable
Deploy to staging:
1. Run `make deploy-staging`
2. Verify: `curl -s https://staging.example.com/health | jq .status`
   Expected: `"ok"`
```

### 6. 寫死的路徑

```markdown
# ❌ Breaks on other machines
Edit the file at /Users/john/projects/my-app/src/config.ts
```

**修正方式：** 使用相對路徑或環境變數。

---

## 測試你的 Skills

### 跨模型測試

在多個模型層級上測試：
- **便宜的模型**（例如 Haiku）：它能照著指示做嗎？如果不行，簡化它。
- **中階模型**（例如 Sonnet）：它產出的結果一致嗎？
- **頂級模型**（例如 Opus）：它會遵守邊界，還是會「超出範圍地改善」？

### 簡潔性測試

> 如果便宜的模型無法可靠地執行你的 skill，代表這個 skill 太複雜了。

這是最強烈的訊號，代表你需要：
- 把邏輯抽取到腳本或 CLI binary 中
- 減少指示中的模糊之處
- 用明確的指令取代描述

### 迭代循環

```
Write skill → Sync → Test in AI CLI → Observe behavior → Edit → Repeat
```

用 `skillshare sync` 部署變更，然後在你的 AI CLI 中測試。留意：
- AI 是否在正確的時機啟用了這個 skill？
- 它是否依序執行步驟？
- 它是否跳過或即興發揮步驟？
- 驗證步驟是否能抓出失敗？

---

## 總結

| 原則 | 一句話 |
|-----------|-----------|
| **Determinism First** | 指令優先於描述、腳本優先於指示 |
| **CLI Wrapper Pattern** | 複雜邏輯 → 已編譯的 binary，skill → 薄薄的 wrapper |
| **Progressive Disclosure** | 分層內容：metadata → body → references → scripts |
| **Match Complexity to Risk** | 高風險 = 明確步驟；低風險 = 準則 |
| **Design Interface First** | 撰寫前先定義觸發條件、輸入、輸出、排除項目 |

---

## 接下來：Design Patterns

理解上述原則之後，參考 [Skill Design Patterns](./skill-design-patterns.md)，裡面有五種結構化範本（Tool Wrapper、Generator、Reviewer、Inversion、Pipeline）可作為你 skills 的起點。

---

## 另請參閱

- [Creating Skills](/docs/how-to/daily-tasks/creating-skills) — 逐步建立指南
- [Best Practices](/docs/how-to/daily-tasks/best-practices) — 命名、組織、版本控制
- [Skill Format](/docs/understand/skill-format) — SKILL.md 結構與 metadata
- [Securing Your Skills](/docs/how-to/advanced/security) — 安全掃描與稽核
