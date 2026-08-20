import type { QuoteSummary } from "@/lib/quotes-types";
import type { UserHolding } from "@/lib/user-holdings-types";

/**
 * 보유종목 손익 계산 — 이 화면의 모든 숫자가 여기서 나온다.
 *
 * 왜 한 곳에 모았나: 평가금액·손익·비중·오늘 손익을 위젯마다 따로 계산하면 같은 화면에서
 * 합계가 서로 다르게 찍힌다(레일 요약과 표 합계가 어긋나는 식). 계산 규칙이 하나라야
 * "총 평가금액"이 화면 어디서든 같은 값을 뜻한다.
 *
 * 규칙 세 가지 — 전부 "모르는 값을 0으로 그리지 않는다"의 변형이다:
 *
 *  1. **평단가가 없는 종목은 손익이 `null`이다.** 스크린샷에서 평단을 못 읽었거나
 *     직접 입력하지 않은 종목이 있다. 그걸 0으로 두면 매입가 0원짜리를 들고 있는 셈이
 *     되어 수익률이 +∞로 뛴다. 합계에서도 그 종목은 분자·분모에서 함께 빠진다.
 *  2. **환율을 못 받았고 달러 종목이 있으면 원화 합계를 보류한다.** 일부만 더한 값을
 *     "총 평가금액"이라 부르면 틀린 숫자를 자신 있게 보여주는 꼴이다.
 *  3. **시세가 없는 종목은 평가금액이 `null`이다.** 수량만으로는 값을 매길 수 없다.
 *
 * ⚠️ 달러 종목의 **매입금액도 현재 환율로 환산**한다. 매입 시점 환율은 거래 이력에
 * `fxRate`로 넣을 수 있지만 대부분 비어 있어서, 환차손익을 분리해 내지 못한다.
 * 화면에 적용 환율을 적어두는 것이 지금 할 수 있는 정직한 처리다.
 */

export type HoldingRow = UserHolding & { quote?: QuoteSummary };

export type HoldingMetrics = {
  row: HoldingRow;
  symbol: string;
  name: string;
  currency: "KRW" | "USD";
  quantity: number;
  /** 시세가 없으면 0 — `hasQuote`로 먼저 거른다. */
  currentPrice: number;
  hasQuote: boolean;
  /** 종목 통화 기준. 모르면 null. */
  avgPrice: number | null;
  marketValue: number | null;
  marketValueKRW: number | null;
  costBasis: number | null;
  costBasisKRW: number | null;
  unrealizedPnl: number | null;
  unrealizedPnlKRW: number | null;
  /** % */
  returnRate: number | null;
  /** 전일대비 % */
  changeRate: number;
  dayPnlKRW: number | null;
  /** 총 평가금액 대비 %. 합계를 못 내면 null. */
  weight: number | null;
};

export type HoldingsTotals = {
  /** 원화 합계를 낼 수 있는가 (달러 종목이 있으면 환율 필요) */
  canTotal: boolean;
  marketValueKRW: number;
  dayPnlKRW: number;
  dayPnlRate: number;
  /** 평단이 있는 종목만의 매입금액. 하나도 없으면 null. */
  costBasisKRW: number | null;
  unrealizedPnlKRW: number | null;
  returnRate: number | null;
  count: number;
  pricedCount: number;
  withAvgCount: number;
  krwCount: number;
  usdCount: number;
  krwValueKRW: number;
  usdValueKRW: number;
  /** 손익을 못 내는(평단 없는) 종목 심볼 — 화면이 이유를 말할 수 있게 */
  missingAvgSymbols: string[];
  /** 수동 입력 현금 (원·달러) */
  cashKRW: number;
  cashUSD: number;
  /** 원화로 환산한 현금 합계. 달러 현금이 있으면 환율 필요. */
  cashTotalKRW: number;
  hasCash: boolean;
  /** 주식 평가 + 현금을 원화로 합산할 수 있는가 */
  canTotalAssets: boolean;
  /** canTotalAssets일 때 marketValueKRW + cashTotalKRW */
  totalAssetsKRW: number;
};

