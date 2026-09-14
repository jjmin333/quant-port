-- 지수 LLM 해설 (`llm/index_note.go`, `service/index_llm_note.go`)
--
-- `ai_summary`에는 지금까지 **규칙이 만든 지표 나열**이 들어 있었다:
--   「나스닥 — 최근 5거래일 +1.4%, 20거래일 +4.6%, 20일선 위, RSI 56.8(strong), …」
-- 사실은 다 맞지만 읽어도 무슨 상황인지는 모른다 — RSI 56.8이 좋은 건지, 20일선
-- 위인데 52주 최고 대비 −2%인 건 어떤 국면인지를 읽는 사람이 직접 조립해야 했다.
-- 이름에 「AI」가 붙어 있어 오해도 샀다.
--
-- 그 문장을 **덮어쓰지 않고 옆에 둔다**. 이유는 하나다: LLM은 꺼질 수도, 실패할
-- 수도, 예산에 걸릴 수도 있다. 그때 지수 칸이 빈 채로 남는 것보다 예전 지표 나열이
-- 남는 편이 낫다. 규칙 문장은 열세 지수 전부에 언제나 있고, 아래 두 칸은 배치가
-- 실제로 돌았을 때만 찬다.
--
--   llm_note   — 한 문단 해설.
--   llm_stance — 국면 라벨(「상승 흐름」·「방향 없음」·「하락 흐름」) 중 하나.
--   llm_at     — 언제 만든 것인지. 오래된 해설을 오늘 것처럼 보여 주지 않기 위해서다.

ALTER TABLE index_daily_closes
  ADD COLUMN llm_note TEXT NOT NULL DEFAULT ('') AFTER tags,
  ADD COLUMN llm_stance VARCHAR(16) NOT NULL DEFAULT '' AFTER llm_note,
  ADD COLUMN llm_at DATETIME(3) NULL AFTER llm_stance;
