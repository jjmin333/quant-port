export function formatAmount(value: number): string {
  return new Intl.NumberFormat("ko-KR", {
    maximumFractionDigits: 0,
  }).format(value);
}

/** 좁은 카드용 축약 표기 (만/억 단위) */
export function formatCompactAmount(value: number): string {
  const abs = Math.abs(value);
  if (abs >= 100_000_000) {
    const eok = abs / 100_000_000;
    return `${Number.isInteger(eok) ? eok : eok.toFixed(1)}억`;
  }
  if (abs >= 10_000) {
    const man = abs / 10_000;
    return `${Number.isInteger(man) ? man : man.toFixed(1)}만`;
  }
  return formatAmount(abs);
}

/** 차트 툴팁 등 문자열이 필요한 곳에서 사용 */
export function formatKRW(value: number, compact = false): string {
  if (value <= 0) return "—";
  if (compact && Math.abs(value) >= 100_000_000) {
    return `${(value / 100_000_000).toFixed(1)}억원`;
  }
  return `${formatAmount(value)}원`;
}

/**
 * Full date+time in KST, e.g. "2026. 8. 8. 오후 3:42:10". Use for anything
 * timestamp-like shown to the user (admin run times, generatedAt, last-update
 * lines) — always pass timeZone explicitly rather than relying on the host's
 * local zone, which is not guaranteed to be Asia/Seoul (SSR runs in whatever
 * zone the server container has, commonly UTC).
 */
export function formatDateTimeKST(value: string | number | Date): string {
  const d = value instanceof Date ? value : new Date(value);
  if (Number.isNaN(d.getTime())) return String(value);
  return d.toLocaleString("ko-KR", { timeZone: "Asia/Seoul" });
}

/** HH:MM in KST — for "n분 전 갱신" style live-update timestamps. */
export function formatTimeKST(value: string | number | Date): string {
  const d = value instanceof Date ? value : new Date(value);
  if (Number.isNaN(d.getTime())) return String(value);
  return d.toLocaleTimeString("ko-KR", {
    timeZone: "Asia/Seoul",
    hour: "2-digit",
    minute: "2-digit",
  });
}

export function formatDateKST(isoOrDate: string): string {
  const d = new Date(isoOrDate.includes("T") ? isoOrDate : `${isoOrDate}T00:00:00+09:00`);
  if (Number.isNaN(d.getTime())) return isoOrDate;
  // timeZone must be explicit here — despite the name, this previously relied
  // on the browser's own local zone matching KST, which silently breaks for
  // anyone (or any SSR host) not set to Asia/Seoul.
  if (isoOrDate.includes("T")) {
    return d.toLocaleString("ko-KR", {
      timeZone: "Asia/Seoul",
      month: "short",
      day: "numeric",
      hour: "2-digit",
      minute: "2-digit",
    });
  }
  return d.toLocaleDateString("ko-KR", {
    timeZone: "Asia/Seoul",
    year: "numeric",
    month: "short",
    day: "numeric",
  });
}

/**
 * "2시간 전" 꼴의 상대 시각. 위젯 목록(뉴스·리포트)처럼 좁은 칸에서 절대 시각보다
 * 짧고, 최신인지 아닌지를 바로 읽게 한다. 하루가 넘으면 일 단위로만 말한다.
 */
export function formatRelativeTime(isoOrDate: string): string {
  const then = new Date(isoOrDate).getTime();
  if (!Number.isFinite(then)) return "";
  const mins = Math.floor((Date.now() - then) / 60000);
  if (mins < 1) return "방금 전";
  if (mins < 60) return `${mins}분 전`;
  const hours = Math.floor(mins / 60);
  if (hours < 24) return `${hours}시간 전`;
  return `${Math.floor(hours / 24)}일 전`;
}

/** 상장일 — 백엔드가 YYYY-MM-DD로 내려와도 연도만 표시 */
export function formatListDate(isoOrDate: string): string {
  const trimmed = isoOrDate.trim();
  const match = trimmed.match(/^(\d{4})(?:-\d{2}-\d{2})?/);
  if (match) return match[1];

  const d = new Date(trimmed.includes("T") ? trimmed : `${trimmed}T00:00:00+09:00`);
  if (Number.isNaN(d.getTime())) return trimmed;
  return String(d.getFullYear());
}

export function formatPercent(value: number, signed = true): string {
  const formatted = `${Math.abs(value).toFixed(2)}%`;
  if (!signed) return formatted;
  if (value > 0) return `+${formatted}`;
  if (value < 0) return `-${formatted}`;
  return formatted;
}

/** 전일 종가 등 기준이 없을 때 등락률 표시 생략 */
export function formatChangeRate(changeRate: number, hasBaseline: boolean): string {
  if (!hasBaseline) return "—";
  return formatPercent(changeRate);
}

export function formatNumber(value: number): string {
  return formatAmount(value);
}

export function formatUSD(value: number, decimals = 2): string {
  return new Intl.NumberFormat("en-US", {
    minimumFractionDigits: decimals,
    maximumFractionDigits: decimals,
  }).format(value);
}

export type PriceFormatOptions = {
  exchangeRate?: number;
  /** Explicit KRW amount; overrides exchangeRate conversion when set. */
  krwValue?: number;
};

function formatKRWAmount(value: number): string {
  return `${formatAmount(Math.round(value))}원`;
}

