-- 기사별 **감성 점수와 사건 종류** — 심리 축과 변화 감지의 재료
--
-- # 왜 필요한가
--
-- 심리·수급 축은 AI 요약이 다녀간 종목에만 붙었다(요약 LLM이 그때그때 기사 감성을
-- 셌다). 2026-09-23 실측: 그날 심리 축이 있는 종목 **9개**, 2주간 뉴스가 붙은 종목
-- 3,664개. 기사 단위로 한 번 매겨 저장해 두면 점수 배치가 전 종목에 쓸 수 있다.
--
-- # 무엇이 매기나
--
--   score       로컬 감성 모델(be/sentiment — KR-FinBert-SC · FinBERT)의 P(긍정)−P(부정), −1..1
--   label       그 모델의 최다 확률 라벨(positive · neutral · negative)
--   event_type  제목 규칙(model.ClassifyNewsEvent) — 모르면 NULL
--
-- 기사가 아니라 **기사 × 종목**으로 두지 않는 이유: 감성은 제목의 속성이고, 한 기사에
-- 붙은 종목이 여럿이면 같은 점수를 나눠 쓴다. 종목 쪽 집계는 `symbols_json`을 펼쳐서 한다.
--
-- # 대조 규칙
--
-- `news_articles`는 utf8mb4_unicode_ci다(다른 표는 0900_ai_ci). article_id로 조인하므로 그쪽에
-- 맞춘다 — 다르면 조인이 「Illegal mix of collations」로 실패한다(2026-09-23 실제로 그랬다).
--
-- # 보관
--
-- 기사 행과 함께 간다 — 기사가 정리되면 이 행도 의미가 없다(외래 키는 걸지 않는다:
-- 기사 정리 배치가 이 표를 몰라도 되게). 행당 약 100B, 하루 약 4,500행.

USE quantport;

CREATE TABLE IF NOT EXISTS news_article_sentiment (
  article_id  VARCHAR(64)  NOT NULL PRIMARY KEY,
  score       DECIMAL(5,4) NOT NULL,
  label       VARCHAR(8)   NOT NULL,
  event_type  VARCHAR(24)  NULL,
  model       VARCHAR(48)  NOT NULL,
  labeled_at  TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  KEY idx_news_sentiment_event (event_type)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
