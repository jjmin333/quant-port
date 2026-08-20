# 토스 계좌 연동 — 제거됨 (2026-08-20)

예전에는 FE만 `ACCOUNT_FEATURES_VISIBLE=false`로 숨기고 BE(OAuth 키 저장, 포트폴리오 동기화, 인사이트 LLM)는 남겨 두었다. 화면이 없는데도 연동 사용자마다 6시간마다 Gemini를 치는 스케줄러가 돌아서, **계좌 연동 기능 자체를 제거**했다.

## 유지

- Toss Open API **글로벌** 클라이언트 — 시세, 캔들, 호가, 체결, 랭킹, 종목 경고 배치
- `/holdings` 수동 입력·스크린샷 OCR, `user_holdings` / `holding_trades` / `user_cash_balances`
- 관심종목, 스탬프, 메모, 목표가, 뉴스, 종목 AI 요약, 뉴스 시장 브리핑

## 제거

- `/api/v1/broker/toss/*`, `/portfolio/*`, `/insights/dashboard`, `/accounts`, `/orders`, 브로커 `/holdings`, `/disclosures/today`
- `InsightsService.RunScheduler` (홈 포트폴리오 LLM 브리핑)
- 테이블: `user_broker_credentials`, `broker_accounts`, `user_portfolio_cache`, `portfolio_snapshots` (`054_drop_broker_linking.sql`)
- FE: 계좌 연동·거래내역·포트폴리오 분석·리스크·백테스트 페이지, 설정 「거래」 탭, `account-features.ts` 플래그

옛 북마크(`/onboarding/link-broker` 등)는 미들웨어가 `/`로 보낸다.
