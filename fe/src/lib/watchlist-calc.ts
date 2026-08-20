import type { QuoteSummary, WatchlistItem } from "@/lib/quotes-types";

/**
 * 관심종목 목록의 파생 값 — 행이 직접 계산하지 않게 한 곳에 모은다.
 *
 * 정렬·필터·요약 카드가 모두 같은 숫자를 봐야 한다. 행에서 한 번, 레일 요약에서 또
 * 한 번 계산하면 둘이 조용히 달라진다(52주 위치 clamp를 한쪽만 하는 식으로).
 */

export type WatchlistSort = "manual" | "change" | "range" | "since" | "score";
export type WatchlistMarketFilter = "all" | "domestic" | "us";
export type WatchlistTrendFilter = "all" | "up" | "down";

export const WATCHLIST_SORT_LABELS: Record<WatchlistSort, string> = {
  manual: "내 순서",
  change: "등락률",
  range: "52주 위치",
  since: "담은 뒤",
  score: "점수",
};

export type WatchlistRow = {
  symbol: string;
  quote?: QuoteSummary;
  /** 종합 점수. 산출된 적이 없으면 null. */
  score: number | null;
  scoreGrade?: string;
  /** 52주 저가 0 · 고가 100 기준의 현재가 위치(0~100). 기준이 없으면 null. */
  rangePct: number | null;
  /** 담은 가격 대비 수익률(%). `addedPrice`가 없는(옛) 종목은 null. */
  sinceAddedRate: number | null;
  addedPrice: number | null;
  addedAt: string | null;
};

/**
 * 52주 레인지 안에서 현재가의 위치.
 *
 * 장중 신고가·신저가면 100을 넘거나 0 아래로 내려간다 — 마커가 트랙 밖으로 튀어나가지
 * 않도록 잘라 둔다. 값 자체는 "레인지 상단 100%"로 읽히면 충분하다.
 */
export function week52RangePct(
  price: number | null | undefined,
  low: number | null | undefined,
  high: number | null | undefined,
): number | null {
  if (price == null || !Number.isFinite(price) || price <= 0) return null;
  if (low == null || high == null) return null;
  if (!Number.isFinite(low) || !Number.isFinite(high)) return null;
  if (low <= 0 || high <= 0 || high <= low) return null;
  const pct = ((price - low) / (high - low)) * 100;
  return Math.min(100, Math.max(0, pct));
}

/** 담은 가격 대비 수익률(%). */
export function sinceAddedRate(
  price: number | null | undefined,
  addedPrice: number | null | undefined,
): number | null {
  if (price == null || !Number.isFinite(price) || price <= 0) return null;
  if (addedPrice == null || !Number.isFinite(addedPrice) || addedPrice <= 0) return null;
  return ((price - addedPrice) / addedPrice) * 100;
}

export function buildWatchlistRows(
  items: WatchlistItem[],
  quotes: QuoteSummary[],
  // ReadonlyMap이라야 `Map<string, StockScoreData>`를 그대로 넘길 수 있다 —
  // 쓰기 가능한 Map은 값 타입이 불변(invariant)이라 더 넓은 타입을 거부한다.
  scores: ReadonlyMap<string, { score: number | null; grade?: string }>,
): WatchlistRow[] {
  const quoteBySymbol = new Map(quotes.map((q) => [q.symbol, q]));
  return items.map((item) => {
    const quote = quoteBySymbol.get(item.symbol);
    const metrics = quote?.watchlistMetrics;
    const scored = scores.get(item.symbol.toUpperCase());
    const addedPrice = item.addedPrice != null && item.addedPrice > 0 ? item.addedPrice : null;
    return {
      symbol: item.symbol,
      quote,
      score: scored?.score ?? null,
      scoreGrade: scored?.grade,
      rangePct: week52RangePct(quote?.currentPrice, metrics?.week52Low, metrics?.week52High),
      sinceAddedRate: sinceAddedRate(quote?.currentPrice, addedPrice),
      addedPrice,
      addedAt: metrics?.addedAt ?? item.createdAt ?? null,
    };
  });
}

