---
sidebar_position: 3
---

# Adding Custom Targets

skill 디렉터리를 가진 어떤 도구든 skillshare에 추가할 수 있습니다.

## Overview

사용 중인 AI CLI가 [지원 목록](./supported-targets.md)에 없다면, 수동으로 추가할 수 있습니다.

---

## Add a Target

```bash
skillshare target add <name> <path>
```

### Example

```bash
skillshare target add aider ~/.aider/skills
skillshare sync
```

---

## Requirements

### Path must exist

필요하다면 먼저 디렉터리를 생성하세요.

```bash
mkdir -p ~/.myapp/skills
skillshare target add myapp ~/.myapp/skills
```

### Path should end with `/skills`

권장 사항이지만 필수는 아닙니다.

```bash
# 권장
skillshare target add myapp ~/.myapp/skills

# 이것도 동작함
skillshare target add myapp ~/.myapp/prompts
```

---

## Verify

추가한 뒤:

```bash
# Target 확인
skillshare target myapp

# 새 Target으로 동기화
skillshare sync

# 확인
skillshare status
```

---

## Common Scenarios

### Add new AI CLI tool

```bash
# 1. 도구가 skill을 저장하는 위치 찾기
# (도구 문서 확인)

# 2. 필요하면 디렉터리 생성
mkdir -p ~/.newtool/skills

# 3. Target으로 추가
skillshare target add newtool ~/.newtool/skills

# 4. 동기화
skillshare sync
```

### Add project-specific target

```bash
# 특정 프로젝트로 skill 동기화
skillshare target add myproject ~/projects/myapp/.ai/skills
skillshare sync
```

### Add multiple tools

```bash
skillshare target add tool1 ~/.tool1/skills
skillshare target add tool2 ~/.tool2/skills
skillshare target add tool3 ~/.tool3/skills
skillshare sync
```

---

## Change Sync Mode

추가한 뒤 Sync 모드를 변경할 수 있습니다.

```bash
# 기본값은 merge 모드
skillshare target myapp --mode symlink
skillshare sync
```

자세한 내용은 [Sync Modes](/docs/understand/sync-modes)를 참고하세요.

---

## Remove Target

더 이상 필요 없는 Target이 있다면:

```bash
skillshare target remove myapp
```

이 명령은 다음을 수행합니다.
1. 백업 생성
2. symlink를 실제 파일로 교체 (merge 모드에서는 Source가 관리하는 symlink만 제거되며, 로컬 skill은 보존됨)
3. config에서 제거

---

## Troubleshooting

### "path does not exist"

먼저 디렉터리를 생성하세요.

```bash
mkdir -p ~/.myapp/skills
skillshare target add myapp ~/.myapp/skills
```

### Target not syncing

Target이 활성화되어 있는지 확인하세요.

```bash
skillshare target list
skillshare target myapp
```

### Wrong path

제거 후 다시 추가하세요.

```bash
skillshare target remove myapp
skillshare target add myapp /correct/path/skills
```

---

## Related

- [Supported Targets](./supported-targets.md) — 기본 제공 Target
- [Configuration](./configuration.md) — config 직접 편집
- [Sync Modes](/docs/understand/sync-modes) — Merge, copy, symlink
