-- 종목별 **마지막 LLM 요약 글**을 영구 보관한다.
--
-- # 왜 필요한가
--
-- AI 요약은 Redis에 24시간 TTL로 캐시된다. 재빌드가 돌 때 LLM을 새로 부르지 않는
-- 경로(= 거의 모든 경로 — LLM 지출은 배치와 관리자 수동뿐이다)에서는 **캐시에 있던
-- 지난 글을 이어 받는 것**이 유일한 보존 수단이었다(`carryOverNarrative`).
--
-- 그 말은, 어떤 종목을 24시간 동안 아무도 열지 않으면 캐시가 만료되고 **그 종목이
-- 돈 주고 만든 글이 사라진다**는 뜻이다. 다음 방문자는 규칙 문장을 본다.
--
-- 개요 탭이 「LLM 글이 없으면 AI 핵심 요약 카드를 아예 안 그린다」로 바뀌면서
-- (2026-09-03 사용자 결정) 이 손실이 눈에 보이게 됐다 — 어제 요약이 있던 종목의
-- 카드가 오늘 사라진다.
--
-- # 왜 stock_daily_closes.ai_summary로는 안 되나
--
-- 그 칼럼에는 밸류에이션·기술적 두 문장을 이어 붙인 **한 덩어리 글**만 들어간다
-- (`dailyCloseSummaryText`). 개요 문단·네 섹션·키워드·뉴스 감성이 없어서 화면을
-- 복원할 수 없다. 여기서는 응답 그대로를 JSON으로 보관한다.
--
-- # 크기
--
-- 종목당 한 행이고 실측 글 길이는 1~2KB다. LLM 글 이력이 있는 종목은 744개(전체의
-- 4%)이므로 지금 기준 1.5MB 남짓이고, 배치가 넓어져도 종목 수(17,900)가 상한이다.

USE quantport;

CREATE TABLE IF NOT EXISTS stock_summary_narratives (
  symbol      VARCHAR(32)  NOT NULL,
  -- overview / valuation / technical / insider / news / keywords / newsSentiment.
  -- 숫자는 넣지 않는다 — 그건 매 빌드가 다시 계산한다.
  payload     JSON         NOT NULL,
  -- 이 글이 **실제로 쓰인** 시각. 화면의 「N월 N일 생성」이 이 값이다.
  analyzed_at TIMESTAMP(3) NOT NULL,
  updated_at  TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (symbol)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
