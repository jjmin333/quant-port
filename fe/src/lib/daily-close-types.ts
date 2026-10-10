/** Mirrors be/internal/model/daily_close.go. */

export type DailyCloseRow = {
  tradeDate: string; // YYYY-MM-DD
  weekday: string; // 월 · 화 · …
  close: number;
  prevClose?: number;
  changeAmount?: number;
  changeRate?: number;
  volume?: number;
  /** null when no analysis ran for that date — rendered as "-". */
  score: number | null;
  /** "" when no analysis ran. */
  aiSummary: string;
  /**
   * 그날의 「핵심 요약」 한 문단 (2026-09-15 사용자 요청 1).
   *
   * `aiSummary`가 네 갈래 본문을 **이어붙인** 글인 반면 이쪽은 같은 LLM 호출이 그 넷을
   * 전부 근거로 **다시 쓴** 요약이다. 모달의 「간단」이 이 값을 쓴다.
   *
   * **없을 수 있다** — 이 컬럼은 2026-09-15 배치부터 쌓이므로 그 이전 날짜의 행은
   * 비어 온다. 그때는 화면이 예전 방식(`aiSummary`의 앞 두 문장)으로 되돌아간다.
   */
  aiOverview?: string;
  analysisSource?: "llm" | "rule" | "";
  /**
   * 지수 전용 — 그날의 LLM 해설과 국면 라벨. 종목 응답에는 없다.
   *
   * 지수는 규칙 문장(`aiSummary`)과 해설이 **따로**다. 규칙 문장은 언제나 있고 해설은
   * 배치가 돈 날에만 있어, 한 칸에 합치면 어느 쪽을 보고 있는지 화면이 구분할 수 없다.
   */
  llmNote?: string;
  llmStance?: string;
};

export type DailyCloseSeries = {
  symbol?: string;
  indexId?: string;
  name?: string;
  currency: "KRW" | "USD";
  rows: DailyCloseRow[];
};
