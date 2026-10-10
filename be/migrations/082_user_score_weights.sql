-- 사용자별 **축 가중치** — AI 랭킹의 「내 기준」 보기가 쓴다 (2026-09-23 사용자 결정)
--
-- 점수를 다시 계산하지 않는다. 저장된 축 점수를 이 가중치로 다시 합산만 한다
-- (`ScoreService.CustomRanking` — 관리자 「가중치 바꿔 보기」와 같은 산식). 그래서 배치도
-- 없고, 이 표는 사용자당 한 행이다.

USE quantport;

CREATE TABLE IF NOT EXISTS user_score_weights (
  user_id      BIGINT       NOT NULL PRIMARY KEY,
  w_technical  SMALLINT     NOT NULL,
  w_quality    SMALLINT     NOT NULL,
  w_growth     SMALLINT     NOT NULL,
  w_valuation  SMALLINT     NOT NULL,
  w_sentiment  SMALLINT     NOT NULL,
  updated_at   TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
