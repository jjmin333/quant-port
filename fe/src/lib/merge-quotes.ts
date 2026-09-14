import type { QuoteSummary, WatchlistMetrics } from "@/lib/quotes-types";

function mergeWatchlistMetrics(prev: WatchlistMetrics, next: WatchlistMetrics): WatchlistMetrics {
  const merged: WatchlistMetrics = {
    ...prev,
    ...next,
    addedAt: next.addedAt || prev.addedAt,
    changeRatePrevClose: next.changeRatePrevClose ?? prev.changeRatePrevClose,
    prevClose: next.prevClose ?? prev.prevClose,
    changeRatePrevWeek: next.changeRatePrevWeek ?? prev.changeRatePrevWeek,
    prevWeekClose: next.prevWeekClose ?? prev.prevWeekClose,
    week52High: next.week52High ?? prev.week52High,
    week52Low: next.week52Low ?? prev.week52Low,
    changeRateWeek52High: next.changeRateWeek52High ?? prev.changeRateWeek52High,
    changeRateWeek52Low: next.changeRateWeek52Low ?? prev.changeRateWeek52Low,
  };

  if (next.prevMonthClose != null && next.prevMonthClose > 0) {
    merged.prevMonthClose = next.prevMonthClose;
    merged.changeRatePrevMonth = next.changeRatePrevMonth ?? null;
  } else {
    delete merged.prevMonthClose;
    delete merged.changeRatePrevMonth;
  }

  return merged;
}

/**
 * 종목명 자리에 이름이 아니라 **심볼이 들어온 것**인지.
 *
 * BE는 종목 메타 조회(`toss.GetStock`)에 실패하면 `zeroQuoteSummary`로 응답하는데,
 * 거기서 `Name`에 심볼을 그대로 넣는다(`be/internal/service/quotes_overlay.go`).
 * 빈 문자열이 아니라서 `!next.name` 검사를 통과해 버리고, 5초 폴링 중 한 번만 실패해도
 * 목록의 종목명이 **종목코드로 튀었다가 돌아온다**. 이름을 모른다는 뜻이므로 빈 값과
 * 똑같이 취급해 직전의 좋은 값을 지킨다.
 */
function isPlaceholderName(quote: QuoteSummary): boolean {
  const name = quote.name?.trim() ?? "";
  if (name === "") return true;
  return name.toUpperCase() === quote.symbol.trim().toUpperCase();
}

/** Keep last-known-good fields when a poll returns partial or empty quote data. */
export function mergeQuoteSummary(prev: QuoteSummary, next: QuoteSummary): QuoteSummary {
  const out: QuoteSummary = { ...prev, ...next };

  if (next.currentPrice <= 0 && prev.currentPrice > 0) {
    out.currentPrice = prev.currentPrice;
    out.change = prev.change;
    out.changeRate = prev.changeRate;
  }

  if (next.watchlistMetrics && prev.watchlistMetrics) {
    out.watchlistMetrics = mergeWatchlistMetrics(prev.watchlistMetrics, next.watchlistMetrics);
  } else if (!next.watchlistMetrics && prev.watchlistMetrics) {
    out.watchlistMetrics = prev.watchlistMetrics;
  }

  if (isPlaceholderName(next) && !isPlaceholderName(prev)) out.name = prev.name;
  if (!next.market && prev.market) out.market = prev.market;
  if (!next.currency && prev.currency) out.currency = prev.currency;
  if (!next.assetType && prev.assetType) out.assetType = prev.assetType;
  if (!next.logoUrl && prev.logoUrl) out.logoUrl = prev.logoUrl;

  return out;
}

/** Merge watchlist/held rows in list order — drops symbols no longer in `symbols`. */
export function mergeQuoteSummariesForSymbols(
  symbols: string[],
  prev: QuoteSummary[],
  next: QuoteSummary[],
): QuoteSummary[] {
  const prevBy = new Map(prev.map((q) => [q.symbol, q]));
  const nextBy = new Map(next.map((q) => [q.symbol, q]));
  const merged: QuoteSummary[] = [];

  for (const symbol of symbols) {
    const n = nextBy.get(symbol);
    const p = prevBy.get(symbol);
    if (!n && !p) continue;
    if (!n) {
      merged.push(p!);
      continue;
    }
    if (!p) {
      merged.push(n);
      continue;
    }
    merged.push(mergeQuoteSummary(p, n));
  }

  return merged;
}

/** Merge catalog quote lists by symbol (union of prev and next). */
export function mergeQuoteSummaries(prev: QuoteSummary[], next: QuoteSummary[]): QuoteSummary[] {
  const prevBy = new Map(prev.map((q) => [q.symbol, q]));
  const merged: QuoteSummary[] = [];
  const seen = new Set<string>();

  for (const n of next) {
    seen.add(n.symbol);
    const p = prevBy.get(n.symbol);
    merged.push(p ? mergeQuoteSummary(p, n) : n);
  }

  for (const p of prev) {
    if (!seen.has(p.symbol)) merged.push(p);
  }

  return merged;
}

/**
 * 스트림이 준 가격을 시세 목록에 덮는다 — 보유·관심이 함께 쓴다.
 *
 * # 기준가를 새로 받지 않는다
 *
 * `change`(전일 대비 금액)와 `currentPrice`가 이미 있으므로 **기준가는 그 둘의
 * 차**다. 세션 안에서 변하지 않는 값이라 마지막 REST 응답의 것을 그대로 쓰면 되고,
 * 그래서 스트림은 가격만 실어 보낸다(서버가 종목마다 기준가를 푸는 비용은
 * `use-quote-stream` 머리글의 실측 참조).
 *
 * `currentPrice`가 0이면 기준가를 못 낸다 — 그 행은 **건드리지 않는다.** 가격만
 * 갈아 끼우고 등락률을 옛 값으로 두면 숫자 둘이 서로 다른 순간을 가리킨다.
 *
 * # 같은 배열·같은 객체를 지킨다
 *
 * 값이 안 바뀐 행은 같은 객체를, 하나도 안 바뀌었으면 같은 배열을 돌려준다.
 * 보유 화면은 이 목록으로 평가손익을 다시 계산하므로, 참조가 매번 바뀌면 2초마다
 * 전체 재계산 + 전체 리렌더가 된다.
 */
export function applyStreamPricesToQuotes(
  quotes: QuoteSummary[],
  stream: Map<string, { price: number }>,
): QuoteSummary[] {
  if (stream.size === 0) return quotes;

  let changed = false;
  const next = quotes.map((q) => {
    const tick = stream.get(q.symbol.toUpperCase());
    if (!tick || !(tick.price > 0)) return q;
    if (q.currentPrice <= 0) return q;
    if (q.currentPrice === tick.price) return q;

    const base = q.currentPrice - q.change;
    if (!(base > 0)) return q;

    changed = true;
    return {
      ...q,
      currentPrice: tick.price,
      change: tick.price - base,
      changeRate: ((tick.price - base) / base) * 100,
    };
  });
  return changed ? next : quotes;
}
