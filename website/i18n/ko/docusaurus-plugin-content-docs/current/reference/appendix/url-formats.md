---
sidebar_position: 4
---

# URL 형식

`skillshare install`이 인식하는 모든 Source URL 패턴입니다.

## 빠른 참조

| 형식 | 예시 | 비고 |
|--------|---------|-------|
| GitHub 단축형 | `owner/repo` | `github.com/owner/repo`로 확장됨 |
| 하위 디렉터리를 포함한 GitHub | `owner/repo/path/to/skill` | 저장소에서 특정 Skill 설치 |
| 전체 HTTPS | `https://github.com/owner/repo` | 모든 Git 호스트 |
| 하위 디렉터리를 포함한 전체 HTTPS | `https://github.com/owner/repo/path` | host/owner/repo 뒤에 하위 디렉터리 |
| SSH | `git@github.com:owner/repo.git` | SSH 키를 통한 비공개 저장소 |
| 하위 디렉터리를 포함한 SSH | `git@github.com:owner/repo.git//path` | `//`로 저장소와 하위 디렉터리 구분 |
| GHE Cloud | `mycompany.github.com/org/repo` | Enterprise Cloud 서브도메인 |
| GHE Server | `github.mycompany.com/org/repo` | Enterprise Server |
| Azure DevOps 단축형 | `ado:org/project/repo` | `dev.azure.com` URL로 확장됨 |
| Azure DevOps HTTPS | `https://dev.azure.com/org/proj/_git/repo` | 최신 형식 |
| Azure DevOps SSH | `git@ssh.dev.azure.com:v3/org/proj/repo` | SSH v3 형식 |
| Azure DevOps Server | `https://custom-host/org/proj/_git/repo` | `azure_hosts` 설정 필요 |
| 로컬 경로 | `~/my-skill`, `/abs/path`, 또는 `C:\path` | 디렉터리를 Source에 복사 |
| Git 파일 URL | `file:///path/to/repo` | 로컬 git clone (테스트용) |

## GitHub 단축형

가장 간단한 형식 — `owner/repo`만 사용:

```bash
skillshare install anthropics/skills
skillshare install ComposioHQ/awesome-claude-skills
```

내부적으로 `https://github.com/owner/repo`로 확장됩니다.

### 하위 디렉터리 포함

`owner/repo` 뒤에 경로를 추가하여 특정 Skill을 설치할 수 있습니다:

```bash
skillshare install anthropics/skills/skills/pdf
skillshare install anthropics/skills/skills/commit
```

하위 디렉터리가 정확히 일치하지 않으면, skillshare는 해당 basename을 가진 Skill을 찾기 위해 저장소를 스캔합니다:

```bash
# 루트에는 "pdf"가 없지만 skills/pdf/에서 발견됨 — 자동으로 해결됨
skillshare install anthropics/skills/pdf
```

## 전체 HTTPS URL

모든 Git 호스트에서 작동합니다:

```bash
# GitHub
skillshare install https://github.com/owner/repo

# GitLab
skillshare install https://gitlab.com/owner/repo

# Bitbucket
skillshare install https://bitbucket.org/owner/repo

# 자체 호스팅 Gitea
skillshare install https://git.mycompany.com/team/skills

# AtomGit (중국)
skillshare install https://atomgit.com/owner/repo

# Gitee (중국)
skillshare install https://gitee.com/owner/repo
```

## SSH URL

비공개 저장소에는 SSH를 사용하세요:

```bash
# 표준 SSH
skillshare install git@github.com:owner/repo.git

# 하위 디렉터리 포함 (// 구분자에 주의)
skillshare install git@github.com:owner/repo.git//path/to/skill

# GitLab SSH
skillshare install git@gitlab.com:owner/repo.git
```

:::info `//` 구분자
SSH URL에서는 저장소와 하위 디렉터리 경로를 구분하기 위해 `//`를 사용합니다. SSH URL에서 `:`가 이미 구분자로 사용되므로 표준 `/` 경로 규칙은 모호해질 수 있기 때문입니다.
:::

## GitHub Enterprise

Enterprise 호스트명은 자동으로 인식됩니다:

```bash
# Enterprise Cloud (서브도메인 패턴: *.github.com)
skillshare install mycompany.github.com/org/repo

# Enterprise Server (호스트명 패턴: github.*.*)
skillshare install github.mycompany.com/org/repo
skillshare install github.internal.corp/team/skills
```

두 패턴 모두 하위 디렉터리 경로를 지원합니다:

```bash
skillshare install github.mycompany.com/org/repo/path/to/skill
```

## Azure DevOps

### 단축형

`ado:` 접두사는 Azure DevOps URL로 확장됩니다:

```bash
skillshare install ado:myorg/myproject/myrepo
skillshare install ado:myorg/myproject/myrepo/skills/react
```

### 전체 URL

```bash
# 최신 형식
skillshare install https://dev.azure.com/myorg/myproject/_git/myrepo

# 레거시 형식 (dev.azure.com으로 자동 정규화됨)
skillshare install https://myorg.visualstudio.com/myproject/_git/myrepo

# SSH
skillshare install git@ssh.dev.azure.com:v3/myorg/myproject/myrepo
```

## 로컬 경로

파일 시스템의 디렉터리에서 설치합니다:

