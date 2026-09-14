-- AI 추천 랭킹의 축·기간별 상위 목록 (사전 계산 결과)
--
-- # 왜 표를 하나 더 만드나
--
-- 화면이 「추세 점수 · 최근 1주 최고」 순으로 목록을 달라고 하면, 그 답은
-- `stock_scores`를 기간만큼 훑어 종목별 MAX를 구한 뒤 정렬해야 나온다. 실측(개발 PC,
-- 국내 · 30일 창):
--
--     축 하나 · 최신일만            0.10초
--     축 하나 · 최근 1주 MAX        3.87초
--     축 하나 · 최근 1개월 MAX      0.61초
--     여섯 축 한 번에 · 1개월 MAX   2.08초 (국내) / 1.30초 (해외)
--
-- 운영 DB는 1 ECPU라 이보다 몇 배 느리다. 요청 경로에 둘 수 없는 값이다.
--
-- 대신 **점수 배치가 끝난 뒤 한 번** 계산해 여기에 넣고, 조회는 이 표의 한 행을
-- 읽는다. 마켓 레이더가 `market_rankings`에 스냅샷을 넣어 두고 읽는 것과 같은 모양이다
-- (migrations/017).
--
-- # 왜 인덱스를 더 만들지 않나
--
-- 축 점수는 `stock_scores.breakdown` JSON 안에 있어서, 정렬을 SQL에 맡기려면 생성
-- 컬럼 다섯 개와 인덱스 다섯 개가 필요하다. 이 표는 하루 17,900행씩 자라고 30거래일을
-- 보관하므로 인덱스 하나가 수십 MB다. 운영 인스턴스의 여유를 그렇게 쓰지 않는다.
--
-- # 행 수
--
-- 시장 2 × 축 6(종합 + 5축) × 기간 4 = **48행 고정**. 매 배치가 같은 행을 덮어쓴다.

USE quantport;

CREATE TABLE IF NOT EXISTS stock_score_rankings (
  market      VARCHAR(16)  NOT NULL,  -- domestic | overseas
  axis        VARCHAR(16)  NOT NULL,  -- composite | technical | quality | growth | valuation | sentiment
  duration    VARCHAR(16)  NOT NULL,  -- realtime | 1d | 1w | 1mo
  -- 이 목록이 근거로 삼은 가장 최근 점수일. 화면의 「… 기준 점수」가 이 값이다.
  trade_date  DATE         NOT NULL,
  -- 창의 시작일. realtime 이면 trade_date 와 같다.
  from_date   DATE         NOT NULL,
  -- 상위 N 종목의 표시용 행 배열 (model.StockScoreListItem 과 같은 모양).
  payload     JSON         NOT NULL,
  computed_at TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (market, axis, duration)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
