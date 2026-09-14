# Quant Port — 통합 자산 관리 플랫폼

프론트엔드(Next.js)와 백엔드(Go) 모노레포입니다.

```
quant-port/
├── fe/                 # Next.js 16 + TypeScript + Tailwind v4
├── be/                 # Go REST API (chi + MySQL + Redis)
├── docker-compose.yml  # Redis + API (Tesseract 포함)
└── README.md
```

## 기술 스택

| 영역 | 스택 |
|------|------|
| Frontend (`fe/`) | Next.js 16, React 19, TypeScript, Tailwind CSS v4, Recharts |
| Backend (`be/`) | Go 1.25, chi, MySQL, Redis |
| Infra | 로컬 MySQL + Docker Compose (Redis 7 + API/Tesseract) |

## 문서

- [프론트엔드 디자인 시스템](fe/DESIGN.md)
- [백엔드 API README](be/README.md)