/** UI 표시용 — KRW/USD 모두 PriceDisplay 컴포넌트 사용 권장. 툴팁·복사·비UI 문자열용. */
export function formatPrice(
  value: number,
  currency: "KRW" | "USD",
  options?: PriceFormatOptions,
): string {
  if (value <= 0) return "—";
  if (currency === "KRW") return formatKRWAmount(value);

  const usd = `$${formatUSD(value)}`;
  const krw =
    options?.krwValue ??
    (options?.exchangeRate && options.exchangeRate > 0 ? value * options.exchangeRate : 0);
  if (krw > 0) return `${usd} (${formatKRWAmount(krw)})`;
  return usd;
}

export function formatPriceChange(value: number, currency: "KRW" | "USD", exchangeRate?: number): string {
  if (value === 0) return "0";
  const sign = value > 0 ? "+" : value < 0 ? "-" : "";
  const abs = Math.abs(value);
  if (currency === "KRW") {
    return `${sign}${formatAmount(abs)}원`;
  }
  const usd = `${sign}$${formatUSD(abs)}`;
  if (exchangeRate && exchangeRate > 0) {
    const krwSign = value >= 0 ? "+" : "-";
    return `${usd} (${krwSign}${formatAmount(Math.round(abs * exchangeRate))}원)`;
  }
  return usd;
}

export function formatVolume(value: number): string {
  if (value <= 0) return "—";
  if (value >= 1_000_000) return `${(value / 1_000_000).toFixed(1)}M`;
  if (value >= 1_000) return `${(value / 1_000).toFixed(1)}K`;
  return formatAmount(value);
}

export function formatTradingValue(value: number, currency: "KRW" | "USD", exchangeRate?: number): string {
  if (value <= 0) return "—";
  if (currency === "USD") {
    let usdPart: string;
    if (value >= 1_000_000_000) usdPart = `$${(value / 1_000_000_000).toFixed(1)}B`;
    else if (value >= 1_000_000) usdPart = `$${(value / 1_000_000).toFixed(1)}M`;
    else usdPart = `$${formatUSD(value, 0)}`;
    if (exchangeRate && exchangeRate > 0) {
      return `${usdPart} (${formatKRWAmount(value * exchangeRate)})`;
    }
    return usdPart;
  }
  if (value >= 1_000_000_000_000) return `${(value / 1_000_000_000_000).toFixed(1)}조원`;
  if (value >= 100_000_000) return `${(value / 100_000_000).toFixed(0)}억원`;
  return `${formatCompactAmount(value)}원`;
}

/**
 * Financial-statement amounts, scaled to the listing's own currency.
 *
 * Separate from formatTradingValue for two reasons. Statement lines are
 * routinely negative — an operating loss is a real figure — and
 * formatTradingValue renders anything <= 0 as "—", which silently hid losses.
 * And market-data amounts are always positive turnover, so widening that
 * function would change unrelated call sites.
 */
export function formatFinancialAmount(
  value: number | null | undefined,
  currency: "KRW" | "USD",
): string {
  if (value == null || !Number.isFinite(value)) return "—";
  if (value === 0) return currency === "USD" ? "$0" : "0원";

  const sign = value < 0 ? "-" : "";
  const abs = Math.abs(value);

  if (currency === "USD") {
    if (abs >= 1_000_000_000) return `${sign}$${(abs / 1_000_000_000).toFixed(1)}B`;
    if (abs >= 1_000_000) return `${sign}$${(abs / 1_000_000).toFixed(1)}M`;
    if (abs >= 1_000) return `${sign}$${(abs / 1_000).toFixed(1)}K`;
    return `${sign}$${abs.toFixed(0)}`;
  }

  if (abs >= 1_000_000_000_000) return `${sign}${(abs / 1_000_000_000_000).toFixed(1)}조원`;
  if (abs >= 100_000_000) return `${sign}${(abs / 100_000_000).toFixed(0)}억원`;
  if (abs >= 10_000) return `${sign}${(abs / 10_000).toFixed(0)}만원`;
  return `${sign}${formatAmount(abs)}원`;
}

/** Compact axis tick — same scaling, no currency word, to keep ticks narrow. */
export function formatFinancialTick(value: number, currency: "KRW" | "USD"): string {
  if (!Number.isFinite(value) || value === 0) return "0";
  const sign = value < 0 ? "-" : "";
  const abs = Math.abs(value);
  if (currency === "USD") {
    if (abs >= 1_000_000_000) return `${sign}${(abs / 1_000_000_000).toFixed(0)}B`;
    if (abs >= 1_000_000) return `${sign}${(abs / 1_000_000).toFixed(0)}M`;
    if (abs >= 1_000) return `${sign}${(abs / 1_000).toFixed(0)}K`;
    return `${sign}${abs.toFixed(0)}`;
  }
  if (abs >= 1_000_000_000_000) return `${sign}${(abs / 1_000_000_000_000).toFixed(1)}조`;
  if (abs >= 100_000_000) return `${sign}${(abs / 100_000_000).toFixed(0)}억`;
  if (abs >= 10_000) return `${sign}${(abs / 10_000).toFixed(0)}만`;
  return `${sign}${abs.toFixed(0)}`;
}

export function formatMarketCap(value: number, currency: "KRW" | "USD", exchangeRate?: number): string {
  if (value <= 0) return "—";
  return formatTradingValue(value, currency, exchangeRate);
}
