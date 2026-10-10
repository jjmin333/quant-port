USE quantport;

-- 매매 이력 trade_date 의 종가(stock_daily_closes)와 기록된 매수 단가(평단)가
-- 다른 종목은 평가금액 추이 차트에 오점을 만든다. 해당 user/symbol 의
-- holding_trades 를 모두 삭제하고 user_holdings.source 를 manual 로 되돌린다.
-- (2026-09-23부터 기동 경로에서 돌지 않는다 — 장중 체결가는 정상 기록이다.
--  수동 실행: go run ./cmd/cleanup-holding-trades -dry-run)
