# Quant Port — Frontend

Next.js 기반 통합 자산 관리 UI입니다.

## 실행

```bash
npm install
npm run dev
```

→ http://localhost:3000 (0.0.0.0 바인딩)

## 스크립트

| 명령 | 설명 |
|------|------|
| `npm run dev` | 개발 서버 |
| `npm run build` | 프로덕션 빌드 |
| `npm run start` | 프로덕션 서버 |
| `npm run lint` | ESLint |

## 구조

- `src/app/` — App Router 페이지
- `src/components/` — UI·레이아웃·대시보드·시세 컴포넌트
- `src/lib/` — 목업 데이터, 포맷, 설정

디자인 시스템은 [DESIGN.md](./DESIGN.md) 참고.

## 백엔드 연동 (예정)

```env
NEXT_PUBLIC_API_URL=http://localhost:8080
```

프로젝트 루트 README에서 전체 모노레포 실행 방법을 확인하세요.
