# Quant Port API (Go)

REST API(`/api/v1`)와 배치 워커. MySQL이 원본, Redis는 캐시·한도·잠금이다.

## 구조

```
be/
├── cmd/
│   ├── api/            HTTP 서버
│   ├── worker/         배치 워커 — WORKER_ROLES(쉼표 목록)로 역할을 고른다
│   └── …               일회성 백필·시드·디버그 도구
├── internal/
│   ├── server/         라우터 (router.go)
│   ├── handler/        HTTP 핸들러
│   ├── middleware/     인증·레이트 리밋·실제 IP·로깅
│   ├── service/        비즈니스 로직
│   ├── repository/     MySQL·Redis 접근, 테이블별 *IfNeeded 마이그레이션
│   ├── model/          도메인 타입
│   ├── workers/        예약 잡 (종가·실적·공시·점수·뉴스 …)
│   ├── toss/           토스증권 Open API 클라이언트 (토큰·한도 그룹·페일오버)
│   ├── marketdata/     외부 시세·뉴스·지표 클라이언트와 호출 예산(apibudget)
│   ├── llm/            LLM 요약·브리핑 프롬프트와 클라이언트
│   ├── translate/      뉴스 제목 번역 사슬
│   ├── import/         보유 종목 스크린샷 OCR 가져오기
│   ├── growth/         SEO 크롤·검색 콘솔 지표
│   └── config/         환경 변수
├── migrations/         번호순 SQL
├── docs/               배치 일정·API 한도·토스 스펙 기준본
└── pkg/response/       JSON 응답 헬퍼
```

## 실행

```bash
# 프로젝트 루트 — Redis + API (Tesseract 포함)
docker compose up -d --build
```

**Windows** — BE 재시작은 **Docker only**:

```powershell
.\scripts\restart.ps1 -Background
```

Tesseract·health 검증 포함. 로그: `docker logs -f quant-port-api`

상세: [`docs/docker.md`](docs/docker.md)

- Health: `GET http://localhost:8080/health`
- Ready:  `GET http://localhost:8080/ready`
- Quotes: `GET http://localhost:8080/api/v1/quotes`

## API (v1)

전체 라우트는 [`internal/server/router.go`](internal/server/router.go)가 원본이다. 공개 조회(시세·종목·점수·뉴스·실적), 로그인 사용자(보유·관심·메모), 관리자(`/admin/*`, `RequireAdmin`)로 나뉜다.

## 토스증권 Open API

환경 변수 (`be/.env`):

| 변수 | 설명 |
|------|------|
| `TOSS_API_BASE_URL` | API 베이스 URL (기본: `https://openapi.tossinvest.com`) |
| `TOSS_PRIMARY_CLIENT_ID` | Open API client_id — 기본 키 (`TOSS_CLIENT_ID` 별칭) |
| `TOSS_PRIMARY_CLIENT_SECRET` | Open API client_secret (`TOSS_CLIENT_SECRET` 별칭) |
| `TOSS_SECONDARY_CLIENT_ID` | **현재 읽지 않음**(11-05부터 PRIMARY만). 예전엔 PRIMARY 한도 그룹이 소진되면 그 그룹만 이 키로 넘겼다 |
| `TOSS_SECONDARY_CLIENT_SECRET` | 예비 client_secret |

스펙 전문: [`docs/toss-openapi.json`](docs/toss-openapi.json) · 요약: [`docs/README.md`](docs/README.md)

`config.Load().Toss.Enabled()` 로 credentials 설정 여부를 확인합니다. 클라이언트는 `internal/toss/`.
