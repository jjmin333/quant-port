import type { ScoreRankingMarket } from "@/lib/stock-score-api";

/*
AI 추천 랭킹의 두 손잡이 — **무엇으로 줄 세울까(축)** 와 **언제의 값으로(기간)**.

실시간 차트의 「거래대금/거래량 × 실시간~1년」과 같은 자리다(`market-radar-config`).
이 화면은 그 자리를 점수로 바꾼다.

BE의 `model/score_ranking.go`와 같은 목록을 들고 있어야 한다 — 이름이 어긋나면
서버가 조용히 기본값(종합·실시간)으로 되돌린다.
*/

export type ScoreRankingAxis =
  | "composite"
  | "technical"
  | "quality"
  | "growth"
  | "valuation"
  | "sentiment";

export type ScoreRankingDuration = "realtime" | "1d" | "1w" | "1mo";

export type ScoreRankingParams = {
  market: ScoreRankingMarket;
  axis: ScoreRankingAxis;
  duration: ScoreRankingDuration;
};

/** 종합이 맨 앞. 나머지는 점수 카드가 그리는 다섯 축과 같은 순서다. */
export const SCORE_RANKING_AXIS_OPTIONS: { id: ScoreRankingAxis; label: string }[] = [
  { id: "composite", label: "종합" },
  { id: "technical", label: "추세" },
  { id: "quality", label: "수익성" },
  { id: "growth", label: "성장" },
  { id: "valuation", label: "가치" },
  { id: "sentiment", label: "심리·수급" },
];

/*
기간 넷 (2026-09-03 사용자 결정).

실시간 차트는 일곱이지만 여기는 넷이다 — 점수는 하루 한 번 계산되고 저장 기간이
30거래일이라 3개월·6개월·1년은 1개월과 같은 답이 나온다. 없는 구분을 버튼으로
만들면 눌러 본 사람이 「고장 났나」로 읽는다.

**`realtime`은 「가장 최근 배치일의 점수」다.** 나머지는 그 기간 안에서 한 번이라도
도달한 **최고 점수**로 줄을 세운다.
*/
export const SCORE_RANKING_DURATION_OPTIONS: { id: ScoreRankingDuration; label: string }[] = [
  { id: "realtime", label: "실시간" },
  { id: "1d", label: "1일" },
  { id: "1w", label: "1주" },
  { id: "1mo", label: "1개월" },
];

export const SCORE_RANKING_MARKET_OPTIONS: { id: ScoreRankingMarket; label: string }[] = [
  { id: "domestic", label: "국내" },
  { id: "overseas", label: "해외" },
];

const AXIS_IDS = new Set(SCORE_RANKING_AXIS_OPTIONS.map((o) => o.id));
const DURATION_IDS = new Set(SCORE_RANKING_DURATION_OPTIONS.map((o) => o.id));

export function parseScoreRankingAxis(value: string | null): ScoreRankingAxis {
  return value && AXIS_IDS.has(value as ScoreRankingAxis)
    ? (value as ScoreRankingAxis)
    : "composite";
}

export function parseScoreRankingDuration(value: string | null): ScoreRankingDuration {
  return value && DURATION_IDS.has(value as ScoreRankingDuration)
    ? (value as ScoreRankingDuration)
    : "realtime";
}

export function parseScoreRankingMarket(value: string | null): ScoreRankingMarket {
  return value === "overseas" ? "overseas" : "domestic";
}

export function parseScoreRankingParams(searchParams: URLSearchParams): ScoreRankingParams {
  return {
    market: parseScoreRankingMarket(searchParams.get("market")),
    axis: parseScoreRankingAxis(searchParams.get("axis")),
    duration: parseScoreRankingDuration(searchParams.get("duration")),
  };
}

/**
 * 주소에 싣는다 — **기본값은 적지 않는다.**
 *
 * 그래야 화면을 처음 여는 주소(`/analysis`)와 종합·실시간을 고른 주소가 같아진다.
 * 다르면 뒤로가기가 「아무것도 안 바뀌는 한 칸」을 만든다.
 */
export function scoreRankingSearchParams(params: ScoreRankingParams): URLSearchParams {
  const q = new URLSearchParams();
  if (params.market !== "domestic") q.set("market", params.market);
  if (params.axis !== "composite") q.set("axis", params.axis);
  if (params.duration !== "realtime") q.set("duration", params.duration);
  return q;
}

export function scoreRankingParamsKey(params: ScoreRankingParams): string {
  return `${params.market}|${params.axis}|${params.duration}`;
}

/**
 * 목록 위에 적는 한 줄 — **이 순위가 무엇을 잰 것인지.**
 *
 * 예전에는 「YYYY-MM-DD 기준 점수」만 있었다. 축과 기간이 생긴 뒤로는 그 문장이
 * 목록의 절반만 설명한다: 같은 날짜에 여섯 가지 다른 순위가 나오기 때문이다.
 */
export function scoreRankingCaption(
  params: ScoreRankingParams,
  tradeDate: string,
): string {
  const axis = SCORE_RANKING_AXIS_OPTIONS.find((o) => o.id === params.axis)?.label ?? "종합";
  const base = tradeDate ? `${tradeDate} 기준` : "최근 배치 기준";
  if (params.duration === "realtime") return `${base} · ${axis} 점수 순`;
  const period =
    SCORE_RANKING_DURATION_OPTIONS.find((o) => o.id === params.duration)?.label ?? "";
  return `${base} · 최근 ${period} ${axis} 최고 점수 순`;
}
