# Quant Port API (Go)

Go 기반 REST API 서버입니다. 현재는 목업 데이터를 반환하며, MySQL·Redis 연결 틀과 `/api/v1` 라우트가 준비되어 있습니다.

## 구조

```
be/
├── cmd/api/           # 진입점
├── docs/
│   └── toss-openapi.json   # 토스증권 Open API spec (연동 시 참조)
├── internal/
│   ├── config/        # 환경 설정
│   ├── handler/       # HTTP 핸들러
│   ├── middleware/    # CORS, 로깅, recover
│   ├── mock/          # 목업 데이터 (추후 repository로 교체)
│   ├── model/         # 도메인 모델
│   ├── repository/    # MySQL, Redis
│   ├── server/        # 라우터·서버
│   └── service/       # 비즈니스 로직
├── migrations/        # DB 마이그레이션 (추가 예정)
└── pkg/response/      # JSON 응답 헬퍼
```

## 실행

```bash
# 1. 인프라 (MySQL, Redis) — 프로젝트 루트에서
docker compose up -d

# 2. 환경 변수
cp .env.example .env

# 3. API 서버 (권장: go build → 고정 바이너리 실행)
make run
```

**Windows** — `go run`은 임시 exe를 만들어 방화벽 팝업이 뜰 수 있습니다. 아래를 사용하세요:

```powershell
.\scripts\restart.ps1              # 포트 8080 정리 → build → 실행
.\scripts\restart.ps1 -Background  # 백그라운드 + tmp\api.log
.\scripts\stop.ps1                 # 8080 리스너만 종료
```

빌드 산출물: `be/bin/api.exe` (`.gitignore` 대상)

- Health: `GET http://localhost:8080/health`
- Ready:  `GET http://localhost:8080/ready`
- Quotes: `GET http://localhost:8080/api/v1/quotes`

## API (v1)

| Method | Path | 설명 |
|--------|------|------|
| GET | `/api/v1/quotes/indices` | 시장 지수 |
| GET | `/api/v1/quotes?filter=&q=` | 종목 시세 목록 |
| GET | `/api/v1/quotes/{symbol}` | 종목 상세 |
| GET | `/api/v1/quotes/{symbol}/orderbook` | 호가 |
| GET | `/api/v1/quotes/{symbol}/trades` | 체결 |
| GET | `/api/v1/quotes/{symbol}/candles?interval=5m` | 차트 캔들 |

## 토스증권 Open API

환경 변수 (`be/.env`):

| 변수 | 설명 |
|------|------|
| `TOSS_API_BASE_URL` | API 베이스 URL (기본: `https://openapi.tossinvest.com`) |
| `TOSS_CLIENT_ID` | Open API client_id (WTS에서 발급) |
| `TOSS_CLIENT_SECRET` | Open API client_secret |

스펙 전문: [`docs/toss-openapi.json`](docs/toss-openapi.json) · 요약: [`docs/README.md`](docs/README.md)

`config.Load().Toss.Enabled()` 로 credentials 설정 여부를 확인합니다. 연동 클라이언트는 추후 `internal/toss/` 에 추가합니다.

## 다음 단계

- [ ] `internal/toss/` OAuth 토큰 + REST 클라이언트
- [ ] MySQL 스키마·마이그레이션 (`migrations/`)
- [ ] repository/service에서 mock → 토스 API 교체
- [ ] WebSocket 실시간 시세 (토스 WS 지원 시)
- [ ] 키움 등 추가 증권사 연동
- [ ] 인증·사용자별 관심/보유 종목
