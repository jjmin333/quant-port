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
    return `₩${(value / 100_000_000).toFixed(1)}억`;
  }
  return `₩${formatAmount(value)}`;
}

export function formatDateKST(isoOrDate: string): string {
  const d = new Date(isoOrDate.includes("T") ? isoOrDate : `${isoOrDate}T00:00:00+09:00`);
  if (Number.isNaN(d.getTime())) return isoOrDate;
  if (isoOrDate.includes("T")) {
    return d.toLocaleString("ko-KR", {
      month: "short",
      day: "numeric",
      hour: "2-digit",
      minute: "2-digit",
    });
  }
  return d.toLocaleDateString("ko-KR", { year: "numeric", month: "short", day: "numeric" });
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
  return `₩${formatAmount(Math.round(value))}`;
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
    return `${value > 0 ? "+" : ""}${formatAmount(value)}`;
  }
  const usd = `${sign}$${formatUSD(abs)}`;
  if (exchangeRate && exchangeRate > 0) {
    const krwSign = value >= 0 ? "+" : "-";
    return `${usd} (${krwSign}${formatAmount(Math.round(abs * exchangeRate))})`;
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
  if (value >= 1_000_000_000_000) return `${(value / 1_000_000_000_000).toFixed(1)}조`;
  if (value >= 100_000_000) return `${(value / 100_000_000).toFixed(0)}억`;
  return formatCompactAmount(value);
}

export function formatMarketCap(value: number, currency: "KRW" | "USD", exchangeRate?: number): string {
  if (value <= 0) return "—";
  return formatTradingValue(value, currency, exchangeRate);
}
