-- Latest market rankings payload (polled every 5 minutes, KR and US trading amount/volume).

USE quantport;

CREATE TABLE IF NOT EXISTS market_rankings (
  market_country  VARCHAR(8)   NOT NULL,
  ranking_type    VARCHAR(64)  NOT NULL,
  duration        VARCHAR(16)  NOT NULL,
  ranked_at       TIMESTAMP(3) NULL,
  payload         JSON         NOT NULL,
  updated_at      TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (market_country, ranking_type, duration)
) ENGINE=InnoDB;
