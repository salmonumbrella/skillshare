---
sidebar_position: 3
---

# Skill Design

안정적으로 작동하는 skill을 작성하는 방법 — 올바른 복잡도 수준을 선택하고, 결정론성을 극대화하며, progressive disclosure를 사용하기.

:::tip 언제 중요한가요?
skill이 일관성 없이 작동한다면, 저렴한 모델이 여러분의 skill에서 실패한다면, 또는 팀을 위한 skill을 만들고 있다면 — 이 가이드는 **신뢰할 수 있고, 안전하며, 효율적인** skill을 작성하는 데 도움이 됩니다.
:::

## Skill 스펙트럼

모든 skill이 동일하게 만들어지는 것은 아닙니다. skill이 복잡도 스펙트럼 상 어디에 위치하는지 이해하면 올바른 설계 선택을 할 수 있습니다:

| 레벨 | 스타일 | 결정론성 | 모델 비용 | 적합한 대상 |
|-------|-------|-------------|------------|----------|
| **Passive** | 컨텍스트만 | N/A | 최저 | 배경 지식, 코딩 표준 |
| **Instructional** | 규칙 + 가이드라인 | 중간 | 낮음 | 코드 리뷰, 스타일 가이드 |
| **CLI Wrapper** | 컴파일된 바이너리 호출 | **높음** | **낮음** | 자동화, 통합, 데이터 처리 |
| **Workflow** | 검증이 포함된 다단계 | 중간 | 중간 | 배포 파이프라인, 마이그레이션 |
| **Generative** | agent에게 코드 작성을 요청 | 낮음 | 높음 | 스캐폴딩, 코드 생성 |

**핵심 통찰: 가능하면 항상 이 스펙트럼에서 왼쪽으로 이동하세요.** 더 단순한 skill이 더 신뢰할 수 있고, 실행 비용이 더 저렴하며, 더 많은 모델에서 작동합니다.

---

## 원칙 1: 결정론성 우선

잘 설계된 skill의 가장 중요한 특성은 **결정론성**입니다 — 같은 입력은 매번 같은 출력을 만들어야 합니다.

### 결정론성이 중요한 이유

- **저렴한 모델도 결정론적인 skill을 실행할 수 있습니다.** "`eslint --fix`를 실행하라"는 skill은 어떤 모델에서도 작동합니다. "코드를 분석하고 개선안을 제안하라"는 skill은 값비싼 추론이 필요합니다.
- **결정론적인 skill은 깨지지 않습니다.** CLI 명령은 성공하거나 명확한 에러와 함께 실패합니다. 모호한 지침은 조용히 실패하거나 일관되지 않은 결과를 만듭니다.
- **팀에는 예측 가능성이 필요합니다.** skill이 팀원마다 다른 결과를 만든다면 혼란을 야기합니다.

### 결정론성을 높이는 방법

**설명보다 명령을 선호하세요:**

```markdown
# ✅ Deterministic — any model can run this
Run the formatter:
`prettier --write "src/**/*.{ts,tsx}"`

# ❌ Non-deterministic — model must reason about formatting rules
Format the code following the project's style conventions.
Ensure consistent indentation, trailing commas, and import ordering.
```

**지침보다 스크립트를 선호하세요:**

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

**판단보다 명시적인 값을 선호하세요:**

```markdown
# ✅ Deterministic
Block any file larger than 100KB.

# ❌ Non-deterministic
Block files that are too large.
```

---

## 원칙 2: CLI Wrapper 패턴

신뢰할 수 있는 skill을 위한 가장 강력한 기법: **로직을 컴파일된 CLI 바이너리에 감싼 다음, skill이 그것을 호출하게 하세요.**

### 패턴

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

### 이것이 작동하는 이유

1. **런타임 의존성 없음.** Go나 Rust 바이너리에는 `node_modules`도, `pip install`도, 버전 충돌도 없습니다.
2. **바이너리 동작이 고정됨.** 동일한 바이너리 버전은 모든 머신에서 동일한 결과를 만듭니다.
3. **보안.** 전이적 의존성으로 인한 공급망 위험이 없습니다. 바이너리는 자체 완결적입니다.
4. **저렴한 모델에서도 작동함.** 가장 작은 모델조차 `my-tool convert a.csv b.json`을 실행할 수 있습니다.

### 실제 사례

