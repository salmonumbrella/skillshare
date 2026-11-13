---
sidebar_position: 3
---

# Skill 設計

信頼性の高い Skill を書く方法 — 適切な複雑さのレベルを選び、決定性（determinism）を最大化し、progressive disclosure（段階的開示）を活用します。

:::tip これが重要になるのはどんなとき?
Skill の動作が安定しない、安価なモデルで Skill が失敗する、あるいはチームのために Skill を作っている — そんなときこのガイドが、**信頼性が高く、安全で、効率的な** Skill を書く助けになります。
:::

## Skill のスペクトラム

すべての Skill が同じというわけではありません。自分の Skill が複雑さのスペクトラムのどこに位置するかを理解することで、適切な設計判断ができるようになります。

| レベル | スタイル | 決定性 | モデルコスト | 最適な用途 |
|-------|-------|-------------|------------|----------|
| **Passive** | コンテキストのみ | N/A | 最低 | 背景知識、コーディング規約 |
| **Instructional** | ルール + ガイドライン | 中 | 低 | コードレビュー、スタイルガイド |
| **CLI Wrapper** | コンパイル済みバイナリを呼び出す | **高** | **低** | 自動化、統合、データ処理 |
| **Workflow** | 検証を伴う複数ステップ | 中 | 中 | デプロイパイプライン、マイグレーション |
| **Generative** | エージェントにコードを書かせる | 低 | 高 | スキャフォールディング、コード生成 |

**重要なのは、可能な限りこのスペクトラムの左側に寄せることです。** シンプルな Skill ほど信頼性が高く、実行コストが安く、より多くのモデルで動作します。

---

## 原則1: 決定性を最優先する

よく設計された Skill にとって最も重要な性質は**決定性（determinism）** です — 同じ入力から常に同じ出力が得られること。

### 決定性が重要な理由

- **安価なモデルでも決定的な Skill は実行できます。** 「`eslint --fix` を実行する」という Skill はどのモデルでも動作します。「コードを分析して改善案を提案する」という Skill は高コストな推論を必要とします。
- **決定的な Skill は壊れません。** CLI コマンドは成功するか、明確なエラーで失敗するかのどちらかです。曖昧な指示は静かに失敗するか、一貫性のない結果を生みます。
- **チームには予測可能性が必要です。** Skill がチームメンバーごとに異なる結果を出すと、混乱を招きます。

### 決定性を高める方法

**説明よりコマンドを優先する:**

```markdown
# ✅ Deterministic — any model can run this
Run the formatter:
`prettier --write "src/**/*.{ts,tsx}"`

# ❌ Non-deterministic — model must reason about formatting rules
Format the code following the project's style conventions.
Ensure consistent indentation, trailing commas, and import ordering.
```

**指示よりスクリプトを優先する:**

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

**判断より明示的な値を優先する:**

```markdown
# ✅ Deterministic
Block any file larger than 100KB.

# ❌ Non-deterministic
Block files that are too large.
```

---

## 原則2: CLI Wrapper パターン

信頼性の高い Skill を作るための最も強力な手法は、**ロジックをコンパイル済みの CLI バイナリにまとめ、Skill からそれを呼び出すこと**です。

### パターン

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

### これが有効な理由

1. **ランタイム依存関係がゼロ。** Go や Rust のバイナリには `node_modules` も `pip install` もバージョン競合もありません。
2. **バイナリの挙動が固定されている。** 同じバージョンのバイナリはどのマシンでも同じ結果を出します。
3. **セキュリティ。** 推移的依存関係によるサプライチェーンリスクがありません。バイナリは自己完結しています。
4. **安価なモデルでも動作する。** 最小のモデルでも `my-tool convert a.csv b.json` は実行できます。

### 実際の事例

