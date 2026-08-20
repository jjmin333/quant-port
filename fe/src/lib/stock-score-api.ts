import { apiRequest } from "@/lib/api-client";
import { cachedGet } from "@/lib/fetch-cache";

/** Mirrors be/internal/model/stock_score.go. */
export type ScoreAxisId = "technical" | "quality" | "growth" | "valuation" | "sentiment";

export type ScoreBreakdown = Partial<Record<ScoreAxisId, number>>;

export type StockScoreData = {
  symbol: string;
  name?: string;
  tradeDate: string;
  score: number | null;
  grade?: string;
  scoreKind: string;
  coverage: number;
  peerGroup?: string;
  peerPct?: number;
  breakdown: ScoreBreakdown;
  inputs?: Record<string, unknown>;
  weights: Record<ScoreAxisId, number>;
  weightSource: string;
  computedAt: string;
};

export const SCORE_AXIS_LABELS: Record<ScoreAxisId, string> = {
  technical: "추세",
  quality: "수익성",
  growth: "성장",
  valuation: "가치",
  sentiment: "심리·수급",
};

export const SCORE_AXIS_ORDER: ScoreAxisId[] = [
  "technical",
  "quality",
  "growth",
  "valuation",
  "sentiment",
];

/**
 * Which axis each workspace tab is about, so a tab can show the one score that
 * belongs to it rather than the whole composite.
 */
export const TAB_SCORE_AXIS: Record<string, ScoreAxisId> = {
  chart: "technical",
  valuation: "valuation",
  financials: "quality",
  earnings: "growth",
  news: "sentiment",
  insider: "sentiment",
};

/**
 * 여러 종목 점수를 한 번에.
 *
 * 보유종목 화면은 들고 있는 종목 전부의 점수가 필요한데, 단건 API를 종목 수만큼 부르면
 * 20종목이면 20요청이다. BE에 배치 엔드포인트를 뚫고 여기로 묶었다.
 * 점수가 없는 종목은 응답에서 그냥 빠진다 — 단건 API가 `null`을 주는 것과 같은 뜻이다.
 */
export async function fetchScoresBatchClient(symbols: string[]): Promise<StockScoreData[]> {
  const list = [...new Set(symbols.map((s) => s.trim().toUpperCase()).filter(Boolean))];
  if (list.length === 0) return [];
  const data = await apiRequest<StockScoreData[] | null>(
    `/api/v1/stocks/scores/batch?symbols=${encodeURIComponent(list.join(","))}`,
    undefined,
    "점수 조회 실패",
  );
  return data ?? [];
}

export function fetchStockScoreClient(symbol: string): Promise<StockScoreData | null> {
  return apiRequest<StockScoreData | null>(
    `/api/v1/stocks/${encodeURIComponent(symbol)}/score`,
    undefined,
    "점수 조회 실패",
  );
}

/** Mirrors be/internal/model/stock_score.go StockScoreListItem. */
export type StockScoreListItem = {
  rank: number;
  symbol: string;
  name: string;
  assetType?: string;
  score: number;
  grade?: string;
  peerPct?: number;
  changeRate?: number;
  closePrice?: number;
  currency?: string;
  tradeDate: string;
};

export type ScoreRankingMarket = "domestic" | "overseas";

export async function fetchScoreRankingsClient(
  market: ScoreRankingMarket,
  limit = 20,
): Promise<{ items: StockScoreListItem[]; tradeDate: string }> {
  const params = new URLSearchParams({ market, limit: String(limit) });
  const path = `/api/v1/stocks/scores/top?${params.toString()}`;
  // 이 랭킹은 하루 한 번 도는 배치가 만든다. 국내/해외 토글을 오가거나 화면을
  // 떠났다 돌아올 때마다 다시 받을 이유가 없다.
  const items = await cachedGet(
    path,
    () =>
      apiRequest<StockScoreListItem[] | null>(path, undefined, "종목 랭킹 조회 실패"),
    5 * 60_000,
  );
  const rows = items ?? [];
  return { items: rows, tradeDate: rows[0]?.tradeDate ?? "" };
}

/**
 * 종합 점수 신호등 구간 (08-12 사용자 결정): 0~35 빨강 · 35~65 주황 · 65~100 초록.
 *
 * 이 함수는 **종합 점수에만** 쓴다. 5축·레이더·펀더멘털 같은 하위 점수에 같은 색을
 * 입히면 화면이 온통 초록이 되고, 정작 종합 점수가 어느 구간인지가 묻힌다.
 */
export function scoreTone(score: number): ScoreTone {
  if (score >= 65) return "high";
  if (score >= 35) return "mid";
  return "low";
}

export type ScoreTone = "high" | "mid" | "low";

/**
 * 점수 신호등 — 초록 / 주황 / 빨강 (사용자 결정 08-11).
 *
 * 이 표를 여기 둔 이유: 예전엔 같은 맵이 5개 파일에 복사돼 있었고, 그중 `mid`만
 * 파랑(`--brand`)이라 화면마다 중간 점수 색이 달랐다. 게다가 `--success`·`--danger`는
 * **정의된 적조차 없어서** 실제로는 전부 상속색(검정)으로 떨어지고 있었다.
 *
 * ⚠️ 점수 색은 **점수를 말하는 자리에만** 쓴다. 등락률 칸은 `trendTextClass`(빨강=상승,
 * 파랑=하락)가 주인이고, 거기에 점수 색을 섞으면 빨강이 두 가지를 뜻하게 된다.
 *
 * ⚠️ **종목 랭킹 목록은 예외로 색을 쓰지 않는다** (08-12 사용자 결정) — `SCORE_NEUTRAL_TEXT`.
 * 스무 줄이 세로로 붙어 있는 목록에서 점수마다 색이 붙으면 화면이 초록 벽이 되고,
 * 정작 어느 종목이 높은지는 이미 정렬 순서가 말하고 있다.
 */
export const SCORE_TONE_TEXT: Record<ScoreTone, string> = {
  high: "text-[var(--score-high)]",
  mid: "text-[var(--score-mid)]",
  low: "text-[var(--score-low)]",
};

export const SCORE_TONE_BG: Record<ScoreTone, string> = {
  high: "bg-[var(--score-high)]",
  mid: "bg-[var(--score-mid)]",
  low: "bg-[var(--score-low)]",
};

export const SCORE_TONE_BORDER: Record<ScoreTone, string> = {
  high: "border-[var(--score-high)]/40",
  mid: "border-[var(--score-mid)]/40",
  low: "border-[var(--score-low)]/40",
};

/** 등급 배지처럼 옅은 배경 + 같은 계열 글자가 필요한 곳. */
export const SCORE_TONE_BADGE: Record<ScoreTone, string> = {
  high: "bg-[var(--score-high)]/12 text-[var(--score-high)]",
  mid: "bg-[var(--score-mid)]/12 text-[var(--score-mid)]",
  low: "bg-[var(--score-low)]/12 text-[var(--score-low)]",
};

/**
 * 일부러 색을 빼는 자리 — 본문과 같은 글씨색(라이트 검정 / 다크 흰색).
 *
 * 상수로 둔 이유: `text-foreground`를 그냥 적으면 다음 사람이 "색 지정을 깜빡했나"
 * 하고 신호등으로 되돌린다. 지금은 종목 랭킹 목록이 유일한 사용처다.
 */
export const SCORE_NEUTRAL_TEXT = "text-foreground";
