# Quant Port

국내·미국 주식을 점수·뉴스·실적으로 한눈에 보는 투자 정보 서비스 — <https://www.quant-port.com>

Go API + Next.js 프론트엔드 모노레포다. 운영은 Docker Compose 한 스택(api · 워커 · fe · nginx · redis)이고, MySQL은 호스트에서 돈다.

## 구조

```
quant-port/
├── be/                 Go API · 배치 워커 (chi, MySQL, Redis)
│   ├── cmd/            진입점 — api, worker, 일회성 백필·시드 도구
│   ├── internal/       handler → service → repository, 외부 연동(toss·llm·marketdata·translate), workers/
│   ├── migrations/     번호순 SQL (기동 시 *IfNeeded 함수가 적용)
│   ├── data/           종목 시드·DART 코드 캐시
│   ├── sentiment/      헤드라인 감성 분류기 (Python 사이드카)
│   └── testdata/       OCR 가져오기 시험 이미지
├── fe/                 Next.js (App Router) + TypeScript + Tailwind v4
│   └── src/            app/ · components/ · hooks/ · lib/
├── e2e/                Playwright — API 계약·브라우저 시나리오
├── deploy/             nginx 설정, 역할별 compose, 워치독 스크립트
├── docs/               설계 문서·릴리스 노트·시안(mockups)
└── docker-compose.yml  로컬/운영 스택 정의
```

## 로컬 실행

필요한 것: Docker, MySQL 8 (호스트), Go 1.25, Node 20.

```bash
cp be/.env.example be/.env          # 값 채우기 — be/.env 는 git에 들어가지 않는다
mysql -u root -p < be/scripts/setup-local-mysql.sql
docker compose up -d --build redis api
```

```bash
cd fe && npm ci && npm run dev      # http://localhost:3000, API는 NEXT_PUBLIC_API_URL
```

상세: [be/README.md](be/README.md) · [fe/README.md](fe/README.md) · [deploy/README.md](deploy/README.md) · [be/docs/docker.md](be/docs/docker.md)

## 검사

```bash
cd be && go build ./... && go vet ./... && go test ./...
```

```bash
cd fe && npx tsc --noEmit && npx vitest run
```

E2E는 [e2e/README.md](e2e/README.md). CI(`.github/workflows/ci.yml`)가 위 두 묶음을 푸시마다 돌린다.

## 문서

- [docs/PRINCIPLES.md](docs/PRINCIPLES.md) — 저장·캐시·시세·LLM 지출·목록 API 원칙
- [be/docs/BATCH_SCHEDULE.md](be/docs/BATCH_SCHEDULE.md) — 배치 잡 일정
- [docs/STOCK_SCORE_DESIGN.md](docs/STOCK_SCORE_DESIGN.md) — 종목 점수 설계
- [docs/DATA_SOURCES_LICENSE.md](docs/DATA_SOURCES_LICENSE.md) — 데이터 원천과 라이선스
- [docs/releases/](docs/releases/) — 릴리스 노트
- [fe/DESIGN.md](fe/DESIGN.md) — 디자인 시스템
- `AGENTS.md` · `docs/AGENT_STATE.md` — 코딩 에이전트용 작업 규칙과 현재 상태

## 비밀값

API 키·DB 비밀번호·서비스 계정 JSON은 저장소에 없다(`be/.env`, `be/secrets/`, `deploy/*.env`, `deploy/watchdog.config.json`은 `.gitignore`). 예시 파일(`*.example`)의 값은 자리표시자다. 보안 문제는 공개 이슈 대신 저장소 소유자에게 직접 알려 주기 바란다.
