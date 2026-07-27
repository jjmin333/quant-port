# Quant Port — 통합 자산 관리 플랫폼

프론트엔드(Next.js)와 백엔드(Go) 모노레포입니다.

```
quant-port/
├── fe/                 # Next.js 16 + TypeScript + Tailwind v4
├── be/                 # Go REST API (chi + MySQL + Redis)
├── docker-compose.yml  # 개발용 Redis (MySQL은 로컬 PC 사용)
└── README.md
```

## 빠른 시작

### 1. 인프라

**MySQL** — 로컬 PC에 설치된 MySQL `3306`, DB `quantport` 사용.  
`be/.env`의 `MYSQL_DSN`을 로컬 계정에 맞게 설정하세요.

**Redis** — Docker로 실행:

```bash
docker compose up -d
```

### 2. 프론트엔드

```bash
cd fe
npm install
npm run dev
```

→ http://localhost:3000

### 3. 백엔드

```powershell
cd be
cp .env.example .env
.\scripts\restart.ps1          # 빌드 후 실행 (방화벽 친화적)
# 또는 백그라운드: .\scripts\restart.ps1 -Background
```

Linux/macOS: `make run` (동일하게 `go build` → `bin/api` 실행)

→ http://localhost:8080/health

## 기술 스택

| 영역 | 스택 |
|------|------|
| Frontend (`fe/`) | Next.js 16, React 19, TypeScript, Tailwind CSS v4, Recharts |
| Backend (`be/`) | Go 1.25, chi, MySQL, Redis |
| Infra | 로컬 MySQL + Docker Compose (Redis 7) |

## 문서

- [프론트엔드 디자인 시스템](fe/DESIGN.md)
- [백엔드 API README](be/README.md)

## 개발 메모

- 프론트는 현재 `fe/src/lib/*-mock-data.ts` 목업 데이터 사용
- 백엔드 `/api/v1/quotes/*` 엔드포인트가 동일 도메인 API 틀 제공 (목업 응답)
- 추후 FE에서 `NEXT_PUBLIC_API_URL=http://localhost:8080` 로 BE 연동 예정
