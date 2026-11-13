---
sidebar_position: 4
---

# Recipe: Project Mode Workflow

> 코드베이스와 함께 이동하는 Project 범위의 Skill을 관리합니다.

## 시나리오

특정 Skill을 프로젝트 저장소에 커밋하여 다음을 달성하고 싶습니다:
- 모든 기여자가 동일한 AI 지침을 받습니다
- Skill이 코드와 함께 버전 관리됩니다
- 저장소를 클론하는 것 외에 수동 설정이 필요 없습니다

## 해결 방법

### 1단계: Project mode 초기화

```bash
cd your-project
skillshare init -p
```

이 명령은 프로젝트 루트에 `.skillshare/config.yaml`을 생성합니다.

### 2단계: Project 범위 Skill 설치

```bash
skillshare install anthropics/courses/prompt-eng -p
skillshare install your-org/team-skills --skill code-review -p
```

Skill은 `.skillshare/skills/`에 배치됩니다.

### 3단계: Project Target에 Sync

```bash
skillshare sync -p
```

이 명령은 `.skillshare/skills/`에서 프로젝트 레벨 Target 디렉터리(예: `.claude/skills/`, `.cursor/skills/`)로 심볼릭 링크를 생성합니다.

### 4단계: 버전 관리에 커밋

```bash
git add .skillshare/
git commit -m "Add project skills"
```

### 5단계: 팀원 설정

팀원이 저장소를 클론하면:

```bash
git clone your-org/your-project
cd your-project
skillshare sync -p
```

명령어 하나로 모든 Project Skill이 로컬 AI 도구에 동기화됩니다.

## 확인

- `.skillshare/config.yaml`이 프로젝트 루트에 존재합니다
- `.skillshare/skills/`에 설치된 Skill이 들어 있습니다
- `skillshare list -p`가 Project Skill을 표시합니다
- `sync -p` 이후 Target 디렉터리에 심볼릭 링크가 있습니다

## 변형

- **Dev container 자동 Sync**: `.devcontainer/devcontainer.json`의 `postCreateCommand`에 `skillshare sync -p`를 추가하세요
- **혼합 모드**: 개인 취향에는 Global Skill을, 팀 표준에는 Project Skill을 사용하세요
- **CI 검증**: Project Skill을 검증하기 위해 CI 파이프라인에 `skillshare audit -p`를 추가하세요

## 관련 문서

- [Project 설정 가이드](/docs/how-to/sharing/project-setup)
- [Project Skill 이해하기](/docs/understand/project-skills)
- [Dev container 가이드](/docs/learn/with-devcontainer)
