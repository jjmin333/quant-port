"use client";

import { forwardRef, useEffect, useImperativeHandle, useRef } from "react";
import {
  AreaSeries,
  BarSeries,
  CandlestickSeries,
  ColorType,
  createChart,
  CrosshairMode,
  HistogramSeries,
  isBusinessDay,
  type AreaData,
  type BarData,
  type CandlestickData,
  type HistogramData,
  type IChartApi,
  type ISeriesApi,
  type LogicalRange,
  type SeriesType,
  type Time,
} from "lightweight-charts";
import {
  computeVisibleSpan,
  formatAdaptiveChartAxisTime,
  formatChartCrosshairBusinessDay,
  formatChartCrosshairTime,
  isDailyCandleInterval,
  toChartTime,
  type ChartVisibleSpan,
} from "@/lib/chart-time";
import type { Candle, CandleInterval, ChartStyle } from "@/lib/quotes-types";

type PriceSeriesType = Extract<SeriesType, "Candlestick" | "Bar" | "Area">;

type CandlestickChartProps = {
  candles: Candle[];
  currency: "KRW" | "USD";
  interval: CandleInterval;
  chartStyle?: ChartStyle;
  height?: number;
  hasMore?: boolean;
  loadingMore?: boolean;
  onLoadMore?: () => void;
};

const UP_COLOR = "#EF4444";
const DOWN_COLOR = "#3B82F6";
const INTRADAY_INITIAL_BARS = 120;
/** Gaps larger than this start a new intraday session (overnight / multi-day tail). */
const MAX_INTRADAY_GAP_SEC = 3 * 3600;

function readChartThemeColors() {
  if (typeof window === "undefined") {
    return { textColor: "#94A3B8", gridColor: "rgba(148, 163, 184, 0.08)" };
  }
  const root = getComputedStyle(document.documentElement);
  return {
    textColor: root.getPropertyValue("--text-secondary").trim() || "#94A3B8",
    gridColor: root.getPropertyValue("--chart-grid").trim() || "rgba(148, 163, 184, 0.08)",
  };
}

function applyChartTheme(chart: IChartApi) {
  const { textColor, gridColor } = readChartThemeColors();
  chart.applyOptions({
    layout: { textColor },
    grid: {
      vertLines: { color: gridColor },
      horzLines: { color: gridColor },
    },
  });
}

export type CandlestickChartHandle = {
  zoomIn: () => void;
  zoomOut: () => void;
  resetZoom: () => void;
};

function timeKey(time: Time): string {
  if (typeof time === "string") return time;
  if (isBusinessDay(time)) {
    return `${time.year}-${String(time.month).padStart(2, "0")}-${String(time.day).padStart(2, "0")}`;
  }
  return String(time);
}

function compareChartTime(a: Time, b: Time): number {
  return barEpochSeconds(a) - barEpochSeconds(b);
}

function barEpochSeconds(time: Time): number {
  if (typeof time === "number") return time;
  if (typeof time === "string") return new Date(`${time}T00:00:00+09:00`).getTime() / 1000;
  return new Date(time.year, time.month - 1, time.day).getTime() / 1000;
}

/** Index of the first bar in the latest contiguous intraday session. */
function findIntradaySessionStart(ohlc: CandlestickData<Time>[]): number {
  if (ohlc.length <= 1) return 0;
  let start = ohlc.length - 1;
  for (let i = ohlc.length - 2; i >= 0; i--) {
    const gap = barEpochSeconds(ohlc[i + 1]!.time) - barEpochSeconds(ohlc[i]!.time);
    if (gap > MAX_INTRADAY_GAP_SEC) break;
    start = i;
  }
  return start;
}

function formatCrosshairTime(time: Time, interval: CandleInterval): string {
  if (typeof time === "string") {
    return time.slice(0, 10);
  }
  if (isBusinessDay(time)) {
    return formatChartCrosshairBusinessDay(time.year, time.month, time.day);
  }
  return formatChartCrosshairTime(time as number, interval);
}

