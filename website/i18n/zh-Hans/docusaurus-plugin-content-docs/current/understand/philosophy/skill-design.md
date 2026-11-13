---
sidebar_position: 3
---

# Skill Design

如何编写可靠运行的 skill——选择合适的复杂度级别、最大化确定性，并使用渐进式披露。

:::tip 什么时候这很重要？
如果你的 skill 表现不稳定，如果廉价模型在你的 skill 上失败，或者你正在为团队构建 skill——本指南将帮助你编写**可靠、安全且高效**的 skill。
:::

## Skill 谱系

并非所有 skill 都生来平等。理解你的 skill 在复杂度谱系中所处的位置，有助于你做出正确的设计选择：

| 级别 | 风格 | 确定性 | 模型成本 | 最适用于 |
|-------|-------|-------------|------------|----------|
| **Passive** | 仅提供上下文 | 不适用 | 最低 | 背景知识、编码规范 |
| **Instructional** | 规则 + 准则 | 中等 | 低 | 代码审查、风格指南 |
| **CLI Wrapper** | 调用编译好的二进制文件 | **高** | **低** | 自动化、集成、数据处理 |
| **Workflow** | 带验证的多步骤流程 | 中等 | 中等 | 部署流水线、迁移 |
| **Generative** | 要求 agent 编写代码 | 低 | 高 | 脚手架搭建、代码生成 |

**关键洞察：尽可能在这个谱系上向左移动。** 更简单的 skill 更可靠、执行成本更低，并且能在更多模型上工作。

---

## 原则一：确定性优先

设计良好的 skill 最重要的品质是**确定性**——相同的输入应该每次都产生相同的输出。

### 为什么确定性很重要

- **廉价模型可以运行确定性 skill。** 一个说 "运行 `eslint --fix`" 的 skill 在任何模型上都能工作。一个说 "分析代码并提出改进建议" 的 skill 则需要昂贵的推理。
- **确定性 skill 不会出错。** CLI 命令要么成功，要么伴随明确的错误失败。含糊的指令则会静默失败或产生不一致的结果。
- **团队需要可预测性。** 如果一个 skill 对不同的团队成员产生不同的结果，就会造成混乱。

### 如何提高确定性

**优先使用命令而非描述：**

```markdown
# ✅ Deterministic — any model can run this
Run the formatter:
`prettier --write "src/**/*.{ts,tsx}"`

# ❌ Non-deterministic — model must reason about formatting rules
Format the code following the project's style conventions.
Ensure consistent indentation, trailing commas, and import ordering.
```

**优先使用脚本而非指令：**

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

**优先使用明确的数值而非主观判断：**

```markdown
# ✅ Deterministic
Block any file larger than 100KB.

# ❌ Non-deterministic
Block files that are too large.
```

---

## 原则二：CLI Wrapper 模式

打造可靠 skill 最强大的技巧：**将逻辑封装在编译好的 CLI 二进制文件中，然后让 skill 去调用它。**

### 该模式

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

### 为什么这样有效

1. **零运行时依赖。** Go 或 Rust 二进制文件没有 `node_modules`，不需要 `pip install`，也没有版本冲突。
2. **二进制行为是固定的。** 同一个二进制文件版本在每台机器上都会产生相同的结果。
3. **安全性。** 没有来自传递依赖的供应链风险。二进制文件是自包含的。
4. **适用于廉价模型。** 即使是最小的模型也能执行 `my-tool convert a.csv b.json`。

### 真实案例

