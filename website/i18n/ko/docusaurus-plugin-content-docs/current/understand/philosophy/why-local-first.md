---
sidebar_position: 1
---

# 왜 Local-First인가

> skillshare는 런타임 의존성이 없는 단일 바이너리입니다. 그 이유는 다음과 같습니다.

## 결정

skillshare는 단일 Go 바이너리로 배포됩니다. Node.js도, Python도, 패키지 매니저도, 데몬도 없습니다. 설치하고, 실행하면, 끝입니다.

이는 최소 저항의 경로가 아니었습니다 — 세 가지 원칙에 의해 이루어진 의도적인 선택이었습니다.

## 원칙 1: 의존성 체인 제로

모든 의존성은 공격 표면이자 유지보수 부담입니다.

skillshare가 Node.js를 필요로 했다면, Node 버전을 관리하고, `node_modules`를 다뤄야 하며, 플랫폼별 네이티브 모듈을 처리하고, npm 공급망 전체를 신뢰해야 했을 것입니다. 그 자체로 신뢰할 수 없는 콘텐츠인 AI skill을 관리하는 도구에게, 신뢰할 수 없는 의존성 체인을 추가하는 것은 용납할 수 없습니다.

Go는 정적 바이너리로 컴파일됩니다. 의존성 체인은 컴파일 시점에서 끝납니다. 다운로드한 것이 곧 실행하는 것입니다.

## 원칙 2: 어디서나 동일하게 작동함

skillshare는 다음에서 실행됩니다:
- macOS (Intel과 Apple Silicon)
- Linux (amd64와 arm64)
- Windows (amd64)
- Docker 컨테이너 (특별한 설정 없이)
- CI/CD 파이프라인 (언어 런타임 불필요)
- Dev container와 Codespaces

단일 바이너리는 모든 플랫폼에서 동일한 동작을 의미합니다. "내 머신에서는 되는데" 하는 디버깅이 없습니다. CI 환경 드리프트도 없습니다.

## 원칙 3: 기본적으로 오프라인

skillshare의 핵심 작업들 — `sync`, `list`, `status`, `backup`, `restore` — 는 네트워크 접근 없이 작동합니다. 명시적으로 원격이 필요한 작업(`install`, `search`, `check`, `update`, `push`, `pull`)만 연결이 필요합니다.

이는 다음 상황에서 중요합니다:
- **에어갭 환경**: 국방, 의료, 금융 기관은 종종 네트워크 접근을 제한합니다
- **불안정한 연결**: 기차, 비행기, 컨퍼런스 Wi-Fi
- **속도**: 로컬 작업은 초 단위가 아니라 밀리초 단위로 완료됩니다

## 왜 패키지 매니저 플러그인이 아닌가?

npm 패키지, Homebrew formula(현재는 추가 채널로 지원 중), 또는 pip 패키지로 배포하는 것도 고려했습니다. 각각 동일한 문제가 있었습니다: skillshare 사용자가 갖고 있지 않거나 원하지 않을 수 있는 런타임 의존성을 추가한다는 것입니다.

Windows에서 Cursor를 사용하는 개발자가 Homebrew를 설치할 필요는 없어야 합니다. Alpine Linux를 실행하는 CI 파이프라인이 Node.js를 필요로 해서는 안 됩니다. 도구가 사용자의 환경에 맞춰야지, 그 반대여서는 안 됩니다.

## 트레이드오프

단일 바이너리 접근 방식에는 비용이 따릅니다:

- **빌드 복잡도**: 6개 이상의 target에 대한 크로스 컴파일, CGO 비활성화
- **업데이트 메커니즘**: `npm update`가 없습니다 — skillshare에는 자체 `upgrade` 명령이 있습니다
- **UI 배포**: 웹 대시보드는 바이너리에 번들될 수 없어서(너무 큼) 런타임에 다운로드되어 캐시됩니다

우리는 사용자 경험을 단순하게 유지하기 위해 이러한 트레이드오프를 받아들입니다: 다운로드하고, 실행하면, 끝입니다.

## 관련 문서

- [Security-First Design](/docs/understand/philosophy/security-first)
- [Comparison with other tools](/docs/understand/philosophy/comparison)