[Peter Steinberger](https://github.com/steipete)(PSPDFKit 창업자)는 AI agent에 필요한 모든 것을 위해 컴파일된 CLI를 만듭니다:

| CLI | 언어 | 목적 |
|-----|------|------|
| `gogcli` | Go | Google Suite (Gmail, Calendar, Drive) |
| `peekaboo` | Swift | AI vision을 위한 macOS 스크린샷 |
| `imsg` | Swift | iMessage 송수신 |
| `mcporter` | Bun | MCP 서버를 CLI 바이너리로 변환 |

그의 접근 방식: **SKILL.md는 한 줄짜리 지침이고, 모든 작업은 바이너리가 처리합니다.**

> "Agent는 CLI 호출을 정말, 정말 잘합니다 — 실제로 MCP 호출보다 훨씬 낫습니다. 컨텍스트를 어지럽힐 필요도 없고 필요할 때마다 모든 기능을 사용할 수 있습니다."
> — [Peekaboo 2.0](https://steipete.me/posts/2025/peekaboo-2-freeing-the-cli-from-its-mcp-shackles)

### 이 패턴을 언제 사용하나요

- 프롬프트에 담기에는 부적절한 복잡한 로직이 있을 때
- 팀원 간 재현 가능한 동작이 필요할 때
- 외부 서비스(API, 데이터베이스, 클라우드)와 통합할 때
- 보안이 중요할 때 (의존성 공급망 없음)

### 이 패턴을 사용하지 말아야 할 때

- 단순한 지식이나 관례 (instructional skill을 대신 사용하세요)
- 로직이 매번 정말로 달라질 때 (generative skill을 사용하세요)
- CLI를 만들 시간이 없을 때 (지침으로 시작한 뒤 나중에 리팩터링하세요)

---

## 원칙 3: Progressive Disclosure

SKILL.md에 모든 것을 쏟아붓지 마세요. AI가 필요한 것만 로드하도록 콘텐츠를 계층화하세요.

### 세 계층

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

**Layer 1 — 메타데이터** (항상 컨텍스트에 있음):
frontmatter의 `name` + `description`. 200자 이내로 유지하세요. 이는 AI가 skill을 활성화할지 결정하는 데 사용하는 부분입니다.

**Layer 2 — 본문 + 참조** (skill이 활성화될 때 로드됨):
SKILL.md 본문과 참조되는 파일들. SKILL.md는 500줄 이내로 유지하세요. 상세한 문서는 `references/`에 두세요.

**Layer 3 — 스크립트 + 자산** (실행되거나 경로로 참조되며, 절대 로드되지 않음):
Bash를 통해 실행되는 스크립트, 출력에 복사되는 템플릿. 이들은 컨텍스트 토큰을 소비하지 않습니다.

### 컨텍스트 윈도우는 공유 자원입니다

여러분의 skill 안의 모든 토큰은 사용자의 코드, 대화 히스토리, 다른 skill과 경쟁합니다. 스스로에게 물어보세요:

> "이 줄은 그것이 드는 컨텍스트 토큰만큼의 가치가 있는가?"

**개선 전:**
```markdown
## Background

PDF (Portable Document Format) was developed by Adobe in 1993. It's widely used
for document exchange because it preserves formatting across platforms. PDFs can
contain text, images, forms, and multimedia. The PDF specification is maintained
by ISO as ISO 32000...

## Instructions

Use pdfplumber to extract text from PDF files.
```

**개선 후:**
```markdown
Use `pdfplumber` for text extraction:

    import pdfplumber
    with pdfplumber.open("file.pdf") as pdf:
        text = pdf.pages[0].extract_text()
```

AI는 이미 PDF가 무엇인지 알고 있습니다. AI가 모르는 것만 추가하세요.

---

## 원칙 4: 복잡도를 위험도에 맞추기

"좁은 다리 vs 열린 들판" 휴리스틱을 사용하세요:

| 시나리오 | 위험 | 자유도 | 접근 방식 |
|----------|------|---------|----------|
| 데이터베이스 마이그레이션 | 높음 | 낮음 | 정확한 명령, 검증 단계, 롤백 계획 |
| 코드 리뷰 | 낮음 | 높음 | 일반 가이드라인, AI가 판단하도록 함 |
| 프로덕션 배포 | 높음 | 낮음 | 명시적인 단계와 검사가 있는 스크립트 |
| 문서 작성 | 낮음 | 높음 | 스타일 가이드 + 예시 |

**고위험 작업에는 낮은 자유도의 skill이 필요합니다:**

```markdown
## Database Migration

⚠️ Follow these steps EXACTLY in order:

1. Create backup: `pg_dump -Fc mydb > backup_$(date +%Y%m%d).dump`
2. Run migration: `psql mydb < migrations/0042_add_index.sql`
3. Verify: `psql mydb -c "SELECT count(*) FROM pg_indexes WHERE indexname = 'idx_users_email'"`
4. If verification fails, rollback: `pg_restore -d mydb backup_*.dump`
```

**저위험 작업은 높은 자유도로 만들 수 있습니다:**

```markdown
## Code Review Guidelines

When reviewing code, consider:
- Are there obvious bugs or edge cases?
- Is the code readable and well-structured?
- Are there performance concerns?

Adapt your review depth to the change size.
```

---

## 원칙 5: 먼저 인터페이스를 설계하기

skill을 작성하기 전에 계약을 정의하세요 — 무엇이 이를 트리거하는지, 무엇을 하는지, 무엇을 만드는지.

### 답해야 할 다섯 가지 질문

1. **이 skill은 언제 활성화되어야 하나요?** 새 팀원에게 이 도구를 언제 써야 하는지 가르친다는 마음으로 `description` 필드를 작성하세요.
2. **어떤 입력이 필요한가요?** 인자, 파일, 환경 상태?
3. **성공은 어떤 모습인가요?** 특정 출력 형식, 생성되는 파일, 실행되는 명령?
4. **무엇을 하면 안 되나요?** 명시적인 제외 사항이 범위 확장을 방지합니다.
5. **작동했는지 어떻게 확인하나요?** 검증 단계를 포함하세요.

### 템플릿

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

## 안티패턴

skill을 신뢰할 수 없게 만드는 흔한 실수들:

### 1. 종합 선물세트

```markdown
# ❌ Too many responsibilities
This skill handles code review, testing, deployment,
documentation updates, and changelog generation.
```

**해결:** 하나의 skill = 하나의 목적. 별도의 skill로 분리하세요.

### 2. 모호한 지침

```markdown
# ❌ Agent must guess what "properly" means
Ensure the code is properly formatted and follows best practices.
```

**해결:** 구체적인 도구와 규칙의 이름을 명시하세요.

```markdown
# ✅ Specific and actionable
Run `prettier --write .` to format. Run `eslint --fix .` to lint.
```

### 3. AI가 이미 아는 것을 설명하기

```markdown
# ❌ Wasting context tokens
React is a JavaScript library for building user interfaces.
Components are reusable pieces of UI. Props are passed from
parent to child components...
```

**해결:** AI가 모르는 것만 추가하세요 — 여러분 프로젝트의 특정 관례, 내부 API, 도메인 규칙.

### 4. 너무 많은 선택지

```markdown
# ❌ Choice paralysis
You can use pdfplumber, PyMuPDF, pdfminer, tabula-py, or camelot
depending on the use case...
```

**해결:** 하나의 기본값을 제시하고, 필요할 때만 대안을 언급하세요.

```markdown
# ✅ Clear default
Use `pdfplumber` for text extraction. For scanned PDFs, fall back to `pytesseract`.
```

### 5. 검증 단계 없음

```markdown
# ❌ No way to confirm success
Deploy the application to staging.
```

**해결:** 항상 확인 방법을 포함하세요.

```markdown
# ✅ Verifiable
Deploy to staging:
1. Run `make deploy-staging`
2. Verify: `curl -s https://staging.example.com/health | jq .status`
   Expected: `"ok"`
```

### 6. 하드코딩된 경로

```markdown
# ❌ Breaks on other machines
Edit the file at /Users/john/projects/my-app/src/config.ts
```

**해결:** 상대 경로나 환경 변수를 사용하세요.

---

## Skill 테스트하기

### 여러 모델에서 테스트

여러 모델 등급에서 테스트하세요:
- **저렴한 모델** (예: Haiku): 지침을 따를 수 있나요? 그렇지 않다면 단순화하세요.
- **중급 모델** (예: Sonnet): 일관된 결과를 만드나요?
- **최상위 모델** (예: Opus): 경계를 존중하나요, 아니면 범위를 벗어나 "개선"하나요?

### 단순성 테스트

> 저렴한 모델이 여러분의 skill을 신뢰성 있게 실행할 수 없다면, 그 skill은 너무 복잡합니다.

이것이 다음이 필요하다는 가장 강력한 신호입니다:
- 로직을 스크립트나 CLI 바이너리로 추출
- 지침의 모호함을 줄임
- 설명 대신 명시적인 명령을 추가

### 반복 루프

```
Write skill → Sync → Test in AI CLI → Observe behavior → Edit → Repeat
```

`skillshare sync`를 사용해 변경 사항을 배포한 다음 AI CLI에서 테스트하세요. 다음을 관찰하세요:
- AI가 올바른 시점에 skill을 활성화하나요?
- 단계를 순서대로 따르나요?
- 단계를 건너뛰거나 임의로 바꾸나요?
- 검증 단계가 실패를 잡아내나요?

---

## 요약

| 원칙 | 한 줄 요약 |
|-----------|-----------|
| **결정론성 우선** | 설명보다 명령, 지침보다 스크립트 |
| **CLI Wrapper 패턴** | 복잡한 로직 → 컴파일된 바이너리, skill → 얇은 wrapper |
| **Progressive Disclosure** | 콘텐츠 계층화: 메타데이터 → 본문 → 참조 → 스크립트 |
| **복잡도를 위험도에 맞추기** | 고위험 = 정확한 단계; 저위험 = 가이드라인 |
| **먼저 인터페이스 설계** | 작성 전에 트리거, 입력, 출력, 제외 사항을 정의 |

---

## 다음: Design Patterns

위의 원칙들을 이해했다면, [Skill Design Patterns](./skill-design-patterns.md)에서 skill의 출발점으로 사용할 수 있는 다섯 가지 구조적 템플릿(Tool Wrapper, Generator, Reviewer, Inversion, Pipeline)을 확인하세요.

---

## 참고

- [Creating Skills](/docs/how-to/daily-tasks/creating-skills) — 단계별 생성 가이드
- [Best Practices](/docs/how-to/daily-tasks/best-practices) — 이름 짓기, 정리, 버전 관리
- [Skill Format](/docs/understand/skill-format) — SKILL.md 구조와 메타데이터
- [Securing Your Skills](/docs/how-to/advanced/security) — 보안 스캔과 audit
