---
sidebar_position: 3
---

# Skill Discovery

커뮤니티에서 Skill을 찾고, 평가하고, 설치하세요.

## 개요

```mermaid
flowchart LR
    SEARCH["SEARCH"] --> BROWSE["BROWSE"] --> EVALUATE["EVALUATE"] --> INSTALL["INSTALL"] --> SYNC["SYNC"]
```

---

## 1단계: 검색

키워드로 Skill을 찾거나 인기 있는 Skill을 둘러보세요:

```bash
skillshare search              # 인기 있는 Skill 둘러보기
skillshare search pdf
skillshare search "code review"
skillshare search react
```

---

## 2단계: 저장소 둘러보기

저장소 안의 Skill을 탐색하세요:

```bash
# 공식 Anthropic Skill
skillshare install anthropics/skills

# 커뮤니티 Skill
skillshare install ComposioHQ/awesome-claude-skills
```

이렇게 하면 **discovery mode**로 진입해 저장소 안에서 사용 가능한 모든 Skill을
보여줍니다.

---

## 3단계: 평가

설치하기 전에 다음을 고려하세요:

- **내 문제를 해결해주는가?** 설명을 읽어보세요
- **잘 관리되고 있는가?** 저장소의 활동을 확인하세요
- **충돌이 있는가?** 기존 Skill과 이름이 겹치는지 확인하세요

무엇이 설치될지 미리보기:
```bash
skillshare install anthropics/skills/skills/pdf --dry-run
```

---

## 4단계: 설치

### 단일 Skill

```bash
skillshare install anthropics/skills/skills/pdf
```

### 한 저장소에서 여러 Skill

```bash
# 대화형 탐색
skillshare install anthropics/skills

# 특정 Skill 선택(비대화형)
skillshare install anthropics/skills -s pdf,commit

# 모든 Skill 설치
skillshare install anthropics/skills --all
```

### 저장소 전체(팀용)

```bash
skillshare install github.com/team/skills --track
```

---

## 5단계: 동기화

설치 후 동기화를 잊지 마세요:

```bash
skillshare sync
```

---

## 인기 있는 Skill 소스

| 소스 | URL |
|--------|-----|
| Anthropic 공식 | `anthropics/skills` |
| Vercel Agent Skills | `vercel-labs/agent-skills` |
| 커뮤니티 | [skillsmp.com](https://skillsmp.com/) |

---

## Discovery 명령어

| 명령어 | 용도 |
|---------|-------|
| `search` | 인기 있는 Skill 둘러보기 |
| `search <query>` | Skill 검색 |
| `check` | 사용 가능한 업데이트 확인 |
| `install <repo>` | 저장소 둘러보기(discovery mode) |
| `install <repo/path>` | 특정 Skill 설치 |
| `list` | 설치된 Skill 표시 |

---

## 설치 옵션

```bash
# 커스텀 이름
skillshare install anthropics/skills/skills/pdf --name my-pdf

# 강제 덮어쓰기
skillshare install anthropics/skills/skills/pdf --force

# 기존 항목 업데이트
skillshare install anthropics/skills/skills/pdf --update

# 팀 공유를 위한 추적
skillshare install github.com/team/skills --track
```

`--name`은 설치 Target이 단일 Skill일 때만 유효합니다.  
여러 Skill을 반환하는 저장소 discovery와 함께 `--name`을 사용하면 오류가 발생합니다.

---

## 설치 후

### 확인

```bash
skillshare list
skillshare status
```

### 테스트

AI CLI에서 Skill을 사용해 예상대로 작동하는지 확인하세요.

### 업데이트 확인

```bash
skillshare check              # 사용 가능한 업데이트 확인
```

### 나중에 업데이트하기

```bash
# 단일 Skill(소스 메타데이터 포함)
skillshare install pdf --update

# 추적되는 저장소
skillshare update _team-skills
```

---

## 참고

- [search](/docs/reference/commands/search) — search 명령어 레퍼런스
- [install](/docs/reference/commands/install) — install 명령어 레퍼런스
- [Hub Index](/docs/how-to/sharing/hub-index) — Skill Hub 관리
- [Daily Workflow](./daily-workflow.md) — 설치 후 일상적으로 사용하기
