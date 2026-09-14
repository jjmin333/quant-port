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
