"use client";

import { forwardRef, useCallback, useEffect, useImperativeHandle, useRef, useState } from "react";
import {
  AreaSeries,
  BarSeries,
  CandlestickSeries,
  ColorType,
  createChart,
  CrosshairMode,
  HistogramSeries,
  isBusinessDay,
  LineStyle,
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
  intradayInitialVisibleBars,
  isDailyBarInterval,
  isDailyCandleInterval,
  isMonthlyCandleInterval,
  isWeeklyCandleInterval,
  toChartTime,
  visibleEpochsFromChartTimes,
  type ChartVisibleSpan,
} from "@/lib/chart-time";
import { formatVolume } from "@/lib/format";
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
const DAILY_INITIAL_BARS = 120;
const WEEKLY_INITIAL_BARS = 52;
const MONTHLY_INITIAL_BARS = 36;
const LOAD_MORE_EDGE_BARS = 12;

// getComputedStyle can return CSS Color 4 syntax (e.g. lab(), oklch()) for
// custom properties defined via oklch() in globals.css. lightweight-charts'
// color parser only understands hex/rgb/hsl, and re-reading fillStyle back
// off a canvas context just echoes the same lab()/oklch() syntax on modern
// Chrome — so instead paint 1x1px and read the composited pixel back,
// which canvas always reports as plain unpremultiplied rgba.
let colorNormalizeCtx: CanvasRenderingContext2D | null = null;
function normalizeColor(value: string, fallback: string): string {
  if (!value) return fallback;
  if (typeof document === "undefined") return fallback;
  if (!colorNormalizeCtx) {
    const canvas = document.createElement("canvas");
    canvas.width = 1;
    canvas.height = 1;
    colorNormalizeCtx = canvas.getContext("2d", { willReadFrequently: true });
  }
  if (!colorNormalizeCtx) return fallback;
  try {
    colorNormalizeCtx.clearRect(0, 0, 1, 1);
    colorNormalizeCtx.fillStyle = value;
    colorNormalizeCtx.fillRect(0, 0, 1, 1);
    const [r, g, b, a] = colorNormalizeCtx.getImageData(0, 0, 1, 1).data;
    return `rgba(${r}, ${g}, ${b}, ${(a / 255).toFixed(3)})`;
  } catch {
    return fallback;
  }
}

function readChartThemeColors() {
  if (typeof window === "undefined") {
    return {
      textColor: "#94A3B8",
      gridColor: "rgba(148, 163, 184, 0.08)",
      crosshairBg: "#1A2030",
      crosshairText: "#F1F5F9",
      crosshairBorder: "#334155",
    };
  }
  const root = getComputedStyle(document.documentElement);
  return {
    textColor: normalizeColor(root.getPropertyValue("--text-secondary").trim(), "#94A3B8"),
    gridColor: normalizeColor(root.getPropertyValue("--chart-grid").trim(), "rgba(148, 163, 184, 0.08)"),
    crosshairBg: normalizeColor(root.getPropertyValue("--chart-crosshair-bg").trim(), "#1A2030"),
    crosshairText: normalizeColor(root.getPropertyValue("--chart-crosshair-text").trim(), "#F1F5F9"),
    crosshairBorder: normalizeColor(root.getPropertyValue("--chart-crosshair-border").trim(), "#334155"),
  };
}

function crosshairOptions() {
  const { crosshairBg, crosshairText } = readChartThemeColors();
  return {
    mode: CrosshairMode.Normal,
    vertLine: {
      color: "rgba(148, 163, 184, 0.45)",
      style: LineStyle.Dashed,
      labelBackgroundColor: crosshairBg,
      labelTextColor: crosshairText,
    },
    horzLine: {
      color: "rgba(148, 163, 184, 0.45)",
      style: LineStyle.Dashed,
      labelBackgroundColor: crosshairBg,
      labelTextColor: crosshairText,
    },
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
    crosshair: crosshairOptions(),
  });
}

function formatChartPrice(price: number, currency: "KRW" | "USD"): string {
  if (currency === "USD") {
    return `$${price.toFixed(2)}`;
  }
  return `${Math.round(price).toLocaleString("ko-KR")}원`;
}

type ChartHoverTooltip = {
  x: number;
  y: number;
  timeLabel: string;
  open: number;
  high: number;
  low: number;
  close: number;
  volume?: number;
};