[Peter Steinberger](https://github.com/steipete)（PSPDFKit 创始人）为他的 AI agent 所需的一切都构建了编译好的 CLI：

| CLI | 语言 | 用途 |
|-----|----------|---------|
| `gogcli` | Go | Google Suite（Gmail、Calendar、Drive） |
| `peekaboo` | Swift | 用于 AI 视觉的 macOS 截图 |
| `imsg` | Swift | 发送/接收 iMessage |
| `mcporter` | Bun | 将 MCP 服务器转换为 CLI 二进制文件 |

他的方法：**SKILL.md 只是一行指令，所有工作都由二进制文件完成。**

> "Agent 非常非常擅长调用 CLI——实际上比调用 MCP 好得多。你不必用上下文塞满内容，而且可以按需使用所有功能。"
> —— [Peekaboo 2.0](https://steipete.me/posts/2025/peekaboo-2-freeing-the-cli-from-its-mcp-shackles)

### 何时使用此模式

- 你有不应存在于 prompt 中的复杂逻辑
- 你需要在团队成员之间实现可复现的行为
- 你正在与外部服务（API、数据库、云）集成
- 安全性很重要（没有依赖供应链）

### 何时不应使用此模式

- 简单的知识或规范（改用 instructional skill）
- 逻辑确实每次都不同（改用 generative skill）
- 你没有时间构建 CLI（先从指令开始，之后再重构）

---

## 原则三：渐进式披露

不要把所有东西都塞进 SKILL.md。分层组织你的内容，让 AI 只加载它需要的部分。

### 三个层级

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

**第一层——元数据**（始终在上下文中）：
frontmatter 中的 `name` + `description`。保持在 200 字符以内。这是 AI 用来决定是否激活该 skill 的依据。

**第二层——正文 + 引用**（skill 激活时加载）：
SKILL.md 的正文以及任何被引用的文件。保持 SKILL.md 在 500 行以内。将详细文档放在 `references/` 中。

**第三层——脚本 + 资源**（执行或按路径引用，从不加载）：
通过 Bash 运行的脚本、被复制到输出的模板。这些不消耗上下文 token。

### 上下文窗口是共享资源

skill 中的每一个 token 都在与用户的代码、对话历史以及其他 skill 竞争空间。问问自己：

> "这一行值得花费它所占用的上下文 token 吗？"

**改进前：**
```markdown
## Background

PDF (Portable Document Format) was developed by Adobe in 1993. It's widely used
for document exchange because it preserves formatting across platforms. PDFs can
contain text, images, forms, and multimedia. The PDF specification is maintained
by ISO as ISO 32000...

## Instructions

Use pdfplumber to extract text from PDF files.
```

**改进后：**
```markdown
Use `pdfplumber` for text extraction:

    import pdfplumber
    with pdfplumber.open("file.pdf") as pdf:
        text = pdf.pages[0].extract_text()
```

AI 已经知道 PDF 是什么。只需添加它不知道的内容。

---

## 原则四：让复杂度匹配风险

使用 "窄桥 vs 开阔地" 这一启发式方法：

| 场景 | 风险 | 自由度 | 方法 |
|----------|------|---------|----------|
| 数据库迁移 | 高 | 低 | 精确命令、验证步骤、回滚计划 |
| 代码审查 | 低 | 高 | 一般性准则，让 AI 自主判断 |
| 部署到生产环境 | 高 | 低 | 带明确步骤和检查的脚本 |
| 编写文档 | 低 | 高 | 风格指南 + 示例 |

**高风险操作需要低自由度的 skill：**

```markdown
## Database Migration

⚠️ Follow these steps EXACTLY in order:

1. Create backup: `pg_dump -Fc mydb > backup_$(date +%Y%m%d).dump`
2. Run migration: `psql mydb < migrations/0042_add_index.sql`
3. Verify: `psql mydb -c "SELECT count(*) FROM pg_indexes WHERE indexname = 'idx_users_email'"`
4. If verification fails, rollback: `pg_restore -d mydb backup_*.dump`
```

**低风险操作可以是高自由度的：**

```markdown
## Code Review Guidelines

When reviewing code, consider:
- Are there obvious bugs or edge cases?
- Is the code readable and well-structured?
- Are there performance concerns?

Adapt your review depth to the change size.
```

---

## 原则五：先设计接口

在编写 skill 之前，先定义它的契约——什么触发它、它做什么、它产生什么。

### 需要回答的五个问题

1. **这个 skill 应该在什么时候激活？** 把 `description` 字段写得像是在教一位新的团队成员什么时候该用这个工具。
2. **它需要什么输入？** 参数、文件、环境状态？
3. **成功是什么样子？** 具体的输出格式、创建的文件、运行的命令？
4. **它不应该做什么？** 明确的排除项能防止范围蔓延。
5. **如何验证它成功运行了？** 包含一个验证步骤。

### 模板

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

## 反模式

让 skill 变得不可靠的常见错误：

### 1. 大杂烩

```markdown
# ❌ Too many responsibilities
This skill handles code review, testing, deployment,
documentation updates, and changelog generation.
```

**修正方法：** 一个 skill 对应一个目的。拆分成多个独立的 skill。

### 2. 含糊的指令

```markdown
# ❌ Agent must guess what "properly" means
Ensure the code is properly formatted and follows best practices.
```

**修正方法：** 指明具体的工具和规则。

```markdown
# ✅ Specific and actionable
Run `prettier --write .` to format. Run `eslint --fix .` to lint.
```

### 3. 解释 AI 已经知道的东西

```markdown
# ❌ Wasting context tokens
React is a JavaScript library for building user interfaces.
Components are reusable pieces of UI. Props are passed from
parent to child components...
```

**修正方法：** 只添加 AI 不知道的内容——你项目特有的规范、内部 API、领域规则。

### 4. 选项太多

```markdown
# ❌ Choice paralysis
You can use pdfplumber, PyMuPDF, pdfminer, tabula-py, or camelot
depending on the use case...
```

**修正方法：** 给出一个默认选项，只有在需要时才提及替代方案。

```markdown
# ✅ Clear default
Use `pdfplumber` for text extraction. For scanned PDFs, fall back to `pytesseract`.
```

### 5. 没有验证步骤

```markdown
# ❌ No way to confirm success
Deploy the application to staging.
```

**修正方法：** 始终包含如何验证。

```markdown
# ✅ Verifiable
Deploy to staging:
1. Run `make deploy-staging`
2. Verify: `curl -s https://staging.example.com/health | jq .status`
   Expected: `"ok"`
```

### 6. 硬编码路径

```markdown
# ❌ Breaks on other machines
Edit the file at /Users/john/projects/my-app/src/config.ts
```

**修正方法：** 使用相对路径或环境变量。

---

## 测试你的 Skill

### 跨模型测试

在多个模型层级上进行测试：
- **廉价模型**（例如 Haiku）：它能遵循指令吗？如果不能，就简化 skill。
- **中端模型**（例如 Sonnet）：它能产生一致的结果吗？
- **顶级模型**（例如 Opus）：它遵守边界了吗，还是会 "改进" 到超出范围？

### 简洁性测试

> 如果一个廉价模型无法可靠地执行你的 skill，那么这个 skill 就太复杂了。

这是最强烈的信号，说明你需要：
- 把逻辑提取到脚本或 CLI 二进制文件中
- 减少指令中的歧义
- 添加明确的命令而不是描述

### 迭代循环

```
Write skill → Sync → Test in AI CLI → Observe behavior → Edit → Repeat
```

使用 `skillshare sync` 部署变更，然后在你的 AI CLI 中测试。留意：
- AI 是否在正确的时机激活了该 skill？
- 它是否按顺序执行了各个步骤？
- 它是否跳过或即兴发挥了某些步骤？
- 验证步骤是否能捕获失败？

---

## 总结

| 原则 | 一句话概括 |
|-----------|-----------|
| **确定性优先** | 命令优于描述，脚本优于指令 |
| **CLI Wrapper 模式** | 复杂逻辑 → 编译好的二进制文件，skill → 轻薄的封装层 |
| **渐进式披露** | 分层组织内容：元数据 → 正文 → 引用 → 脚本 |
| **让复杂度匹配风险** | 高风险 = 精确步骤；低风险 = 一般准则 |
| **先设计接口** | 在编写之前先定义触发条件、输入、输出、排除项 |

---

## 接下来：Design Patterns

理解了上述原则之后，请参阅 [Skill Design Patterns](./skill-design-patterns.md)，了解五种可作为 skill 起点的结构化模板（Tool Wrapper、Generator、Reviewer、Inversion、Pipeline）。

---

## 另见

- [Creating Skills](/docs/how-to/daily-tasks/creating-skills) —— 分步创建指南
- [Best Practices](/docs/how-to/daily-tasks/best-practices) —— 命名、组织、版本控制
- [Skill Format](/docs/understand/skill-format) —— SKILL.md 结构与元数据
- [Securing Your Skills](/docs/how-to/advanced/security) —— 安全扫描与 audit
