---
sidebar_position: 5
---

# Sync Modes 상세 설명

> merge, copy, symlink라는 세 가지 sync mode에 대한 심층 분석 — 각각 언제 사용하는지, 그리고 트레이드오프.

## 세 가지 Mode

skillshare는 source 디렉터리에서 AI 도구의 target 디렉터리로 skill이 전달되는 방식을 제어하는 세 가지 sync mode를 제공합니다.

### Merge Mode (기본값)

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

**동작 방식**: skill마다 하나씩 symlink를 생성합니다. target의 각 skill 디렉터리는 source를 다시 가리킵니다.

**핵심 속성**: **비파괴적**. target 디렉터리에 있는 로컬 skill(위의 `my-local-skill`처럼)이 보존됩니다. skillshare는 자신이 만든 symlink만 관리합니다.

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

**동작 방식**: 각 skill을 target에 물리적으로 복사합니다. `.skillshare-manifest.json` 파일이 관리 중인 skill과 그들의 SHA-256 체크섬을 추적합니다. 이후 sync에서는 변경된 skill만 다시 복사됩니다.

**핵심 속성**: **최대 호환성**. symlink 지원이 필요 없이 어디서든 작동합니다. merge mode와 마찬가지로 로컬 skill이 보존됩니다.

### Symlink Mode

```
Source: ~/.config/skillshare/skills/
├── code-review/SKILL.md
├── testing/SKILL.md
└── debugging/SKILL.md

Target: ~/.claude/skills → ~/.config/skillshare/skills/  (single symlink)
```

**동작 방식**: 전체 target 디렉터리를 source를 가리키는 단일 symlink로 대체합니다.

**핵심 속성**: **완전한 제어**. target은 정확히 source와 같습니다. target에는 어떠한 로컬 skill도 존재할 수 없습니다.

## 각각을 언제 사용하나요

| 요인 | Merge | Copy | Symlink |
|--------|-------|------|---------|
| 로컬 skill 보존 | 예 | 예 | 아니오 |
| 크로스 플랫폼 지원 | 문제가 있을 수 있음 | 어디서나 작동함 | 문제가 있을 수 있음 |
| Source 변경 반영 | 즉시 | `sync` 이후 | 즉시 |
| 중첩 경로 처리 | 평탄화 (`a/b/c` → `a__b__c`) | 평탄화 | 네이티브 구조 |
| Orphan 정리 | 자동 | 자동 | 필요 없음 |
| 디스크 사용량 | 최소 (symlink) | 전체 사본 | 최소 (심볼릭 하나) |
| 권장 대상 | 대부분의 사용자 | WSL, Docker, CI | 단일 source 설정 |

### Merge를 선택할 때

- skillshare가 관리하지 않기를 원하는 로컬 skill이 AI 도구에 있을 때
- 서로 다른 로컬 커스터마이즈가 있는 여러 AI 도구를 사용할 때
- skillshare를 점진적으로 도입하는 중일 때 (일부 skill은 관리됨, 일부는 관리되지 않음)

### Copy를 선택할 때

- 플랫폼의 symlink 지원이 신뢰할 수 없을 때 (WSL, 일부 Docker 설정)
- AI 도구가 symlink를 제대로 따라가지 못할 때
- CI/CD 파이프라인이나 컨테이너 환경에 있을 때
- target이 source 디렉터리와 무관하게 독립적으로 작동하기를 원할 때

### Symlink를 선택할 때

- skillshare가 target의 유일한 skill 소스일 때
- target에 무엇이 있는지에 대해 전혀 모호함이 없기를 원할 때
- 새 환경을 처음부터 설정하는 중일 때

## 중첩 경로 처리

merge와 copy mode에서는 중첩된 source 경로가 이중 언더스코어로 평탄화됩니다:

```
Source: skills/frontend/react-patterns/SKILL.md
Target: ~/.claude/skills/frontend__react-patterns → skills/frontend/react-patterns
```

이는 평평한 skill 구조를 기대하는 target에서 디렉터리 생성을 피합니다. symlink mode에서는 디렉터리 구조가 그대로 보존됩니다.

## Orphan 정리

Merge와 copy mode는 `skillshare sync` 중 orphan 항목을 자동으로 제거합니다. source에서 skill을 제거하면, target의 해당 symlink(또는 복사된 디렉터리)가 다음 sync에서 정리됩니다.

```bash
skillshare uninstall old-skill
skillshare sync
# → Pruned orphan: old-skill
```

## Target별 Mode 재정의

target마다 다른 mode를 설정할 수 있습니다. Global config는 map 형식을 사용합니다:

```yaml
targets:
  claude:
    path: ~/.claude/skills
    mode: merge
  cursor:
    path: ~/.cursor/skills
    mode: copy
```

Project config는 list 형식을 사용합니다:

```yaml
targets:
  - name: claude
    mode: merge
  - name: cursor
    mode: copy
```

또는 CLI를 통해 mode를 변경하세요:

```bash
skillshare target claude --mode copy
```

## 관련 문서

- [Sync modes 개념 페이지](/docs/understand/sync-modes)
- [`sync` 명령 참조](/docs/reference/commands/sync)
- [Source and targets](/docs/understand/source-and-targets)
