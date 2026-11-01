# Quant Port — Frontend

Next.js(App Router) + TypeScript + Tailwind v4.

## 실행

```bash
npm ci
npm run dev
```

→ http://localhost:3000 (0.0.0.0 바인딩). API 주소는 `NEXT_PUBLIC_API_URL`(기본 `http://localhost:8080`).

## 스크립트

| 명령 | 설명 |
|------|------|
| `npm run dev` | 개발 서버 |
| `npm run build` | 프로덕션 빌드 |
| `npm run start` | 프로덕션 서버 |
| `npm run lint` | ESLint |
| `npm test` | Vitest 단위 시험 |

## 구조

- `src/app/` — App Router 페이지. `(main)` 서비스 화면, `(auth)` 로그인·가입, `dev/` 시안 갤러리(noindex)
- `src/components/` — 화면별 컴포넌트, `ui/`는 shadcn 기반 공용 부품
- `src/hooks/` — 데이터 조회·폴링 훅
- `src/lib/` — API 클라이언트, 포맷, SEO, 기능 스위치(`hidden-features.ts`)
- `public/logos/` — 종목 로고(`be/cmd/migrate-logos`가 만든다)

디자인 시스템은 [DESIGN.md](./DESIGN.md) 참고. 운영 배포는 루트의 `docker compose --profile prod up -d --build fe`.
