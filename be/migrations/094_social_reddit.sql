-- Reddit 커뮤니티 탭 (2026-09-29 사용자 요청 — ApeWisdom 최대 활용 + 해외 종목 커뮤니티 탭)
--
--   social_insights   ApeWisdom 종목 페이지의 24시간 요약 — 언급한 사용자 수 · 감성(긍정 비율) ·
--                     WSB 감성 · 함께 나온 낱말. 목록 API(social_mentions)에 없는 값들이다.
--                     social_mentions 잡이 언급 상위 종목만 받는다(하루 한 줄).
--   social_posts      Reddit 글(제목·앞부분)과 번역 — Reddit 검색 RSS(이번 주 인기순).
--                     JSON API는 이 서버에서 403이라 RSS만 쓴다. 원문 링크로 보낸다.
--   social_summaries  커뮤니티 점수(규칙, -100..100)와 AI 요약(LLM 배치) — 종목당 최신 한 줄.
--
-- 전부 배치가 채우고 조회는 표만 읽는다.

USE quantport;

CREATE TABLE IF NOT EXISTS social_insights (
  symbol            VARCHAR(32)  NOT NULL,
  snap_date         DATE         NOT NULL,
  users             INT          NULL,
  sentiment_pct     TINYINT      NULL,
  sentiment_wsb_pct TINYINT      NULL,
  -- [{"word":"calls","n":7}, …] 많은 순
  keywords          JSON         NULL,
  fetched_at        TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (symbol, snap_date)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS social_posts (
  symbol       VARCHAR(32)   NOT NULL,
  post_id      VARCHAR(16)   NOT NULL,
  subreddit    VARCHAR(40)   NOT NULL,
  title        VARCHAR(400)  NOT NULL,
  title_ko     VARCHAR(600)  NULL,
  snippet      VARCHAR(600)  NOT NULL DEFAULT '',
  snippet_ko   VARCHAR(900)  NULL,
  url          VARCHAR(400)  NOT NULL,
  -- 검색 결과에서의 자리(0 = 이번 주 가장 인기). RSS에는 추천 수가 없어 순서로 대신한다.
  top_rank     INT           NOT NULL DEFAULT 0,
  published_at DATETIME      NOT NULL,
  fetched_at   TIMESTAMP(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (symbol, post_id),
  KEY idx_sp_symbol_pub (symbol, published_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS social_summaries (
  symbol      VARCHAR(32)  NOT NULL PRIMARY KEY,
  snap_date   DATE         NOT NULL,
  score       INT          NOT NULL,
  -- {"summary":"…","tags":["…"],"bull":["…"],"bear":["…"]}
  body        JSON         NOT NULL,
  source      VARCHAR(8)   NOT NULL,
  model       VARCHAR(64)  NOT NULL DEFAULT '',
  input_hash  CHAR(64)     NOT NULL DEFAULT '',
  tokens      INT          NOT NULL DEFAULT 0,
  created_at  TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