function resolveCurrency(row: HoldingRow): "KRW" | "USD" {
  if (row.quote?.currency) return row.quote.currency;
  const cost = (row.costCurrency ?? "").toUpperCase();
  return cost === "USD" ? "USD" : "KRW";
}

export function buildHoldingMetrics(rows: HoldingRow[], exchangeRate: number): HoldingMetrics[] {
  return rows.map((row) => {
    const quote = row.quote;
    const currency = resolveCurrency(row);
    const hasQuote = Boolean(quote && quote.currentPrice > 0);
    const currentPrice = quote?.currentPrice ?? 0;
    const fx = currency === "USD" ? exchangeRate : 1;
    const canConvert = currency !== "USD" || exchangeRate > 0;

    const avgPrice = row.avgPrice != null && row.avgPrice > 0 ? row.avgPrice : null;
    const marketValue = hasQuote ? currentPrice * row.quantity : null;
    const costBasis = avgPrice != null ? avgPrice * row.quantity : null;
    const unrealizedPnl =
      marketValue != null && costBasis != null ? marketValue - costBasis : null;

    const toKRW = (v: number | null) => (v == null || !canConvert ? null : v * fx);
    const dayChange = quote?.change ?? 0;

    return {
      row,
      symbol: row.symbol,
      name: row.name || quote?.name || row.symbol,
      currency,
      quantity: row.quantity,
      currentPrice,
      hasQuote,
      avgPrice,
      marketValue,
      marketValueKRW: toKRW(marketValue),
      costBasis,
      costBasisKRW: toKRW(costBasis),
      unrealizedPnl,
      unrealizedPnlKRW: toKRW(unrealizedPnl),
      returnRate:
        unrealizedPnl != null && costBasis != null && costBasis > 0
          ? (unrealizedPnl / costBasis) * 100
          : null,
      changeRate: quote?.changeRate ?? 0,
      dayPnlKRW: hasQuote ? toKRW(dayChange * row.quantity) : null,
      weight: null,
    };
  });
}

export function computeHoldingsTotals(metrics: HoldingMetrics[], exchangeRate: number): HoldingsTotals {
  const priced = metrics.filter((m) => m.hasQuote);
  const hasUsd = priced.some((m) => m.currency === "USD");
  const canTotal = priced.length > 0 && (!hasUsd || exchangeRate > 0);

  let marketValueKRW = 0;
  let dayPnlKRW = 0;
  let krwValueKRW = 0;
  let usdValueKRW = 0;
  // 매입금액과 그에 대응하는 평가금액을 함께 모은다. 수익률의 분모는 "평단을 아는
  // 종목의 매입금액"이어야 하므로, 평단 없는 종목의 평가금액이 분자에 섞이면 안 된다.
  let costBasisKRW = 0;
  let valueOfPricedWithAvgKRW = 0;
  let withAvgCount = 0;
  const missingAvgSymbols: string[] = [];

  for (const m of priced) {
    if (m.marketValueKRW == null) continue;
    marketValueKRW += m.marketValueKRW;
    dayPnlKRW += m.dayPnlKRW ?? 0;
    if (m.currency === "USD") usdValueKRW += m.marketValueKRW;
    else krwValueKRW += m.marketValueKRW;

    if (m.costBasisKRW != null) {
      costBasisKRW += m.costBasisKRW;
      valueOfPricedWithAvgKRW += m.marketValueKRW;
      withAvgCount += 1;
    } else {
      missingAvgSymbols.push(m.symbol);
    }
  }

  const prevTotal = marketValueKRW - dayPnlKRW;
  const unrealizedPnlKRW = withAvgCount > 0 ? valueOfPricedWithAvgKRW - costBasisKRW : null;

  return {
    canTotal,
    marketValueKRW,
    dayPnlKRW,
    dayPnlRate: prevTotal > 0 ? (dayPnlKRW / prevTotal) * 100 : 0,
    costBasisKRW: withAvgCount > 0 ? costBasisKRW : null,
    unrealizedPnlKRW,
    returnRate:
      unrealizedPnlKRW != null && costBasisKRW > 0 ? (unrealizedPnlKRW / costBasisKRW) * 100 : null,
    count: metrics.length,
    pricedCount: priced.length,
    withAvgCount,
    krwCount: priced.filter((m) => m.currency !== "USD").length,
    usdCount: priced.filter((m) => m.currency === "USD").length,
    krwValueKRW,
    usdValueKRW,
    missingAvgSymbols,
    cashKRW: 0,
    cashUSD: 0,
    cashTotalKRW: 0,
    hasCash: false,
    canTotalAssets: canTotal,
    totalAssetsKRW: canTotal ? marketValueKRW : 0,
  };
}

