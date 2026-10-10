import { apiRequest } from "@/lib/api-client";

/**
 * AI 랭킹의 「내 기준」 — 저장된 축 점수를 내 가중치로 다시 합산한 랭킹.
 * 점수를 다시 계산하지 않는다(`be/internal/service/score_custom_ranking.go`).
 */
export type ScoreWeights = {
  technical: number;
  quality: number;
  growth: number;
  valuation: number;
  sentiment: number;
};

/** 서버의 `model.ScoreAxes` 순서 — 쿼리 `w=`가 이 순서를 따른다. */
export const WEIGHT_AXES: (keyof ScoreWeights)[] = [
  "technical",
  "quality",
  "growth",
  "valuation",
  "sentiment",
];

export type CustomRankingItem = {
  rank: number;
  symbol: string;
  name: string;
  score: number;
  default: number;
  delta: number;
};

export type CustomRanking = {
  tradeDate: string;
  market: string;
  weights: ScoreWeights;
  defaults: ScoreWeights;
  items: CustomRankingItem[];
  note: string;
};

export function fetchCustomRanking(
  market: "domestic" | "overseas",
  w: ScoreWeights,
  limit = 50,
): Promise<CustomRanking> {
  const q = new URLSearchParams({
    market,
    w: WEIGHT_AXES.map((a) => String(w[a])).join(","),
    limit: String(limit),
  });
  return apiRequest<CustomRanking>(`/api/v1/stocks/scores/custom-ranking?${q}`);
}

export function fetchMyWeights(): Promise<{ weights: ScoreWeights; saved: boolean }> {
  return apiRequest(`/api/v1/user/score-weights`);
}

export function saveMyWeights(w: ScoreWeights): Promise<{ weights: ScoreWeights; saved: boolean }> {
  return apiRequest(`/api/v1/user/score-weights`, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(w),
  });
}
