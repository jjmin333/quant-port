import type { Candle, CandleInterval } from "@/lib/quotes-types";

export function isDailyCandleInterval(interval: CandleInterval): boolean {
  return interval === "1d" || interval === "1w" || interval === "1M";
}

export function isIntradayInterval(interval: CandleInterval): boolean {
  return !isDailyCandleInterval(interval);
}

function pad2(value: number): string {
  return String(value).padStart(2, "0");
}

export type ChartVisibleSpan = {
  /** Visible time range in seconds */
  spanSeconds: number;
};

export function timeToEpochSeconds(time: number | string | { year: number; month: number; day: number }): number {
  if (typeof time === "number") return time;
  if (typeof time === "string") return new Date(`${time}T00:00:00+09:00`).getTime() / 1000;
  return new Date(time.year, time.month - 1, time.day).getTime() / 1000;
}

/** Adaptive bottom-axis labels based on zoom level */
export function formatAdaptiveChartAxisTime(
  epochSeconds: number,
  interval: CandleInterval,
  visible: ChartVisibleSpan,
): string {
  const date = new Date(epochSeconds * 1000);
  const y = date.getFullYear();
  const m = pad2(date.getMonth() + 1);
  const d = pad2(date.getDate());
  const hh = pad2(date.getHours());
  const mm = pad2(date.getMinutes());
  const span = visible.spanSeconds;

  if (isDailyCandleInterval(interval)) {
    if (span > 365 * 24 * 3600) return `${y}`;
    if (span > 45 * 24 * 3600) return `${y}-${m}`;
    return `${y}-${m}-${d}`;
  }

  const oneDay = 24 * 3600;
  if (span <= oneDay * 1.2) {
    return `${hh}:${mm}`;
  }
  if (span <= oneDay * 10) {
    return `${m}-${d}`;
  }
  if (span <= oneDay * 120) {
    return `${y}-${m}-${d}`;
  }
  return `${y}-${m}`;
}

export function formatChartCrosshairTime(epochSeconds: number, interval: CandleInterval): string {
  const date = new Date(epochSeconds * 1000);
  const y = date.getFullYear();
  const m = pad2(date.getMonth() + 1);
  const d = pad2(date.getDate());
  if (isDailyCandleInterval(interval)) {
    return `${y}-${m}-${d}`;
  }
  const hh = pad2(date.getHours());
  const mm = pad2(date.getMinutes());
  return `${y}-${m}-${d} ${hh}:${mm}`;
}

export function formatChartCrosshairBusinessDay(
  year: number,
  month: number,
  day: number,
): string {
  return `${year}-${pad2(month)}-${pad2(day)}`;
}

export function formatChartCrosshairFromRaw(raw: string, interval: CandleInterval): string {
  const trimmed = raw.trim();
  if (/^\d{4}-\d{2}-\d{2}$/.test(trimmed)) {
    return trimmed;
  }
  if (/^\d{4}-\d{2}-\d{2}[ T]\d{2}:\d{2}/.test(trimmed)) {
    if (isDailyCandleInterval(interval)) {
      return trimmed.slice(0, 10);
    }
    return `${trimmed.slice(0, 10)} ${trimmed.slice(11, 16)}`;
  }
  const date = new Date(trimmed);
  if (Number.isNaN(date.getTime())) return trimmed;
  return formatChartCrosshairTime(Math.floor(date.getTime() / 1000), interval);
}

export function mergeCandlesByTime(
  existing: Candle[],
  incoming: Candle[],
  interval: CandleInterval = "5m",
  /** When true (default), incoming overwrites existing on duplicate timestamps (SSE/live). */
  preferIncoming = true,
): Candle[] {
  const map = new Map<string, Candle>();
  const primary = preferIncoming ? incoming : existing;
  const secondary = preferIncoming ? existing : incoming;
  for (const candle of primary) {
    map.set(candleTimeSortKey(candle.time, interval), candle);
  }
  for (const candle of secondary) {
    map.set(candleTimeSortKey(candle.time, interval), candle);
  }
  return Array.from(map.values()).sort((a, b) =>
    candleTimeSortKey(a.time, interval).localeCompare(candleTimeSortKey(b.time, interval)),
  );
}

