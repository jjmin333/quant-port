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
};

export type DailyCloseSeries = {
  symbol?: string;
  indexId?: string;
  name?: string;
  currency: "KRW" | "USD";
  rows: DailyCloseRow[];
};