```bash
# 절대 경로
skillshare install /home/user/my-skill

# 홈 디렉터리 단축형
skillshare install ~/my-skill

# 상대 경로
skillshare install ./local-skill

# Windows 드라이브 문자 경로
skillshare install D:\skills\my-skill
```

로컬 설치는 파일을 (심볼릭 링크가 아니라) **복사**하며, `skillshare update`로 업데이트할 수 없습니다.

## 인증

### SSH 키 (비공개 저장소 권장)

```bash
# SSH 키가 로드되어 있는지 확인
ssh-add ~/.ssh/id_ed25519

# SSH로 설치
skillshare install git@github.com:company/private-skills.git
```

### 토큰을 사용한 HTTPS

HTTPS URL의 경우, git은 설정된 자격 증명 헬퍼를 사용합니다:

```bash
# git 자격 증명 헬퍼 설정 (한 번만)
git config --global credential.helper store

# 또는 GitHub용 GH CLI 사용
gh auth login

# 그런 다음 평소처럼 설치
skillshare install https://github.com/company/private-repo
```

### PAT를 사용한 Azure DevOps

Azure DevOps 저장소는 HTTPS 인증에 [개인 액세스 토큰(PAT)](https://learn.microsoft.com/en-us/azure/devops/organizations/accounts/use-personal-access-tokens-to-authenticate?view=azure-devops)을 사용합니다:

```bash
export AZURE_DEVOPS_TOKEN=your_pat_here
skillshare install https://dev.azure.com/org/project/_git/repo
```

또는 SSH를 사용하세요 (토큰 불필요):

```bash
skillshare install git@ssh.dev.azure.com:v3/org/project/repo
```

:::tip 비공개 저장소
HTTPS에서 인증 오류가 발생하면 SSH URL로 전환하세요. skillshare는 자격 증명 프롬프트가 멈추는 것을 방지하기 위해 `GIT_TERMINAL_PROMPT=0`을 설정하므로, 대화형 HTTPS 인증은 작동하지 않습니다.
:::

## 커스텀 GitLab 도메인 {#custom-gitlab-domains}

이름에 `gitlab` 또는 `jihulab`이 포함된 호스트(예: `gitlab.com`, `jihulab.com`, `onprem.gitlab.internal`)는 자동으로 감지되어 중첩된 하위 그룹을 지원하는 방식으로 파싱됩니다.

커스텀 도메인의 자체 관리형 GitLab 인스턴스(예: `git.company.com`)의 경우, 설정 파일의 [`gitlab_hosts`](../targets/configuration.md#gitlab_hosts)에 호스트명을 추가하세요:

```yaml
gitlab_hosts:
  - git.company.com
```

이렇게 하면 skillshare가 GitLab의 중첩 하위 그룹 동작에 맞춰 전체 URL 경로를 저장소로 취급합니다.

**설정이 없는 경우**, `.git`을 사용하여 저장소 경로의 끝을 표시할 수 있습니다:

```bash
# git.company.com/team/frontend/ui에서 설치됨 (전체 경로를 저장소로 취급)
skillshare install git.company.com/team/frontend/ui.git
```


## Gitea와 CNB {#gitea-and-cnb}

`gitea.com`, 이름에 `gitea`가 포함된 모든 호스트, 그리고 `cnb.cool`이 인식됩니다. 경로는 `owner/repo`로 읽히며, 그 이후는 하위 디렉터리로 처리됩니다:

```bash
skillshare install https://gitea.com/owner/repo/skills/review
skillshare install https://cnb.cool/org/repo/skills
```

다른 도메인의 자체 호스팅 인스턴스는 [`gitea_hosts`](../targets/configuration.md#gitea_hosts) 또는 [`cnb_hosts`](../targets/configuration.md#cnb_hosts)에 호스트명을 등록하세요. 비공개 저장소는 [`GITEA_TOKEN`](./environment-variables.md#gitea_token)과 [`CNB_TOKEN`](./environment-variables.md#cnb_token)을 사용합니다.

## 커스텀 Azure DevOps 도메인 {#custom-azure-domains}

내장 Azure DevOps 패턴은 `dev.azure.com`과 `*.visualstudio.com`을 자동으로 매칭합니다.

커스텀 도메인의 자체 호스팅 Azure DevOps Server 인스턴스의 경우, 설정 파일의 [`azure_hosts`](../targets/configuration.md#azure_hosts)에 호스트명을 추가하세요:

```yaml
azure_hosts:
  - azuredevops.mycompany.com
```

이렇게 하면 skillshare는 해당 호스트에서 `/_git/`을 포함한 URL을 Azure DevOps 파싱 경로로 라우팅하며, `.git`을 추가하지 않고도 clone URL을 올바르게 구성합니다.

## 플랫폼 지원

| 기능 | GitHub | GitLab | Bitbucket | Gitea | GHE | Azure DevOps | AtomGit/Gitee |
|---------|--------|--------|-----------|-------|-----|--------------|---------------|
| 단축형 (`owner/repo`) | Yes | No | No | No | Yes | `ado:` 접두사 | No |
| 전체 HTTPS URL | Yes | Yes | Yes | Yes | Yes | Yes | Yes |
| SSH URL | Yes | Yes | Yes | Yes | Yes | Yes | Yes |
| 하위 디렉터리 | Yes | Yes | Yes | Yes | Yes | Yes | Yes |
| `skillshare search` | Yes | No | No | No | No | No | No |

## 관련 문서

- [Install command](/docs/reference/commands/install) — 전체 install 옵션 및 예시
