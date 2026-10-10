import { apiRequest } from "@/lib/api-client";
import { cachedGet } from "@/lib/fetch-cache";

/**
 * AI 점수 성적표 — 축마다 「점수가 높을수록 그 뒤 5거래일에 더 올랐나」(스피어만 IC).
 *
 * 서버는 점수 배치가 매일 쌓은 값을 요약만 한다(`be/internal/service/score_report_card.go`).
 * 표본이 짧으면 `significant=false`다 — 화면은 그것을 숨기지 않고 크게 말한다.
 */
export type ScoreReportAxis = {
  axis: string;
  days: number;
  meanIC?: number;
  tStat?: number;
  positiveShare?: number;
  meanTopExcess?: number;
  meanTopHit?: number;
  significant: boolean;
  series: { date: string; ic: number }[];
};

export type ScoreReportCard = {
  market: string;
  horizon: number;
  topN: number;
  minDays: number;
  axes: ScoreReportAxis[];
  methodChanges: { date: string; note: string }[];
};

export function fetchScoreReportCard(market: "domestic" | "us"): Promise<ScoreReportCard> {
  const path = `/api/v1/stocks/scores/report-card?market=${market}`;
  // 하루 한 번 바뀌는 값이다.
  return cachedGet(
    path,
    () => apiRequest<ScoreReportCard>(path, undefined, "성적표 조회 실패"),
    10 * 60_000,
  );
}

/** 판정 — 숫자보다 먼저 읽히는 한 단어. */
export function reportVerdict(
  a: ScoreReportAxis,
  minDays: number,
): "short" | "noise" | "positive" | "negative" {
  if (a.days < minDays) return "short";
  if (!a.significant || a.meanIC == null) return "noise";
  return a.meanIC > 0 ? "positive" : "negative";
}