/** 미국 종목인가 — ETF는 assetType이 국내·미국을 구분하지 않아 통화로 가른다. */
export function isUsRow(row: WatchlistRow): boolean {
  return row.quote?.currency === "USD";
}

export function filterWatchlistRows(
  rows: WatchlistRow[],
  market: WatchlistMarketFilter,
  trend: WatchlistTrendFilter,
): WatchlistRow[] {
  return rows.filter((row) => {
    if (market === "us" && !isUsRow(row)) return false;
    if (market === "domestic" && (isUsRow(row) || !row.quote)) return false;
    if (trend !== "all") {
      const rate = row.quote?.changeRate;
      // 시세를 아직 못 받은 종목은 상승·하락 어느 쪽도 아니다. 0으로 취급하면
      // 로딩 중인 종목이 전부 "상승"으로 잡힌다.
      if (rate == null || !(row.quote && row.quote.currentPrice > 0)) return false;
      if (trend === "up" && !(rate > 0)) return false;
      if (trend === "down" && !(rate < 0)) return false;
    }
    return true;
  });
}

/**
 * 값이 없는 행은 언제나 뒤로. 점수·담은 뒤는 비어 있는 종목이 흔한데, null을 0으로
 * 두면 "점수 없음"이 "점수 0점"과 같은 자리에 서게 된다.
 */
function byDescNullsLast(a: number | null, b: number | null): number {
  if (a == null && b == null) return 0;
  if (a == null) return 1;
  if (b == null) return -1;
  return b - a;
}

export function sortWatchlistRows(rows: WatchlistRow[], sort: WatchlistSort): WatchlistRow[] {
  if (sort === "manual") return rows;
  const next = [...rows];
  next.sort((a, b) => {
    switch (sort) {
      case "change":
        return byDescNullsLast(
          a.quote && a.quote.currentPrice > 0 ? a.quote.changeRate : null,
          b.quote && b.quote.currentPrice > 0 ? b.quote.changeRate : null,
        );
      case "range":
        return byDescNullsLast(a.rangePct, b.rangePct);
      case "since":
        return byDescNullsLast(a.sinceAddedRate, b.sinceAddedRate);
      case "score":
        return byDescNullsLast(a.score, b.score);
      default:
        return 0;
    }
  });
  return next;
}

export type SinceAddedStats = {
  count: number;
  average: number;
  winners: number;
  losers: number;
  best: WatchlistRow;
  worst: WatchlistRow;
  /** 평균 보유(관심) 일수. `addedAt`이 없는 종목은 뺀다. */
  avgHoldDays: number | null;
};

export function sinceAddedStats(rows: WatchlistRow[]): SinceAddedStats | null {
  const scored = rows.filter((r) => r.sinceAddedRate != null);
  if (scored.length === 0) return null;

  const sorted = [...scored].sort((a, b) => (b.sinceAddedRate ?? 0) - (a.sinceAddedRate ?? 0));
  const sum = scored.reduce((acc, r) => acc + (r.sinceAddedRate ?? 0), 0);

  const days: number[] = [];
  for (const row of scored) {
    if (!row.addedAt) continue;
    const t = new Date(row.addedAt).getTime();
    if (!Number.isFinite(t)) continue;
    days.push(Math.max(0, Math.floor((Date.now() - t) / 86_400_000)));
  }

  return {
    count: scored.length,
    average: sum / scored.length,
    winners: scored.filter((r) => (r.sinceAddedRate ?? 0) > 0).length,
    losers: scored.filter((r) => (r.sinceAddedRate ?? 0) < 0).length,
    best: sorted[0],
    worst: sorted[sorted.length - 1],
    avgHoldDays: days.length > 0 ? days.reduce((a, b) => a + b, 0) / days.length : null,
  };
}
