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
| `TOSS_PRIMARY_CLIENT_ID` | Open API client_id — 기본 키 (`TOSS_CLIENT_ID` 별칭) |
| `TOSS_PRIMARY_CLIENT_SECRET` | Open API client_secret (`TOSS_CLIENT_SECRET` 별칭) |
| `TOSS_SECONDARY_CLIENT_ID` | 예비 client_id. PRIMARY 한도 그룹이 소진되면 그 그룹만 이 키로 넘긴다 |
| `TOSS_SECONDARY_CLIENT_SECRET` | 예비 client_secret |

스펙 전문: [`docs/toss-openapi.json`](docs/toss-openapi.json) · 요약: [`docs/README.md`](docs/README.md)

`config.Load().Toss.Enabled()` 로 credentials 설정 여부를 확인합니다. 연동 클라이언트는 추후 `internal/toss/` 에 추가합니다.

## 다음 단계

- [ ] `internal/toss/` OAuth 토큰 + REST 클라이언트
- [ ] MySQL 스키마·마이그레이션 (`migrations/`)
- [ ] repository/service에서 mock → 토스 API 교체
- [ ] WebSocket 실시간 시세 (토스 WS 지원 시)
- [ ] 키움 등 추가 증권사 연동
- [ ] 인증·사용자별 관심/보유 종목
