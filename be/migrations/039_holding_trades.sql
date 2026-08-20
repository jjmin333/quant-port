USE quantport;

-- 매입 이력 (보유종목 화면 "평가금액 추이"의 근거)
--
-- 왜 필요한가: user_holdings 는 **현재 수량**만 가진다. 그래서 "내 포트폴리오가
-- 요즘 오르고 있나"를 그릴 때 지금 수량을 과거 전체에 곱하는 합성 곡선밖에 못 그렸다.
-- 두 달 전에 산 종목이 석 달 전부터 있던 것처럼 보이는 그림이다.
-- 이 표에 매수/매도를 날짜와 함께 쌓으면 날짜별 실제 보유수량을 복원할 수 있다.
--
-- user_holdings 와의 관계: 이 표에 행이 있는 종목은 여기가 원본이고
-- user_holdings.quantity / avg_price 는 여기서 파생된 캐시다(source='trades').
-- 이력이 없는 종목(스크린샷 임포트만 한 종목)은 종전처럼 user_holdings 가 원본이다.
-- 두 경로를 섞지 않기 위해 파생 여부는 source 컬럼으로 구분한다.
--
-- fx_rate: 매수 시점 환율. 지금 화면은 현재 환율로 환산해 환차손익을 분리하지
-- 못하는데, 사용자가 입력해두면 그때 환율로 원가를 잡을 수 있다. NULL 이면 현재 환율.
-- (마이그레이션 분리기가 세미콜론으로 자르므로 주석에 세미콜론을 쓰지 않는다)

CREATE TABLE IF NOT EXISTS holding_trades (
  id          BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
  user_id     BIGINT UNSIGNED NOT NULL,
  symbol      VARCHAR(32)     NOT NULL,
  -- buy | sell
  side        VARCHAR(4)      NOT NULL DEFAULT 'buy',
  quantity    DECIMAL(18, 4)  NOT NULL,
  price       DECIMAL(18, 4)  NOT NULL,
  currency    CHAR(3)         NOT NULL DEFAULT 'KRW',
  fx_rate     DECIMAL(18, 4)  NULL,
  -- 시장 로컬 기준 체결일
  trade_date  DATE            NOT NULL,
  fee         DECIMAL(18, 4)  NOT NULL DEFAULT 0,
  note        VARCHAR(200)    NOT NULL DEFAULT '',
  created_at  TIMESTAMP(3)    NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at  TIMESTAMP(3)    NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  KEY idx_holding_trades_user_symbol (user_id, symbol, trade_date),
  KEY idx_holding_trades_user_date (user_id, trade_date),
  CONSTRAINT fk_holding_trades_user FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