function buildSeriesData(candles: Candle[], interval: CandleInterval) {
  const sorted = [...candles].sort((a, b) =>
    compareChartTime(toChartTime(a.time, interval) as Time, toChartTime(b.time, interval) as Time),
  );

  const ohlcMap = new Map<string, CandlestickData<Time>>();
  const volumeMap = new Map<string, HistogramData<Time>>();

  for (const candle of sorted) {
    const time = toChartTime(candle.time, interval) as Time;
    const key = timeKey(time);
    const existing = ohlcMap.get(key);

    if (existing) {
      ohlcMap.set(key, {
        time,
        open: existing.open,
        high: Math.max(existing.high, candle.high),
        low: Math.min(existing.low, candle.low),
        close: candle.close,
      });
      const volume = volumeMap.get(key);
      volumeMap.set(key, {
        time,
        value: (volume?.value ?? 0) + candle.volume,
        color: candle.close >= candle.open ? "rgba(239, 68, 68, 0.45)" : "rgba(59, 130, 246, 0.45)",
      });
      continue;
    }

    ohlcMap.set(key, {
      time,
      open: candle.open,
      high: candle.high,
      low: candle.low,
      close: candle.close,
    });
    volumeMap.set(key, {
      time,
      value: candle.volume,
      color: candle.close >= candle.open ? "rgba(239, 68, 68, 0.45)" : "rgba(59, 130, 246, 0.45)",
    });
  }

  const ohlc = Array.from(ohlcMap.values()).sort((a, b) => compareChartTime(a.time, b.time));
  const volume = Array.from(volumeMap.values()).sort((a, b) => compareChartTime(a.time, b.time));
  const area: AreaData<Time>[] = ohlc.map((bar) => ({ time: bar.time, value: bar.close }));

  return { ohlc, volume, area };
}

function addPriceSeries(chart: IChartApi, chartStyle: ChartStyle): ISeriesApi<PriceSeriesType> {
  switch (chartStyle) {
    case "bar":
      return chart.addSeries(BarSeries, {
        upColor: UP_COLOR,
        downColor: DOWN_COLOR,
        openVisible: true,
        thinBars: true,
      });
    case "area":
      return chart.addSeries(AreaSeries, {
        lineColor: UP_COLOR,
        topColor: "rgba(239, 68, 68, 0.35)",
        bottomColor: "rgba(239, 68, 68, 0)",
      });
    case "candle":
    default:
      return chart.addSeries(CandlestickSeries, {
        upColor: UP_COLOR,
        downColor: DOWN_COLOR,
        borderUpColor: UP_COLOR,
        borderDownColor: DOWN_COLOR,
        wickUpColor: UP_COLOR,
        wickDownColor: DOWN_COLOR,
      });
  }
}

function setPriceSeriesData(
  series: ISeriesApi<PriceSeriesType>,
  chartStyle: ChartStyle,
  ohlc: BarData<Time>[],
  area: AreaData<Time>[],
) {
  if (chartStyle === "area") {
    series.setData(area);
    return;
  }
  series.setData(ohlc);
}

function updatePriceSeriesBar(
  series: ISeriesApi<PriceSeriesType>,
  chartStyle: ChartStyle,
  bar: BarData<Time>,
) {
  if (chartStyle === "area") {
    series.update({ time: bar.time, value: bar.close });
    return;
  }
  series.update(bar);
}

function applyDefaultVisibleRange(
  chart: IChartApi,
  ohlc: CandlestickData<Time>[],
  interval: CandleInterval,
) {
  const barCount = ohlc.length;
  if (barCount <= 0) return;
  if (isDailyCandleInterval(interval)) {
    chart.timeScale().fitContent();
    return;
  }
  const sessionStart = findIntradaySessionStart(ohlc);
  const sessionLen = barCount - sessionStart;
  const visible = Math.min(INTRADAY_INITIAL_BARS, sessionLen);
  chart.timeScale().setVisibleLogicalRange({
    from: Math.max(sessionStart, barCount - visible),
    to: barCount + 0.5,
  });
}

function followLiveEdge(chart: IChartApi, barCount: number, span: number) {
  chart.timeScale().setVisibleLogicalRange({
    from: Math.max(0, barCount - span),
    to: barCount + 0.5,
  });
}

