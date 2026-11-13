---
sidebar_position: 10
---

# 중앙집중형 vs 로컬 우선

Skill 관리 도구는 대체로 두 가지 아키텍처 접근 방식 중 하나를 따릅니다. **중앙집중형 플랫폼** 또는 **로컬 우선(local-first)**입니다. 어느 쪽이 보편적으로 더 낫다고 할 수는 없습니다 — 각각 trade-off가 있습니다. 이 페이지는 두 방식을 모두 살펴보고 자신의 워크플로에 맞는 것을 고를 수 있도록 돕습니다.

:::tip 이것은 기능 비교가 아닙니다
기능 수준의 차이(install 흐름, config 형식 등)는 [Skill 관리 방식 비교](/docs/understand/philosophy/comparison)를 참고하세요. 이 페이지는 **아키텍처적 trade-off** — 데이터가 어디에 저장되는지, discovery가 어떻게 작동하는지, 무엇을 통제할 수 있는지에 초점을 맞춥니다.
:::

## 두 가지 접근 방식

### 중앙집중형 플랫폼

중앙집중형 플랫폼은 skill이 게시, 검색, 순위가 매겨지는 통합 레지스트리를 호스팅합니다. install 활동은 다운로드 수, 트렌딩 순위 같은 커뮤니티 지표로 집계됩니다.

**강점**:
- 내장된 discovery — 한 곳에서 skill을 탐색, 검색, 비교 가능
- 커뮤니티 신호 — 다운로드 수와 트렌딩이 인기 있는 skill을 드러내는 데 도움
- 낮은 진입장벽 — discovery를 위한 별도 설정 불필요, 검색하고 install만 하면 됨

**고려 사항**:
- install 활동이 플랫폼에 의해 추적됨
- 순위 및 집계 규칙이 플랫폼 운영자에 의해 관리됨

### 로컬 우선 (skillshare)

skillshare는 모든 상태를 사용자의 머신에 보관합니다. Skill은 `git clone`을 통해 install되고 로컬 config 파일로 관리됩니다. 원격 서버로 전송되는 것은 아무것도 없습니다.

**강점**:
- 텔레메트리 제로 — install 추적 없음, 어디로도 데이터 전송 없음
- 완전한 소유권 — skill이 자신의 파일시스템에 존재
- 초기 install 후 오프라인에서도 작동
- 단일 바이너리, 런타임 의존성 없음

**고려 사항**:
- 내장된 커뮤니티 지표(다운로드 수, 트렌딩) 없음
- discovery를 위해 hub를 설정하거나 연결해야 함

## Discovery

로컬 우선이라고 discovery가 없다는 뜻은 아닙니다. skillshare는 세 가지 discovery 채널을 제공합니다.

| 채널 | 작동 방식 |
|---------|-------------|
| **GitHub 검색** | `skillshare search <query>` — 공개 GitHub repo를 직접 검색 |
| **공개 Hub** | `skillshare search --hub` — 내장된 [커뮤니티 hub](https://github.com/runkids/skillshare-hub) 조회 |
| **커스텀 Hub** | `skillshare search --hub <url>` — 당신이나 조직이 유지 관리하는 모든 hub 조회 |

### Hub란 무엇인가요?

Hub는 이름, 설명, source, 태그와 함께 skill을 나열하는 정적 JSON 파일(`skillshare-hub.json`)입니다. Git repo, HTTP 서버, 로컬 파일시스템 등 어디에나 존재할 수 있습니다.

```bash
# 설치된 skill로부터 인덱스 빌드
skillshare hub index

# 조직의 내부 hub 검색
skillshare search --hub https://internal.corp/skills/hub.json

# 로컬 인덱스 파일 검색
skillshare search --hub ./skillshare-hub.json
```

Hub는 독립적입니다 — 누구나 만들 수 있고, 사용자는 여러 hub에 동시에 연결할 수 있습니다. 이는 공개 hub와 함께 비공개 skill 카탈로그를 유지해야 하는 조직에 적합합니다.

자세한 안내는 [Hub Index 가이드](/docs/how-to/sharing/hub-index)를 참고하세요.

### 자체 호스팅 지표

skillshare 자체는 install을 추적하지 않지만, 자체 서버에서 hub를 호스팅한다면 원하는 어떤 분석 계층이든 추가할 수 있습니다.

1. 서버에서 `skillshare-hub.json` 호스팅
2. 요청 로깅이나 경량 분석 엔드포인트 추가
3. 검색 히트, install referral, 원하는 모든 지표 추적

이를 통해 skill 작성자나 조직은 자신만의 기준으로 채택률을 측정할 수 있습니다.

## 올바른 접근 방식 선택하기

**중앙집중형 플랫폼이 더 나은 선택일 수 있습니다:**
- 기본으로 내장된 커뮤니티 지표와 트렌딩을 원하는 경우
- skill 탐색을 위한 단일 브라우징 목적지를 선호하는 경우
- AI CLI를 하나만 사용하고 도구 간 sync가 필요 없는 경우

**로컬 우선이 더 나은 선택일 수 있습니다:**
- 여러 AI CLI를 사용하며 통합 관리를 원하는 경우
- install 활동이 자신의 머신에만 머무르길 선호하는 경우
- 오프라인 작동이 필요하거나 제한된 네트워크 환경에서 일하는 경우
- 어떤 skill을 사용 가능하고 탐색 가능하게 할지 통제해야 하는 조직인 경우

---

## 참고

- [Skill 관리 방식 비교](/docs/understand/philosophy/comparison) — 기능 수준 비교
- [Hub Index 가이드](/docs/how-to/sharing/hub-index) — skill hub 구축 및 사용
- [hub 명령어](/docs/reference/commands/hub) — hub 명령어 레퍼런스
- [보안 가이드](./security.md) — skill 보안 스캐닝
