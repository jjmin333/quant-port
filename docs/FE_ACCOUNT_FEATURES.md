# FE 계좌 연동 기능 비노출 (2026-07-26)

## 결정

**프론트엔드에서 계좌·증권사 연동 관련 메뉴와 화면을 비노출**한다.  
백엔드 API, DB 스키마, 저장 로직은 **삭제하지 않고 유지**한다 (추후 재활용 가능).

제어 플래그: `fe/src/lib/account-features.ts` → `ACCOUNT_FEATURES_VISIBLE = false`

## 배경

개인 투자자는 여러 증권사 계좌를 나눠 사용하는 경우가 많다.  
그러나 **모든 증권사가 Open API를 제공하지 않아**, 특정 증권사만 연동 가능한 현재 구조에서는:

- 보유종목 통합 조회
- 실시간 체결·주문 연동
- 계좌 단위 포트폴리오 분석

등의 기능이 **연동 가능한 증권사 사용자에게만** 의미가 있고, 그렇지 않은 사용자에게는 제약·혼란을 준다.

당분간 FE에서는 계좌 의존 UX를 숨기고, **시세·관심종목·뉴스·종목 분석** 등 API 제약이 적은 기능에 집중한다.

## BE / Storage

| 영역 | 처리 |
|------|------|
| `/api/v1/broker/*`, `/accounts`, `/orders`, `/portfolio/*` | **유지** |
| Toss OAuth, broker credentials 테이블 | **유지** |
| Portfolio poller, Redis/MySQL 캐시 | **유지** |

재활성화 시 BE 변경 없이 FE 플래그만 `true`로 되돌리면 된다.

## FE 비노출 범위

### 사이드바 (제거된 메뉴)

| 그룹 | 메뉴 |
|------|------|
| 계좌 | 계좌 연동, 보유종목 통합, 거래내역 |
| 분석 | 포트폴리오 분석, 리스크 분석 |
| 기록 | 백테스트 |

### 기타 UI

- 헤더 **「계좌 연동」** CTA
- 설정 **「거래」** 탭 (증권사 API 키 연동)
- 대시보드: 계좌 연동 부제·포트폴리오 위젯 (자산 요약, 계좌 목록, 보유 미리보기, 자산 추이 등)
- 종목 상세 **「보유 현황」** 카드 (연동 계좌 기준)

### URL 접근

플래그가 `false`일 때 위 경로로 직접 접근하면 `/dashboard`로 리다이렉트 (`middleware.ts`).

## FE 유지 (노출) 기능 예

- 통합 대시보드 (시장 개요 중심)
- 실시간 시세, 관심종목
- 종목 분석, 재무, 뉴스
- AI 투자일지, 목표가 (계좌 비의존 UI)

## 재활성화 절차

1. `fe/src/lib/account-features.ts`에서 `ACCOUNT_FEATURES_VISIBLE = true`
2. BE `.env`에 Toss 등 broker 설정 확인
3. FE 빌드·배포 후 사이드바·설정·대시보드 포트폴리오 위젯 복구 확인

## 관련 파일

- `fe/src/lib/account-features.ts` — 플래그·경로 헬퍼
- `fe/src/components/layout/sidebar.tsx`
- `fe/src/components/layout/header.tsx`
- `fe/src/lib/settings-config.ts`, `settings-panel.tsx`
- `fe/src/app/(main)/dashboard/page.tsx`
- `fe/src/middleware.ts`
