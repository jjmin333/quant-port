import type { CandleInterval, ChartStyle } from "@/lib/quotes-types";

export type QuotesChartPrefs = {
  interval: CandleInterval;
  chartStyle: ChartStyle;
};

export const DEFAULT_QUOTES_CHART_PREFS: QuotesChartPrefs = {
  interval: "5m",
  chartStyle: "candle",
};

const STORAGE_KEY = "quant-port-quotes-chart";

const VALID_INTERVALS = new Set<CandleInterval>([
  "1m",
  "5m",
  "10m",
  "30m",
  "60m",
  "1d",
  "1w",
  "1M",
]);

const VALID_CHART_STYLES = new Set<ChartStyle>(["bar", "candle", "area"]);

export function loadQuotesChartPrefs(): QuotesChartPrefs {
  if (typeof window === "undefined") return DEFAULT_QUOTES_CHART_PREFS;

  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (!raw) return DEFAULT_QUOTES_CHART_PREFS;
    const parsed = JSON.parse(raw) as Partial<QuotesChartPrefs>;
    return {
      interval: VALID_INTERVALS.has(parsed.interval as CandleInterval)
        ? (parsed.interval as CandleInterval)
        : DEFAULT_QUOTES_CHART_PREFS.interval,
      chartStyle: VALID_CHART_STYLES.has(parsed.chartStyle as ChartStyle)
        ? (parsed.chartStyle as ChartStyle)
        : DEFAULT_QUOTES_CHART_PREFS.chartStyle,
    };
  } catch {
    return DEFAULT_QUOTES_CHART_PREFS;
  }
}

export function saveQuotesChartPrefs(prefs: QuotesChartPrefs) {
  if (typeof window === "undefined") return;
  localStorage.setItem(STORAGE_KEY, JSON.stringify(prefs));
}
