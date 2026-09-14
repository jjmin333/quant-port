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
  /*
    밸류에이션도 **다른 탭과 같은 배지**를 쓴다 (2026-09-03 사용자 결정).

    한동안 여기서 빠져 있었다 — 그 탭의 「종합 지표」 카드가 가치 점수를 자기 머리줄에
    직접 적고 있었기 때문이다. 그래서 같은 앱에서 「수익성 점수」는 막대 달린 배지로,
    「가치 점수」는 카드 안 글자로 나왔다. 점수를 읽는 자리는 탭마다 같아야 한다 —
    카드 쪽 글자를 지우고 이 표에 넣었다.
  */
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

/**
 * 종합 점수 — **하루 한 번 도는 배치**가 만든다. 화면이 다시 물어도 같은 값이다.
 * 워크스페이스의 여러 탭이 각자 축 점수를 그리므로 같은 순간에 여러 번 불린다.
 */
export function fetchStockScoreClient(symbol: string): Promise<StockScoreData | null> {
  const path = `/api/v1/stocks/${encodeURIComponent(symbol)}/score`;
  return cachedGet(
    path,
    () => apiRequest<StockScoreData | null>(path, undefined, "점수 조회 실패"),
    60_000,
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
  /**
   * 등락률·가격의 출처. `"live"`(지금 시세) 또는 종가의 거래일(`YYYY-MM-DD`).
   *
   * 서버가 정한다 — 프리·정규·애프터 중이면 실시간 시세로 덮고, 애프터마켓까지
   * 끝난 뒤에는 확정 종가를 쓴다. 화면은 **툴팁 문구를 고르는 데만** 쓴다:
   * 이 값으로 색을 빼거나 각주를 달지 않는다(그러면 정상 상태인 해외 종목이
   * 언제나 「밀린 값」으로 보인다 — 점수의 날은 KST, 미국 종가는 하루 뒤다).
   */
  priceAsOf?: string;
  /**
   * 이 목록이 **실제로 줄 세운 값**. 축을 고르면 그 축 점수이고, 기간을 고르면 그
   * 기간의 최고점이다. 종합·실시간에서는 `score`와 같다.
   *
   * 행에 적히는 숫자는 이 값이다 — `score`(최신일 종합)를 그리면 축 랭킹에서
   * 목록이 뒤죽박죽 정렬된 것처럼 읽힌다.
   */
  rankValue?: number;
  /**
   * 등락률의 **기준가**. 서버가 고른 값이다(`scorePrevCloseHint`).
   *
   * 목록 가격이 SSE로 오기 때문에 필요하다 — 스트림은 가격만 보내고, 등락률은 이
   * 값으로 화면이 낸다(`changeRateFrom`). 기준가를 서버가 틱마다 풀면 종목당 4~5
   * SELECT가 붙어서(실측: 이 응답 20줄이 100 SELECT였다) 푸시로 옮긴 뜻이 없어진다.
   *
   * 없으면 등락률을 **그리지 않는다.** 0.00%로 때우면 「보합」으로 읽히는데 실제
   * 뜻은 「기준가를 모른다」다.
   */
  basePrice?: number;
};

export type ScoreRankingMarket = "domestic" | "overseas";

export type ScoreRankingQuery = {
  market: ScoreRankingMarket;
  /** `lib/score-ranking-config`의 축. 생략하면 종합. */
  axis?: string;
  /** 같은 파일의 기간. 생략하면 최신 배치일. */
  duration?: string;
  limit?: number;
};

/**
 * 축·기간별 랭킹.
 *
 * 종합·실시간을 뺀 조합은 **배치가 미리 만들어 둔 목록**이다(BE
 * `service/stock_score_rankings.go`) — 기간 창을 훑는 계산은 운영 DB에서 수 초라
 * 요청 경로에 둘 수 없다. 그래서 조합을 바꿔도 응답 시간은 종합·실시간과 같다.
 */
export async function fetchScoreRankingsClient({
  market,
  axis,
  duration,
  limit = 20,
}: ScoreRankingQuery): Promise<{ items: StockScoreListItem[]; tradeDate: string }> {
  const params = new URLSearchParams({ market, limit: String(limit) });
  if (axis && axis !== "composite") params.set("axis", axis);
  if (duration && duration !== "realtime") params.set("duration", duration);
  const path = `/api/v1/stocks/scores/top?${params.toString()}`;
  /*
   * 순위·점수는 하루 한 번 도는 배치가 만들지만 **등락률은 장중에 서버가 실시간
   * 시세로 덮는다.** 그래서 예전의 5분 캐시는 너무 길다 — 토글을 오가는 동안의
   * 중복 요청만 걷어내면 되므로 30초로 줄인다.
   */
  const items = await cachedGet(
    path,
    () =>
      apiRequest<StockScoreListItem[] | null>(path, undefined, "종목 랭킹 조회 실패"),
    30_000,
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

/**
 * 랭킹 등락률 칸의 툴팁 문구.
 *
 * 화면에는 숫자만 두고 출처는 여기로 보낸다 — 여덟 줄에 날짜를 일일이 적으면
 * 목록이 날짜 표가 되고, 색으로 구분하면 색이 방향 말고 다른 것도 뜻하게 된다.
 */
export function priceAsOfLabel(priceAsOf?: string): string | undefined {
  if (!priceAsOf) return undefined;
  if (priceAsOf === "live") return "실시간 시세 기준";
  return `${priceAsOf} 종가 기준`;
}
