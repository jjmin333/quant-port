-- 백필한 과거 분기에 **진짜 발표일**을 붙인다 — SEC EDGAR 8-K Item 2.02.
--
-- # 왜 필요한가
--
-- 055가 만든 표에는 발표일이 없다. Finnhub `/stock/earnings`가 주는 날짜는
-- `period`, 즉 **달력 분기로 반올림한 기간말**이라(실측 NVDA: 회계 1분기가
-- 2026-04월에 끝났는데 period는 2026-06-30) 캘린더 날짜로 쓸 수 없다.
-- 그대로 올리면 네 날짜에 3천 종목씩 뭉친다:
--
--	2026-06-30  3,007종목
--	2026-03-31  2,959종목
--	2025-12-31  2,894종목
--	2025-09-30  2,841종목
--
-- 아무도 그날 발표하지 않았다.
--
-- # 어디서 진짜 날짜를 얻나
--
-- 미국 상장사는 실적을 **8-K Item 2.02**(Results of Operations)로 공시한다.
-- `data.sec.gov/submissions/CIK{...}.json`이 그 제출일을 준다 — 종목당 1호출,
-- 우리가 이미 쓰는 `edgar_bulk` 버킷.
--
-- # 매칭 규칙 (실측 8종목 검증)
--
-- 기간말 기준 **가장 가까운 미사용 8-K**를 1:1로 짝짓고, 간격이 60일을 넘으면
-- 버린다. 관측된 실제 간격은 −42일(NVDA·WMT처럼 회계연도가 1월에 끝나 반올림이
-- 앞서는 경우)부터 +48일(AMAT)까지였다. 분기 간격이 90일이라 60일 창은 인접
-- 분기를 침범하지 않는다. 짝지은 날짜가 기간말 순서와 어긋나면(단조성 위반)
-- 그 종목은 통째로 버린다 — 한 칸 밀리면 모든 날짜가 조용히 틀리기 때문이다.
--
-- # session은 「8-K 접수 시각」이지 보도자료 시각이 아니다
--
-- `acceptanceDateTime`은 SEC가 제출을 받은 시각이라 보도자료보다 몇 시간 늦다
-- (실측 WMT: 장전 발표인데 접수 10:59 ET). 그래도 쓰는 이유는 **반응 계산이
-- 장전과 장중을 똑같이 다루기** 때문이다(earnings_reaction.go: 둘 다 당일 종가
-- 대 전 거래일 종가). 숫자가 갈리는 것은 장후뿐이고, 장후는 접수가 20시 이후로
-- 뚜렷이 분리된다(실측 AAPL 20:30 · NVDA 20:21 vs WMT 10:59 · CVX 10:16).
ALTER TABLE earnings_surprise_history
  -- 매칭된 8-K 제출일. NULL이면 아직 못 붙였거나 규칙에 걸려 버린 분기다.
  ADD COLUMN announced_on DATE NULL DEFAULT NULL AFTER period_end,
  -- 'bmo' | 'dmh' | 'amc'. 위 주석의 한계를 안고 쓰는 값.
  ADD COLUMN announce_session VARCHAR(8) NULL DEFAULT NULL AFTER announced_on,
  -- 감사용: 기간말과 발표일의 간격(일). 부호가 있다 — 음수는 반올림이 앞선 경우.
  ADD COLUMN announce_gap_days SMALLINT NULL DEFAULT NULL AFTER announce_session,
  -- 'edgar_8k'. 나중에 다른 소스가 붙으면 구분한다.
  ADD COLUMN announce_source VARCHAR(16) NULL DEFAULT NULL AFTER announce_gap_days,
  -- 이 종목의 8-K를 언제 받아 봤는지. 재실행에서 건너뛸 판단에 쓴다 —
  -- **매칭에 실패한 종목도 「봤다」는 사실이 남아야** 다음 실행이 같은 종목을
  -- 3천 번 다시 묻지 않는다.
  ADD COLUMN announce_checked_at TIMESTAMP NULL DEFAULT NULL AFTER announce_source;

-- 캘린더로 승격할 대상(발표일이 붙은 행)을 고르는 질의용.
CREATE INDEX idx_surprise_history_announced
  ON earnings_surprise_history (market, announced_on);
