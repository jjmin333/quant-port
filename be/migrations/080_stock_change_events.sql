-- 종목별 **변화 이벤트** — 「변화」 메뉴가 읽는 표
--
-- # 무엇을 담나
--
-- 점수 배치 끝에 하루 한 번(`ChangeDetectService`), 그날 전 종목에서 일어난 일을 규칙으로
-- 골라 한 줄씩 남긴다:
--
--   price_move      종목별 변동성 기준을 넘은 등락(고정 % 기준이 아니다)
--   volume_surge    20일 평균의 5배 이상 거래량
--   score_jump      AI 점수 ±8 이상 — 축이 생기거나 사라져서 바뀐 경우는 뺀다
--   news_event      기사 사건(목표가·실적·수주·증자·M&A …, 제목 규칙)
--   disclosure      주요 공시(단일판매·공급계약 · 유상증자 · 자기주식 · 최대주주 변경 …)
--   earnings        실적 발표 — 컨센서스가 있으면 서프라이즈 방향과 크기
--   insider         내부자 대량 순매매(평균 거래량 대비)
--
-- `explained`는 등락·거래량 이벤트에만 뜻이 있다: 같은 날 그 종목에 다른 사건이나 기사가
-- 있었으면 1, **아무것도 없으면 0** — 화면이 「설명 불가」를 그대로 말한다(타임라인과
-- 같은 정직성). 나머지 종류는 NULL이다.
--
-- # 멱등
--
-- `detected_for`(점수 기준일) 단위로 지우고 다시 쓴다 — 재계산하면 그날 목록이 통째로 바뀐다.
--
-- # 보관
--
-- 90일. 하루 수백 행이다.

USE quantport;

CREATE TABLE IF NOT EXISTS stock_change_events (
  id            BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
  detected_for  DATE          NOT NULL,
  event_date    DATE          NOT NULL,   -- 그 일이 일어난 날(미국 종목은 현지 세션일)
  symbol        VARCHAR(32)   NOT NULL,
  kind          VARCHAR(24)   NOT NULL,
  subtype       VARCHAR(24)   NOT NULL DEFAULT '',  -- 사건 종류(model.NewsEventType) 등
  direction     TINYINT       NOT NULL DEFAULT 0,   -- +1 · 0 · −1
  magnitude     DECIMAL(12,4) NULL,                 -- 종류별 단위(%, 배수, 점)
  -- 중요도 — 종류마다 단위가 다른 magnitude를 「그 종류의 문턱 대비 몇 배」로 옮긴 값.
  -- 목록 정렬이 이것 하나로 된다.
  importance    DECIMAL(8,3)  NOT NULL DEFAULT 0,
  title         VARCHAR(255)  NOT NULL,
  explained     TINYINT       NULL,
  detail        JSON          NULL,
  created_at    TIMESTAMP(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  KEY idx_change_events_day (detected_for, importance),
  KEY idx_change_events_symbol (symbol, detected_for)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