[Peter Steinberger](https://github.com/steipete)（PSPDFKit の創業者）は、自分の AI エージェントが必要とするあらゆるもののためにコンパイル済み CLI を作っています。

| CLI | 言語 | 用途 |
|-----|----------|---------|
| `gogcli` | Go | Google Suite（Gmail、Calendar、Drive） |
| `peekaboo` | Swift | AI ビジョン向け macOS スクリーンショット |
| `imsg` | Swift | iMessage の送受信 |
| `mcporter` | Bun | MCP サーバーを CLI バイナリに変換 |

彼のアプローチ: **SKILL.md は一行の指示だけ、実際の作業はすべてバイナリが行う。**

> "Agents are really, really good at calling CLIs — actually much better than calling MCPs. You don't have to clutter up your context and you can use all the features on demand."
> — [Peekaboo 2.0](https://steipete.me/posts/2025/peekaboo-2-freeing-the-cli-from-its-mcp-shackles)

### このパターンを使うべき場面

- プロンプトに収めるべきではない複雑なロジックがある
- チームメンバー間で再現可能な挙動が必要
- 外部サービス（API、データベース、クラウド）と統合する
- セキュリティが重要（依存関係のサプライチェーンを持たない）

### このパターンを使うべきでない場面

- シンプルな知識や規約(代わりに instructional Skill を使う)
- ロジックが毎回本質的に異なる(代わりに generative Skill を使う)
- CLI を構築する時間がない（まず指示から始め、後でリファクタリングする）

---

## 原則3: Progressive Disclosure（段階的開示）

すべてを SKILL.md に詰め込まないこと。AI が必要なものだけを読み込めるよう、コンテンツを層に分けます。

### 3つの層

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

**Layer 1 — メタデータ**（常にコンテキストに含まれる）:
frontmatter の `name` と `description`。200文字以内に収めます。AI が Skill を起動すべきかどうかを判断する際に使う部分です。

**Layer 2 — 本文 + References**(Skill が起動したときに読み込まれる):
SKILL.md の本文と参照されるファイル群です。SKILL.md は500行以内に収め、詳細なドキュメントは `references/` に置きます。

**Layer 3 — スクリプト + アセット**（実行されるか、パス参照されるだけで、読み込まれることはない）:
Bash 経由で実行されるスクリプトや、出力にコピーされるテンプレートです。これらはコンテキストトークンを消費しません。

### コンテキストウィンドウは共有資源である

Skill 内のすべてのトークンは、ユーザーのコード、会話履歴、他の Skill と競合します。自問してください。

> "Is this line worth the context tokens it costs?"

**Before:**
```markdown
## Background

PDF (Portable Document Format) was developed by Adobe in 1993. It's widely used
for document exchange because it preserves formatting across platforms. PDFs can
contain text, images, forms, and multimedia. The PDF specification is maintained
by ISO as ISO 32000...

## Instructions

Use pdfplumber to extract text from PDF files.
```

**After:**
```markdown
Use `pdfplumber` for text extraction:

    import pdfplumber
    with pdfplumber.open("file.pdf") as pdf:
        text = pdf.pages[0].extract_text()
```

AI はすでに PDF が何かを知っています。AI が知らないことだけを追加してください。

---

## 原則4: 複雑さをリスクに合わせる

「狭い橋 vs 開けた野原」というヒューリスティックを使います。

| シナリオ | リスク | 自由度 | アプローチ |
|----------|------|---------|----------|
| データベースのマイグレーション | 高 | 低 | 正確なコマンド、検証手順、ロールバック計画 |
| コードレビュー | 低 | 高 | 一般的なガイドライン、AI の判断に委ねる |
| 本番環境へのデプロイ | 高 | 低 | 明示的な手順とチェックを含むスクリプト |
| ドキュメント執筆 | 低 | 高 | スタイルガイド + 例 |

**リスクの高い操作には自由度の低い Skill が必要です:**

```markdown
## Database Migration

⚠️ Follow these steps EXACTLY in order:

1. Create backup: `pg_dump -Fc mydb > backup_$(date +%Y%m%d).dump`
2. Run migration: `psql mydb < migrations/0042_add_index.sql`
3. Verify: `psql mydb -c "SELECT count(*) FROM pg_indexes WHERE indexname = 'idx_users_email'"`
4. If verification fails, rollback: `pg_restore -d mydb backup_*.dump`
```

**リスクの低い操作は自由度が高くても構いません:**

```markdown
## Code Review Guidelines

When reviewing code, consider:
- Are there obvious bugs or edge cases?
- Is the code readable and well-structured?
- Are there performance concerns?

Adapt your review depth to the change size.
```

---

## 原則5: まずインターフェースを設計する

Skill を書く前に、その契約 — 何によって起動され、何をし、何を生成するか — を定義します。

### 答えるべき5つの質問

1. **この Skill はいつ起動すべきか?** 新しいチームメンバーにこのツールをいつ使うか教えるつもりで `description` フィールドを書きます。
2. **どんな入力が必要か?** 引数、ファイル、環境の状態?
3. **成功とはどんな状態か?** 具体的な出力フォーマット、作成されるファイル、実行されるコマンド?
4. **やってはいけないことは何か?** 明示的な除外事項がスコープの肥大化を防ぎます。
5. **動作したことをどう検証するか?** 検証ステップを含めます。

### テンプレート

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

## アンチパターン

Skill の信頼性を損なうよくある間違いです。

### 1. 何でも詰め込む

```markdown
# ❌ Too many responsibilities
This skill handles code review, testing, deployment,
documentation updates, and changelog generation.
```

**対策:** 1つの Skill = 1つの目的。複数の Skill に分割します。

### 2. 曖昧な指示

```markdown
# ❌ Agent must guess what "properly" means
Ensure the code is properly formatted and follows best practices.
```

**対策:** 具体的なツールとルールを名指しします。

```markdown
# ✅ Specific and actionable
Run `prettier --write .` to format. Run `eslint --fix .` to lint.
```

### 3. AI がすでに知っていることを説明する

```markdown
# ❌ Wasting context tokens
React is a JavaScript library for building user interfaces.
Components are reusable pieces of UI. Props are passed from
parent to child components...
```

**対策:** AI が知らないこと — あなたのプロジェクト固有の規約、内部 API、ドメインルールだけを追加します。

### 4. 選択肢が多すぎる

```markdown
# ❌ Choice paralysis
You can use pdfplumber, PyMuPDF, pdfminer, tabula-py, or camelot
depending on the use case...
```

**対策:** デフォルトを1つ提示し、代替手段は必要な場合のみ言及します。

```markdown
# ✅ Clear default
Use `pdfplumber` for text extraction. For scanned PDFs, fall back to `pytesseract`.
```

### 5. 検証ステップがない

```markdown
# ❌ No way to confirm success
Deploy the application to staging.
```

**対策:** 検証方法を必ず含めます。

```markdown
# ✅ Verifiable
Deploy to staging:
1. Run `make deploy-staging`
2. Verify: `curl -s https://staging.example.com/health | jq .status`
   Expected: `"ok"`
```

### 6. ハードコードされたパス

```markdown
# ❌ Breaks on other machines
Edit the file at /Users/john/projects/my-app/src/config.ts
```

**対策:** 相対パスや環境変数を使います。

---

## Skill をテストする

### クロスモデルテスト

複数のモデルティアでテストします。

- **安価なモデル**（例: Haiku）: 指示に従えるか? 従えないなら、シンプルにします。
- **中位モデル**（例: Sonnet）: 一貫した結果を出すか?
- **最上位モデル**（例: Opus）: 境界を守るか、それともスコープを超えて「改善」してしまうか?

### シンプルさのテスト

> 安価なモデルがあなたの Skill を確実に実行できないなら、その Skill は複雑すぎます。

これは以下が必要だという最も強いシグナルです。

- ロジックをスクリプトや CLI バイナリに抽出する
- 指示の曖昧さを減らす
- 説明の代わりに明示的なコマンドを追加する

### 反復ループ

```
Write skill → Sync → Test in AI CLI → Observe behavior → Edit → Repeat
```

`skillshare sync` を使って変更をデプロイし、AI CLI でテストします。以下に注目してください。

- AI は適切なタイミングで Skill を起動しているか?
- 手順を順番通りに実行しているか?
- ステップを飛ばしたり、勝手にアレンジしたりしていないか?
- 検証ステップは失敗を検知できているか?

---

## まとめ

| 原則 | 一言でいうと |
|-----------|-----------|
| **決定性を最優先する** | 説明よりコマンド、指示よりスクリプト |
| **CLI Wrapper パターン** | 複雑なロジック → コンパイル済みバイナリ、Skill → 薄いラッパー |
| **Progressive Disclosure** | コンテンツを層に分ける: メタデータ → 本文 → references → scripts |
| **複雑さをリスクに合わせる** | リスクが高い場合は正確な手順、低い場合はガイドライン |
| **まずインターフェースを設計する** | 書き始める前にトリガー、入力、出力、除外事項を定義する |

---

## 次へ: デザインパターン

上記の原則を理解したら、[Skill Design Patterns](./skill-design-patterns.md) を参照してください。Skill の出発点として使える5つの構造テンプレート(Tool Wrapper、Generator、Reviewer、Inversion、Pipeline)を紹介しています。

---

## 関連項目

- [Creating Skills](/docs/how-to/daily-tasks/creating-skills) — ステップバイステップの作成ガイド
- [Best Practices](/docs/how-to/daily-tasks/best-practices) — 命名、整理、バージョン管理
- [Skill Format](/docs/understand/skill-format) — SKILL.md の構造とメタデータ
- [Securing Your Skills](/docs/how-to/advanced/security) — セキュリティスキャンと監査