function isPinnedToLiveEdge(range: LogicalRange | null, barCount: number): boolean {
  if (!range || barCount <= 0) return true;
  return range.to >= barCount - 2;
}

function zoomVisibleRange(chart: IChartApi, factor: number) {
  const range = chart.timeScale().getVisibleLogicalRange();
  if (!range) return;
  const center = (range.from + range.to) / 2;
  const span = (range.to - range.from) * factor;
  chart.timeScale().setVisibleLogicalRange({
    from: center - span / 2,
    to: center + span / 2,
  });
}

export const CandlestickChart = forwardRef<CandlestickChartHandle, CandlestickChartProps>(
  function CandlestickChart(
    {
      candles,
      currency,
      interval,
      chartStyle = "candle",
      height = 320,
      hasMore = false,
      loadingMore = false,
      onLoadMore,
    }: CandlestickChartProps,
    ref,
  ) {
  const containerRef = useRef<HTMLDivElement>(null);
  const chartRef = useRef<IChartApi | null>(null);
  const priceSeriesRef = useRef<ISeriesApi<PriceSeriesType> | null>(null);
  const volumeSeriesRef = useRef<ISeriesApi<"Histogram"> | null>(null);
  const shouldFitRef = useRef(true);
  const prevBarCountRef = useRef(0);
  const loadMoreLockRef = useRef(false);
  const onLoadMoreRef = useRef(onLoadMore);
  const hasMoreRef = useRef(hasMore);
  const loadingMoreRef = useRef(loadingMore);
  const intervalRef = useRef(interval);
  const chartStyleRef = useRef(chartStyle);
  const ohlcRef = useRef<CandlestickData<Time>[]>([]);
  const volumeRef = useRef<HistogramData<Time>[]>([]);
  const visibleSpanRef = useRef<ChartVisibleSpan>({ spanSeconds: 0 });
  const pinnedToLiveRef = useRef(true);
  const intervalRefForZoom = useRef(interval);
  const daily = isDailyCandleInterval(interval);

  useImperativeHandle(ref, () => ({
    zoomIn: () => {
      if (chartRef.current) zoomVisibleRange(chartRef.current, 0.75);
    },
    zoomOut: () => {
      if (chartRef.current) zoomVisibleRange(chartRef.current, 1.35);
    },
    resetZoom: () => {
      if (chartRef.current) {
        applyDefaultVisibleRange(chartRef.current, ohlcRef.current, intervalRefForZoom.current);
        pinnedToLiveRef.current = true;
      }
    },
  }));

  useEffect(() => {
    intervalRefForZoom.current = interval;
  }, [interval]);

  useEffect(() => {
    chartStyleRef.current = chartStyle;
  }, [chartStyle]);

  useEffect(() => {
    onLoadMoreRef.current = onLoadMore;
  }, [onLoadMore]);

  useEffect(() => {
    hasMoreRef.current = hasMore;
  }, [hasMore]);

  useEffect(() => {
    loadingMoreRef.current = loadingMore;
    if (!loadingMore) {
      loadMoreLockRef.current = false;
    }
  }, [loadingMore]);

  useEffect(() => {
    shouldFitRef.current = true;
    prevBarCountRef.current = 0;
    pinnedToLiveRef.current = true;
  }, [interval, currency, daily, chartStyle]);

  useEffect(() => {
    const container = containerRef.current;
    if (!container) return;

    const applyAxisFormat = () => {
      chartRef.current?.applyOptions({
        timeScale: {
          tickMarkFormatter: (time: Time) => {
            if (typeof time === "string") {
              return formatAdaptiveChartAxisTime(
                new Date(`${time}T00:00:00+09:00`).getTime() / 1000,
                intervalRef.current,
                visibleSpanRef.current,
              );
            }
            if (isBusinessDay(time)) {
              return formatAdaptiveChartAxisTime(
                new Date(time.year, time.month - 1, time.day).getTime() / 1000,
                intervalRef.current,
                visibleSpanRef.current,
              );
            }
            return formatAdaptiveChartAxisTime(
              time as number,
              intervalRef.current,
              visibleSpanRef.current,
            );
          },
        },
      });
    };

    const syncVisibleSpan = (range: LogicalRange | null) => {
      if (!range || ohlcRef.current.length === 0) return;
      visibleSpanRef.current = computeVisibleSpan(
        ohlcRef.current.map((bar) => bar.time),
        range.from,
        range.to,
      );
      applyAxisFormat();
    };

    const { textColor, gridColor } = readChartThemeColors();

    const chart = createChart(container, {
      height,
      layout: {
        background: { type: ColorType.Solid, color: "transparent" },
        textColor,
        fontSize: 11,
        attributionLogo: false,
      },
      grid: {
        vertLines: { color: gridColor },
        horzLines: { color: gridColor },
      },
      crosshair: { mode: CrosshairMode.Normal },
      handleScale: {
        axisPressedMouseMove: { time: true, price: true },
        mouseWheel: false,
        pinch: true,
      },
      handleScroll: {
        mouseWheel: true,
        pressedMouseMove: true,
        horzTouchDrag: true,
        vertTouchDrag: false,
      },
      rightPriceScale: {
        borderVisible: false,
        scaleMargins: { top: 0.08, bottom: 0.28 },
      },
      timeScale: {
        borderVisible: false,
        timeVisible: !daily,
        secondsVisible: false,
        tickMarkFormatter: (time: Time) => {
          if (typeof time === "string") {
            return formatAdaptiveChartAxisTime(
              new Date(`${time}T00:00:00+09:00`).getTime() / 1000,
              intervalRef.current,
              visibleSpanRef.current,
            );
          }
          if (isBusinessDay(time)) {
            return formatAdaptiveChartAxisTime(
              new Date(time.year, time.month - 1, time.day).getTime() / 1000,
              intervalRef.current,
              visibleSpanRef.current,
            );
          }
          return formatAdaptiveChartAxisTime(
            time as number,
            intervalRef.current,
            visibleSpanRef.current,
          );
        },
      },
      localization: {
        locale: "en-US",
        timeFormatter: (time: Time) => formatCrosshairTime(time, intervalRef.current),
        priceFormatter: (price: number) =>
          currency === "USD"
            ? `$${price.toFixed(2)}`
            : `${Math.round(price).toLocaleString("ko-KR")}`,
      },
    });

    const priceSeries = addPriceSeries(chart, chartStyle);

    const volumeSeries = chart.addSeries(HistogramSeries, {
      priceFormat: { type: "volume" },
      priceScaleId: "volume",
    });

    chart.priceScale("volume").applyOptions({
      scaleMargins: { top: 0.78, bottom: 0 },
    });

    chartRef.current = chart;
    priceSeriesRef.current = priceSeries;
    volumeSeriesRef.current = volumeSeries;

    const themeObserver = new MutationObserver(() => {
      if (chartRef.current) applyChartTheme(chartRef.current);
    });
    themeObserver.observe(document.documentElement, {
      attributes: true,
      attributeFilter: ["data-theme"],
    });

    const handleVisibleRange = (range: LogicalRange | null) => {
      syncVisibleSpan(range);
      pinnedToLiveRef.current = isPinnedToLiveEdge(range, ohlcRef.current.length);
      if (!range || range.from > 12 || !hasMoreRef.current || loadingMoreRef.current) return;
      if (loadMoreLockRef.current) return;
      loadMoreLockRef.current = true;
      onLoadMoreRef.current?.();
    };

    chart.timeScale().subscribeVisibleLogicalRangeChange(handleVisibleRange);

    const resize = () => {
      if (containerRef.current && chartRef.current) {
        chartRef.current.applyOptions({ width: containerRef.current.clientWidth });
        syncVisibleSpan(chartRef.current.timeScale().getVisibleLogicalRange());
      }
    };

    resize();
    const observer = new ResizeObserver(resize);
    observer.observe(container);

    return () => {
      themeObserver.disconnect();
      observer.disconnect();
      chart.timeScale().unsubscribeVisibleLogicalRangeChange(handleVisibleRange);
      chart.remove();
      chartRef.current = null;
      priceSeriesRef.current = null;
      volumeSeriesRef.current = null;
    };
  }, [height, interval, currency, daily, chartStyle]);

  useEffect(() => {
    if (!priceSeriesRef.current || !volumeSeriesRef.current || !chartRef.current) return;

    const { ohlc, volume, area } = buildSeriesData(candles, interval);
    if (ohlc.length === 0) return;

    const chart = chartRef.current;
    const priceSeries = priceSeriesRef.current;
    const volumeSeries = volumeSeriesRef.current;
    const prevOhlc = ohlcRef.current;
    const prevCount = prevBarCountRef.current;
    const visibleRange = chart.timeScale().getVisibleLogicalRange();
    const pinned = pinnedToLiveRef.current;

    // Real-time forming bar: update last point only (avoids full redraw / viewport jump).
    if (
      prevCount > 0 &&
      ohlc.length === prevCount &&
      prevOhlc.length === ohlc.length &&
      ohlc.length > 0
    ) {
      const lastIdx = ohlc.length - 1;
      const prevLast = prevOhlc[lastIdx];
      const nextLast = ohlc[lastIdx];
      if (
        prevLast &&
        nextLast &&
        timeKey(prevLast.time) === timeKey(nextLast.time) &&
        (prevLast.open !== nextLast.open ||
          prevLast.high !== nextLast.high ||
          prevLast.low !== nextLast.low ||
          prevLast.close !== nextLast.close)
      ) {
        updatePriceSeriesBar(priceSeries, chartStyleRef.current, nextLast);
        const volBar = volume[lastIdx];
        if (volBar) volumeSeries.update(volBar);
        ohlcRef.current = ohlc;
        volumeRef.current = volume;
        if (pinned && !isDailyCandleInterval(interval)) {
          const span = visibleRange ? visibleRange.to - visibleRange.from : INTRADAY_INITIAL_BARS;
          followLiveEdge(chart, ohlc.length, span);
        }
        prevBarCountRef.current = ohlc.length;
        return;
      }
    }

    ohlcRef.current = ohlc;
    volumeRef.current = volume;

    const addedLeft = ohlc.length > prevCount && prevCount > 0 ? ohlc.length - prevCount : 0;

    setPriceSeriesData(priceSeries, chartStyleRef.current, ohlc, area);
    volumeSeries.setData(volume);

    if (shouldFitRef.current) {
      applyDefaultVisibleRange(chart, ohlc, interval);
      shouldFitRef.current = false;
      pinnedToLiveRef.current = true;
    } else if (pinned && ohlc.length > prevCount && prevCount > 0 && !isDailyCandleInterval(interval)) {
      const span = visibleRange ? visibleRange.to - visibleRange.from : INTRADAY_INITIAL_BARS;
      followLiveEdge(chart, ohlc.length, span);
    } else if (visibleRange && addedLeft > 0) {
      chart.timeScale().setVisibleLogicalRange({
        from: visibleRange.from + addedLeft,
        to: visibleRange.to + addedLeft,
      });
    }

    prevBarCountRef.current = ohlc.length;

    const nextRange = chart.timeScale().getVisibleLogicalRange();
    if (nextRange) {
      visibleSpanRef.current = computeVisibleSpan(
        ohlc.map((bar) => bar.time),
        nextRange.from,
        nextRange.to,
      );
      chart.applyOptions({
        timeScale: {
          tickMarkFormatter: (time: Time) => {
            if (typeof time === "string") {
              return formatAdaptiveChartAxisTime(
                new Date(`${time}T00:00:00+09:00`).getTime() / 1000,
                interval,
                visibleSpanRef.current,
              );
            }
            if (isBusinessDay(time)) {
              return formatAdaptiveChartAxisTime(
                new Date(time.year, time.month - 1, time.day).getTime() / 1000,
                interval,
                visibleSpanRef.current,
              );
            }
            return formatAdaptiveChartAxisTime(
              time as number,
              interval,
              visibleSpanRef.current,
            );
          },
        },
      });
    }
  }, [candles, interval, chartStyle]);

  return <div ref={containerRef} className="h-full w-full min-h-[240px]" />;
},
);
