import type { CandleInterval, ChartStyle } from "@/lib/quotes-types";

export type QuotesChartPrefs = {
  interval: CandleInterval;
  chartStyle: ChartStyle;
};

export const DEFAULT_QUOTES_CHART_PREFS: QuotesChartPrefs = {
  interval: "1d",
  chartStyle: "candle",
};

const STORAGE_KEY = "quant-port-quotes-chart";

const VALID_INTERVALS = new Set<CandleInterval>([
  "1m",
  "5m",
  "10m",
  "30m",
  "1d",
  "1w",
  "1M",
]);

const VALID_CHART_STYLES = new Set<ChartStyle>(["bar", "candle", "area"]);

/**
 * 사라진 인터벌을 가장 가까운 살아 있는 것으로 옮긴다.
 *
 * 60분봉은 2026-08-24에 제거됐다(서버 부하가 인터벌 중 압도적으로 컸다 —
 * 첫 화면 한 번에 1분봉 7,200개, Toss 36호출). 그런데 그때 60분봉을 쓰고 있던
 * 사용자의 localStorage에는 "60m"이 남는다.
 *
 * 아래 일반 검증만 두면 그 값이 무효로 판정돼 기본값(일봉)으로 떨어진다.
 * 분봉을 보던 사람에게 일봉은 시간축이 통째로 달라지는 큰 점프라,
 * **가장 가까운 살아 있는 간격(30분봉)**으로 보낸다.
 */
const RETIRED_INTERVALS: Record<string, CandleInterval> = {
  "60m": "30m",
};

function resolveStoredInterval(stored: unknown): CandleInterval {
  if (typeof stored !== "string") return DEFAULT_QUOTES_CHART_PREFS.interval;
  const moved = RETIRED_INTERVALS[stored];
  if (moved) return moved;
  return VALID_INTERVALS.has(stored as CandleInterval)
    ? (stored as CandleInterval)
    : DEFAULT_QUOTES_CHART_PREFS.interval;
}

export function loadQuotesChartPrefs(): QuotesChartPrefs {
  if (typeof window === "undefined") return DEFAULT_QUOTES_CHART_PREFS;

  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (!raw) return DEFAULT_QUOTES_CHART_PREFS;
    const parsed = JSON.parse(raw) as Partial<QuotesChartPrefs>;
    return {
      interval: resolveStoredInterval(parsed.interval),
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
