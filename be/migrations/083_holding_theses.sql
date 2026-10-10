-- 보유 종목의 **매수 이유**(살 때 본 축) — 보유 이유 점검이 그 축을 추적한다 (2026-09-24 사용자 결정)
--
-- # 왜 user_holdings의 열이 아닌가
--
-- `PUT /user/holdings`(보유 전체 교체 — 스크린샷 가져오기·일괄 편집)가 행을 **지우고 다시
-- 넣는다**(`UserHoldingsRepo.Replace`). 열에 두면 가져오기를 할 때마다 매수 이유가 사라진다.
-- 따로 두면 교체와 무관하게 남고, 같은 종목을 다시 담으면 그 이유가 다시 붙는다.
--
-- axes는 쉼표로 이은 축 id(technical · quality · growth · valuation · sentiment), 최대 셋.

USE quantport;

CREATE TABLE IF NOT EXISTS holding_theses (
  user_id     BIGINT       NOT NULL,
  symbol      VARCHAR(32)  NOT NULL,
  axes        VARCHAR(64)  NOT NULL,
  updated_at  TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (user_id, symbol)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
