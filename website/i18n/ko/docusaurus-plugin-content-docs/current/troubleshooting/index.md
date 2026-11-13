---
sidebar_position: 1
---

# Troubleshooting

문제가 있으신가요? 여기서 시작하세요.

## Quick Diagnosis

doctor 명령어를 실행하세요.

```bash
skillshare doctor
```

이 명령어는 다음을 확인합니다.
- Source 디렉터리
- Config 파일
- Target 접근 가능성
- Symlink 상태
- Git 상태

---

## What's happening?

| Problem | Go To |
|---------|-------|
| 오류 메시지가 보임 | [Common Errors](./common-errors.md) |
| Windows에서 뭔가 동작하지 않음 | [Windows](./windows.md) |
| 단계별 디버깅 과정이 필요함 | [Troubleshooting Workflow](./troubleshooting-workflow.md) |
| 일반적인 질문이 있음 | [FAQ](./faq.md) |

---

## Quick Fixes

### Skills not appearing

```bash
skillshare sync
```

### Broken symlinks

```bash
skillshare sync --force
```

### Config issues

```bash
skillshare doctor
```

### Start fresh

```bash
rm ~/.config/skillshare/config.yaml
skillshare init
```

---

## Getting Help

문제를 해결할 수 없다면:

1. **정보 수집:**
   ```bash
   skillshare doctor
   skillshare status
   ```

2. **기존 이슈 검색:** [GitHub Issues](https://github.com/runkids/skillshare/issues)

3. **새 이슈 등록**, 다음을 포함하여:
   - doctor 출력
   - 오류 메시지
   - 하려고 했던 작업
   - 운영체제

---

## Related

- [Troubleshooting Workflow](./troubleshooting-workflow.md) — 단계별 디버깅
- [Commands: doctor](/docs/reference/commands/doctor) — doctor 명령어 세부 사항