function barFromCrosshairData(
  data: BarData<Time> | AreaData<Time> | undefined,
  chartStyle: ChartStyle,
): Pick<ChartHoverTooltip, "open" | "high" | "low" | "close"> | null {
  if (!data) return null;
  if (chartStyle === "area") {
    const area = data as AreaData<Time>;
    if (typeof area.value !== "number" || area.value <= 0) return null;
    return { open: area.value, high: area.value, low: area.value, close: area.value };
  }
  const bar = data as BarData<Time>;
  if (bar.close <= 0) return null;
  return {
    open: bar.open,
    high: bar.high,
    low: bar.low,
    close: bar.close,
  };
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

function formatAxisTickLabel(
  time: Time,
  interval: CandleInterval,
  visible: ChartVisibleSpan,
  visibleEpochs: number[],
): string {
  return formatAdaptiveChartAxisTime(
    barEpochSeconds(time),
    interval,
    visible,
    visibleEpochs,
  );
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
    if (candle.close <= 0 && candle.open <= 0 && candle.high <= 0) continue;
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

  if (ohlc.length >= 2 && compareChartTime(ohlc[0]!.time, ohlc[ohlc.length - 1]!.time) > 0) {
    ohlc.reverse();
    volume.reverse();
    area.reverse();
  }

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
  if (isMonthlyCandleInterval(interval)) {
    const visible = Math.min(MONTHLY_INITIAL_BARS, barCount);
    chart.timeScale().setVisibleLogicalRange({
      from: Math.max(0, barCount - visible),
      to: barCount + 0.5,
    });
    return;
  }
  if (isWeeklyCandleInterval(interval)) {
    const visible = Math.min(WEEKLY_INITIAL_BARS, barCount);
    chart.timeScale().setVisibleLogicalRange({
      from: Math.max(0, barCount - visible),
      to: barCount + 0.5,
    });
    return;
  }
  if (isDailyBarInterval(interval)) {
    const visible = Math.min(DAILY_INITIAL_BARS, barCount);
    chart.timeScale().setVisibleLogicalRange({
      from: Math.max(0, barCount - visible),
      to: barCount + 0.5,
    });
    return;
  }
  const visible = Math.min(intradayInitialVisibleBars(interval), barCount);
  chart.timeScale().setVisibleLogicalRange({
    from: Math.max(0, barCount - visible),
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
  const visibleEpochsRef = useRef<number[]>([]);
  const pinnedToLiveRef = useRef(true);
  const intervalRefForZoom = useRef(interval);
  const daily = isDailyCandleInterval(interval);
  const [tooltip, setTooltip] = useState<ChartHoverTooltip | null>(null);
  const currencyRef = useRef(currency);

  const tryLoadMoreAtLeftEdgeRef = useRef<() => void>(() => {});

  const tryLoadMoreAtLeftEdge = useCallback(() => {
    const chart = chartRef.current;
    if (!chart || !hasMoreRef.current || loadingMoreRef.current || loadMoreLockRef.current) return;
    const range = chart.timeScale().getVisibleLogicalRange();
    if (!range || range.from > LOAD_MORE_EDGE_BARS) return;
    loadMoreLockRef.current = true;
    onLoadMoreRef.current?.();
  }, []);

  useEffect(() => {
    tryLoadMoreAtLeftEdgeRef.current = tryLoadMoreAtLeftEdge;
  }, [tryLoadMoreAtLeftEdge]);

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
    chartRef.current?.applyOptions({
      timeScale: { fixLeftEdge: !hasMore },
    });
  }, [hasMore]);

  useEffect(() => {
    loadingMoreRef.current = loadingMore;
    if (!loadingMore) {
      loadMoreLockRef.current = false;
    }
  }, [loadingMore]);

  useEffect(() => {
    currencyRef.current = currency;
  }, [currency]);

  useEffect(() => {
    shouldFitRef.current = true;
    prevBarCountRef.current = 0;
    pinnedToLiveRef.current = true;
    setTooltip(null);
  }, [interval, currency, daily, chartStyle]);

  useEffect(() => {
    const container = containerRef.current;
    if (!container) return;

    const formatAxisTick = (time: Time) =>
      formatAxisTickLabel(time, intervalRef.current, visibleSpanRef.current, visibleEpochsRef.current);

    const applyAxisFormat = () => {
      chartRef.current?.applyOptions({
        timeScale: {
          tickMarkFormatter: formatAxisTick,
        },
      });
    };

    const syncVisibleSpan = (range: LogicalRange | null) => {
      if (!range || ohlcRef.current.length === 0) return;
      const times = ohlcRef.current.map((bar) => bar.time);
      visibleSpanRef.current = computeVisibleSpan(times, range.from, range.to);
      visibleEpochsRef.current = visibleEpochsFromChartTimes(times, range.from, range.to);
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
      crosshair: crosshairOptions(),
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
        fixLeftEdge: !hasMore,
        timeVisible: !daily,
        secondsVisible: false,
        tickMarkFormatter: (time: Time) =>
          formatAxisTickLabel(time, intervalRef.current, visibleSpanRef.current, visibleEpochsRef.current),
      },
      localization: {
        locale: "ko-KR",
        timeFormatter: (time: Time) => formatCrosshairTime(time, intervalRef.current),
        priceFormatter: (price: number) => formatChartPrice(price, currencyRef.current),
      },
    });

    const priceSeries = addPriceSeries(chart, chartStyle);

    const volumeSeries = chart.addSeries(HistogramSeries, {
      priceFormat: { type: "volume" },
      priceScaleId: "volume",
      // The chart-wide priceFormatter appends the price currency, so a volume
      // last-value badge would read "5,533,129원". Volume stays in the tooltip.
      lastValueVisible: false,
      priceLineVisible: false,
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
      tryLoadMoreAtLeftEdgeRef.current();
    };

    chart.timeScale().subscribeVisibleLogicalRangeChange(handleVisibleRange);

    const handleCrosshairMove = (param: { point?: { x: number; y: number }; time?: Time; seriesData: Map<unknown, unknown> }) => {
      if (!param.point || param.point.x < 0 || param.point.y < 0 || param.time == null) {
        setTooltip(null);
        return;
      }
      const priceData = param.seriesData.get(priceSeries) as BarData<Time> | AreaData<Time> | undefined;
      const volData = param.seriesData.get(volumeSeries) as HistogramData<Time> | undefined;
      const bar = barFromCrosshairData(priceData, chartStyleRef.current);
      if (!bar) {
        setTooltip(null);
        return;
      }
      const width = container.clientWidth;
      setTooltip({
        x: Math.min(Math.max(param.point.x + 14, 8), Math.max(width - 176, 8)),
        y: Math.max(param.point.y - 96, 8),
        timeLabel: formatCrosshairTime(param.time, intervalRef.current),
        ...bar,
        volume: volData?.value,
      });
    };

    chart.subscribeCrosshairMove(handleCrosshairMove);

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
      chart.unsubscribeCrosshairMove(handleCrosshairMove);
      chart.timeScale().unsubscribeVisibleLogicalRangeChange(handleVisibleRange);
      chart.remove();
      chartRef.current = null;
      priceSeriesRef.current = null;
      volumeSeriesRef.current = null;
      setTooltip(null);
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
          const span = visibleRange
            ? visibleRange.to - visibleRange.from
            : intradayInitialVisibleBars(interval);
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
      const span = visibleRange
        ? visibleRange.to - visibleRange.from
        : intradayInitialVisibleBars(interval);
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
      visibleEpochsRef.current = visibleEpochsFromChartTimes(
        ohlc.map((bar) => bar.time),
        nextRange.from,
        nextRange.to,
      );
      chart.applyOptions({
        timeScale: {
          tickMarkFormatter: (time: Time) =>
            formatAxisTickLabel(time, interval, visibleSpanRef.current, visibleEpochsRef.current),
        },
      });
    }
  }, [candles, interval, chartStyle]);

  return (
    <div className="relative h-full w-full min-h-[240px]">
      <div ref={containerRef} className="h-full w-full" />
      {tooltip && (
        <div
          className="pointer-events-none absolute z-10 min-w-[9.5rem] rounded-lg border px-2.5 py-2 shadow-lg"
          style={{
            left: tooltip.x,
            top: tooltip.y,
            background: "var(--chart-crosshair-bg)",
            borderColor: "var(--chart-crosshair-border)",
            color: "var(--chart-crosshair-text)",
          }}
        >
          <p className="text-[11px] font-semibold">{tooltip.timeLabel}</p>
          <dl className="mt-1.5 space-y-0.5 font-mono text-[10px] leading-relaxed">
            <div className="flex justify-between gap-3">
              <dt className="text-[var(--text-muted)]">시가</dt>
              <dd>{formatChartPrice(tooltip.open, currency)}</dd>
            </div>
            <div className="flex justify-between gap-3">
              <dt className="text-[var(--text-muted)]">고가</dt>
              <dd>{formatChartPrice(tooltip.high, currency)}</dd>
            </div>
            <div className="flex justify-between gap-3">
              <dt className="text-[var(--text-muted)]">저가</dt>
              <dd>{formatChartPrice(tooltip.low, currency)}</dd>
            </div>
            <div className="flex justify-between gap-3">
              <dt className="font-medium text-[var(--text-secondary)]">종가</dt>
              <dd className="font-semibold">{formatChartPrice(tooltip.close, currency)}</dd>
            </div>
            {tooltip.volume != null && tooltip.volume > 0 && (
              <div className="flex justify-between gap-3 border-t border-[var(--chart-crosshair-border)] pt-1">
                <dt className="text-[var(--text-muted)]">거래량</dt>
                <dd>{formatVolume(tooltip.volume)}</dd>
              </div>
            )}
          </dl>
        </div>
      )}
    </div>
  );
},
);
