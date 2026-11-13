---
sidebar_position: 3
---

# Recipe: Pre-commit Hook

> [pre-commit](https://pre-commit.com/) 프레임워크를 사용해 매 커밋마다 `skillshare audit`를 자동으로 실행합니다.

## 언제 사용하나요

Pre-commit hook은 다음과 같은 경우에 가장 유용합니다:

- **여러 기여자가 Skill을 편집하는 경우** — 팀원이 실수로 위험한 명령어(`curl | bash`, `sudo rm -rf`)를 추가할 수 있습니다. Hook은 이런 명령어가 버전 관리에 들어가기 전에 잡아냅니다.
- **Skill이 외부 소스에서 오는 경우** — GitHub, 커뮤니티 저장소, AI가 생성한 콘텐츠에서 Skill을 복사하면 수동 검토가 어렵습니다. 자동화된 스캔이 안전망을 제공합니다.
- **즉각적인 피드백을 원하는 경우** — CI도 문제를 잡아내지만 push 이후에만 가능합니다. Hook은 개발자에게 몇 초 안에 즉각적인 로컬 피드백을 제공합니다.

다음과 같은 경우에는 건너뛸 수 있습니다:

- 본인이 유일한 작성자이고 모든 Skill을 신뢰하는 경우
- Skill이 거의 변경되지 않는 경우 (Hook은 `.skillshare/` 또는 `skills/` 파일이 수정될 때만 실행됩니다)

## 설정

프로젝트의 `.pre-commit-config.yaml`에 추가하세요:

```yaml
repos:
  - repo: https://github.com/runkids/skillshare
    rev: v0.16.8  # 최신 릴리스 태그를 사용하세요
    hooks:
      - id: skillshare-audit
```

그런 다음 Hook을 설치하세요:

```bash
pre-commit install
```

## 동작 방식

이 Hook은 `.skillshare/` 또는 `skills/` 디렉터리와 일치하는 파일에 변경 사항을 커밋할 때마다 `skillshare audit -p`를 실행합니다. 발견된 항목이 설정된 임계값을 초과하면 커밋이 차단됩니다.

## 설정 (Configuration)

이 Hook은 프로젝트의 `.skillshare/config.yaml` 설정을 따릅니다:

```yaml
audit:
  block_threshold: high  # HIGH 이상의 발견 항목에서 차단
```

## Hook 건너뛰기

일회성으로 건너뛰려면:

```bash
SKIP=skillshare-audit git commit -m "your message"
```

## 요구 사항

- `skillshare` CLI가 설치되어 있고 `PATH`에서 사용 가능해야 합니다
- 프로젝트는 `skillshare init -p`로 초기화되어 있어야 합니다

## CI와 함께 사용하기

Pre-commit hook은 로컬에서 문제를 잡아내며, [CI/CD 검증](ci-cd-skill-validation.md)은 팀 전체를 위한 안전망을 제공합니다. 심층 방어를 위해 둘 다 사용하세요.