/** 주식 합계에 수동 입력 현금을 합친다. Toss 예수금과는 별개다. */
export function mergeCashIntoTotals(
  totals: HoldingsTotals,
  cash: { krw: number; usd: number },
  exchangeRate: number,
): HoldingsTotals {
  const cashKRW = Math.max(0, cash.krw ?? 0);
  const cashUSD = Math.max(0, cash.usd ?? 0);
  const hasCash = cashKRW > 0 || cashUSD > 0;
  const cashFxOk = cashUSD <= 0 || exchangeRate > 0;
  const stockFxOk = totals.pricedCount === 0 || totals.canTotal;
  const canTotalAssets = (hasCash || totals.pricedCount > 0) && stockFxOk && cashFxOk;
  const cashUsdKRW = cashUSD > 0 && exchangeRate > 0 ? cashUSD * exchangeRate : 0;
  const cashTotalKRW = cashKRW + cashUsdKRW;
  const stockKRW = totals.canTotal ? totals.marketValueKRW : 0;
  const totalAssetsKRW = canTotalAssets ? stockKRW + cashTotalKRW : 0;

  return {
    ...totals,
    cashKRW,
    cashUSD,
    cashTotalKRW,
    hasCash,
    canTotalAssets,
    totalAssetsKRW,
  };
}

/** 비중은 합계를 낸 뒤에야 알 수 있어 두 번째 패스로 채운다. */
export function withWeights(metrics: HoldingMetrics[], totals: HoldingsTotals): HoldingMetrics[] {
  const denom =
    totals.canTotalAssets && totals.totalAssetsKRW > 0
      ? totals.totalAssetsKRW
      : totals.canTotal && totals.marketValueKRW > 0
        ? totals.marketValueKRW
        : 0;
  if (denom <= 0) return metrics;
  return metrics.map((m) =>
    m.marketValueKRW == null ? m : { ...m, weight: (m.marketValueKRW / denom) * 100 },
  );
}

export type HoldingSortKey =
  | "name"
  | "quantity"
  | "avgPrice"
  | "price"
  | "change"
  | "value"
  | "pnl"
  | "weight";

/**
 * 정렬. 값이 없는 행(`null`)은 방향과 무관하게 **항상 아래로** 보낸다 —
 * 오름차순에서 `—`가 맨 위를 채우면 정렬한 의미가 사라진다.
 */
export function sortHoldingMetrics(
  metrics: HoldingMetrics[],
  key: HoldingSortKey,
  desc: boolean,
): HoldingMetrics[] {
  const value = (m: HoldingMetrics): number | string | null => {
    switch (key) {
      case "name":
        return m.name;
      case "quantity":
        return m.quantity;
      case "avgPrice":
        return m.avgPrice;
      case "price":
        return m.hasQuote ? m.currentPrice : null;
      case "change":
        return m.hasQuote ? m.changeRate : null;
      case "value":
        return m.marketValueKRW;
      case "pnl":
        return m.returnRate;
      case "weight":
        return m.weight;
    }
  };

  return [...metrics].sort((a, b) => {
    const va = value(a);
    const vb = value(b);
    if (va == null && vb == null) return 0;
    if (va == null) return 1;
    if (vb == null) return -1;
    if (typeof va === "string" || typeof vb === "string") {
      const cmp = String(va).localeCompare(String(vb), "ko");
      return desc ? -cmp : cmp;
    }
    return desc ? vb - va : va - vb;
  });
}