/** Canonical sort key — avoids duplicate bars when formats differ (RFC3339 vs space). */
export function candleTimeSortKey(raw: string, interval: CandleInterval): string {
  const trimmed = raw.trim();
  if (isDailyCandleInterval(interval)) {
    if (/^\d{4}-\d{2}-\d{2}/.test(trimmed)) {
      return trimmed.slice(0, 10);
    }
    const date = parseChartCandleDate(trimmed);
    return date ? date.slice(0, 10) : trimmed;
  }
  const epoch = parseChartCandleEpoch(trimmed);
  return epoch > 0 ? String(epoch) : trimmed;
}

function parseChartCandleDate(raw: string): string | null {
  if (/^\d{4}-\d{2}-\d{2}$/.test(raw)) return raw;
  if (/^\d{4}-\d{2}-\d{2}[ T]/.test(raw)) {
    return raw.slice(0, 10);
  }
  const date = new Date(raw.includes("T") || raw.includes("+") ? raw : `${raw.replace(" ", "T")}+09:00`);
  if (Number.isNaN(date.getTime())) return null;
  const y = date.getFullYear();
  const m = pad2(date.getMonth() + 1);
  const d = pad2(date.getDate());
  return `${y}-${m}-${d}`;
}

/** Parse candle time to UTC epoch seconds (KST wall clock). */
export function parseChartCandleEpoch(raw: string): number {
  const trimmed = raw.trim();
  if (/^\d{2}:\d{2}(:\d{2})?$/.test(trimmed)) {
    const now = new Date();
    const [h, m, s = "0"] = trimmed.split(":");
    const date = new Date(
      `${now.getFullYear()}-${pad2(now.getMonth() + 1)}-${pad2(now.getDate())}T${h}:${m}:${s}+09:00`,
    );
    return Math.floor(date.getTime() / 1000);
  }
  if (/^\d{4}-\d{2}-\d{2}$/.test(trimmed)) {
    return Math.floor(new Date(`${trimmed}T00:00:00+09:00`).getTime() / 1000);
  }
  if (/^\d{4}-\d{2}-\d{2}[ T]\d{2}:\d{2}/.test(trimmed)) {
    const iso = trimmed.includes("T")
      ? trimmed.includes("+")
        ? trimmed
        : `${trimmed.replace(" ", "T")}+09:00`
      : `${trimmed.replace(" ", "T")}+09:00`;
    const date = new Date(iso);
    if (!Number.isNaN(date.getTime())) {
      return Math.floor(date.getTime() / 1000);
    }
  }
  const date = new Date(trimmed);
  if (Number.isNaN(date.getTime())) return 0;
  return Math.floor(date.getTime() / 1000);
}

export function toChartTime(raw: string, interval: CandleInterval): number | string {
  const trimmed = raw.trim();
  if (isDailyCandleInterval(interval)) {
    const day = parseChartCandleDate(trimmed);
    return day ?? trimmed.slice(0, 10);
  }
  return parseChartCandleEpoch(trimmed) as number;
}

export function computeVisibleSpan(
  ohlcTimes: Array<number | string | { year: number; month: number; day: number }>,
  from: number,
  to: number,
): ChartVisibleSpan {
  if (ohlcTimes.length === 0) {
    return { spanSeconds: 0 };
  }
  const fromIdx = Math.max(0, Math.ceil(from));
  const toIdx = Math.min(ohlcTimes.length - 1, Math.floor(to));
  if (fromIdx > toIdx) {
    return { spanSeconds: 0 };
  }
  const start = timeToEpochSeconds(ohlcTimes[fromIdx]!);
  const end = timeToEpochSeconds(ohlcTimes[toIdx]!);
  return { spanSeconds: Math.max(end - start, 0) };
}
