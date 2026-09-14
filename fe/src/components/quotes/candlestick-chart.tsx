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
  type IPriceLine,
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
import {
  measureAxisLabelWidth,
  rightEdgePad,
  rightEdgePadPx,
  widestAxisLabelSample,
} from "@/lib/chart-axis";
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
  /**
   * 축 오른쪽 끝의 **현재가 표**를 이 값으로 그린다 (2026-09-05 사용자 지적 4).
   *
   * # 왜 시리즈에게 맡기지 않는가
   *
   * lightweight-charts는 기본으로 **마지막 봉의 종가**를 그 자리에 적는다. 사용자가
   * 「상단 가격과 하단 가격이 다르다」고 한 것이 그것이다 — 헤더는 `detail.currentPrice`
   * (체결·호가·배치 캐시를 거친 값)인데, 차트 표는 마지막 봉의 close라 **두 값이 서로
   * 다른 길에서 온다**.
   *
   * 평소에는 둘이 같다. 형성 중인 봉을 화면이 현재가로 덧칠하기 때문이다
   * (`use-quote-candles`의 `applyLivePrice`). 그런데 그 덧칠은 **형성봉이 있을 때만**
   * 된다 — 서버가 형성봉을 안 준 구간(일봉의 장 초반, 프리·애프터 세션)에서는 표가
   * 조용히 어제 종가에 머문다. 값이 두 길에서 오는 한 그런 구간은 또 생긴다.
   *
   * 그래서 표를 **헤더와 같은 값에 직접 묶는다.** 봉 데이터는 건드리지 않는다 —
   * 일봉의 종가는 정의가 있는 값이라 시간외 가격으로 덮으면 그건 다른 종류의 거짓이다.
   *
   * 넘기지 않으면(0 · undefined) 예전처럼 시리즈 기본 동작을 쓴다.
   */
  livePrice?: number;
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

/*
 * 캔들 차트의 축·크로스헤어 글자는 **캔버스에 그려지므로 CSS를 상속받지 않는다.**
 * 지정하지 않으면 lightweight-charts 자체 기본값(Trebuchet MS 계열)이 쓰여, 앱에서
 * 유일하게 Pretendard가 아닌 글자가 여기에 남는다 — 축의 날짜·가격만 다른 글꼴로
 * 보이던 원인이다. `--font-pretendard`를 읽어 그대로 넘긴다.
 */
/** 축 글자 크기 — 아래 차트 옵션과 여백 계산이 **같은 값**을 봐야 한다. */
const AXIS_FONT_SIZE_PX = 11;

/**
 * 이 간격에서 라벨이 잘리지 않을 오른쪽 여백(px). 캔버스로 **실제로 재서** 정한다 —
 * 어림수로 적어 둔 16px이 6px 모자라 「고쳤는데 그대로」였던 적이 있다
 * (`lib/chart-axis.ts` 참조).
 */
function axisRightPadPx(interval: CandleInterval): number {
  const font = `${AXIS_FONT_SIZE_PX}px ${readChartFontFamily()}`;
  return rightEdgePadPx(measureAxisLabelWidth(widestAxisLabelSample(interval), font));
}

function readChartFontFamily() {
  const fallback =
    '"Pretendard Variable", Pretendard, -apple-system, system-ui, sans-serif';
  if (typeof window === "undefined") return fallback;
  const v = getComputedStyle(document.documentElement)
    .getPropertyValue("--font-pretendard")
    .trim();
  return v || fallback;
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

/**
 * 오른쪽 끝의 논리 좌표. 여백 계산은 `lib/chart-axis`에 있다(캔버스 없이 검증한다).
 *
 * 두 번째 반환값은 「**실제 축 폭으로 쟀는가**」다 — 첫 렌더에는 캔버스 폭이 아직
 * 0이라 폴백(0.5봉)을 타고, 그러면 「2026-09」가 잘린 채로 그려진다.
 */
function rightEdgeLogical(
  chart: IChartApi,
  barCount: number,
  visible: number,
  padPx: number,
): { to: number; exact: boolean } {
  const pad = rightEdgePad(chart.timeScale().width(), visible, padPx);
  return { to: barCount + pad.bars, exact: pad.exact };
}

/**
 * 처음 보여 줄 구간. **축 폭을 알고 계산했으면 true**를 돌려준다 — false면 부르는
 * 쪽이 폭이 정해진 뒤 한 번 더 불러야 한다(`retryRightEdge`).
 */
function applyDefaultVisibleRange(
  chart: IChartApi,
  ohlc: CandlestickData<Time>[],
  interval: CandleInterval,
): boolean {
  const padPx = axisRightPadPx(interval);
  const barCount = ohlc.length;
  if (barCount <= 0) return true;
  let initial = intradayInitialVisibleBars(interval);
  if (isMonthlyCandleInterval(interval)) initial = MONTHLY_INITIAL_BARS;
  else if (isWeeklyCandleInterval(interval)) initial = WEEKLY_INITIAL_BARS;
  else if (isDailyBarInterval(interval)) initial = DAILY_INITIAL_BARS;

  const visible = Math.min(initial, barCount);
  const edge = rightEdgeLogical(chart, barCount, visible, padPx);
  chart.timeScale().setVisibleLogicalRange({
    from: Math.max(0, barCount - visible),
    to: edge.to,
  });
  return edge.exact;
}

/** 축 폭이 정해지기를 기다리며 다시 잡아 볼 최대 프레임 수 — 위 주석 참조. */
const RIGHT_EDGE_MAX_RETRIES = 30;

function followLiveEdge(
  chart: IChartApi,
  barCount: number,
  span: number,
  interval: CandleInterval,
) {
  chart.timeScale().setVisibleLogicalRange({
    from: Math.max(0, barCount - span),
    to: rightEdgeLogical(chart, barCount, span, axisRightPadPx(interval)).to,
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
      livePrice = 0,
    }: CandlestickChartProps,
    ref,
  ) {
  const containerRef = useRef<HTMLDivElement>(null);
  const chartRef = useRef<IChartApi | null>(null);
  const priceSeriesRef = useRef<ISeriesApi<PriceSeriesType> | null>(null);
  /** 헤더와 같은 값으로 그리는 현재가 선. `livePrice`를 안 받으면 만들지 않는다. */
  const livePriceLineRef = useRef<IPriceLine | null>(null);
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
  /*
    **축 폭을 모른 채 처음 구간을 잡았는가** (2026-09-03 사용자 신고).

    첫 렌더에는 캔버스 폭이 0이라 오른쪽 여백을 봉 단위로 환산할 수 없다. 그때는
    폴백(0.5봉)으로 그리는데 그 폭으로는 맨 오른쪽 라벨(「2026-09」)이 잘린다 —
    「개요 탭을 모바일에서 **처음** 열었을 때만」 잘려 보이던 것이 이것이다.
    폭이 정해지는 대로 프레임마다 다시 시도해서 제대로 된 여백으로 바꿔 놓는다.
  */
  const rightEdgePendingRef = useRef(false);
  const rightEdgeFrameRef = useRef(0);
  const intervalRefForZoom = useRef(interval);
  const daily = isDailyCandleInterval(interval);
  const [tooltip, setTooltip] = useState<ChartHoverTooltip | null>(null);
  const currencyRef = useRef(currency);

  /**
   * 축 폭이 생길 때까지 프레임마다 처음 구간을 다시 잡는다.
   *
   * 폭이 0인 이유는 대개 **아직 배치가 끝나지 않아서**라 한두 프레임이면 정해진다.
   * 그래도 상한을 둔다 — 탭이 숨겨져 있거나 부모가 `display:none`이면 폭은 영영
   * 0이고, 그때 프레임을 계속 잡으면 보이지도 않는 차트가 배터리를 쓴다. 상한에
   * 걸려도 손해는 없다: 폴백 여백으로 이미 그려져 있고, 화면에 나타나는 순간
   * `ResizeObserver`가 같은 일을 한다.
   */
  const rightEdgeRetriesRef = useRef(0);
  const retryRightEdge = useCallback(() => {
    window.cancelAnimationFrame(rightEdgeFrameRef.current);
    rightEdgePendingRef.current = true;
    rightEdgeRetriesRef.current = 0;

    const step = () => {
      const chart = chartRef.current;
      if (!chart || !rightEdgePendingRef.current) return;
      if (applyDefaultVisibleRange(chart, ohlcRef.current, intervalRef.current)) {
        rightEdgePendingRef.current = false;
        return;
      }
      if (rightEdgeRetriesRef.current >= RIGHT_EDGE_MAX_RETRIES) {
        rightEdgePendingRef.current = false;
        return;
      }
      rightEdgeRetriesRef.current += 1;
      rightEdgeFrameRef.current = window.requestAnimationFrame(step);
    };

    rightEdgeFrameRef.current = window.requestAnimationFrame(step);
  }, []);

  useEffect(() => () => window.cancelAnimationFrame(rightEdgeFrameRef.current), []);

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
        fontSize: AXIS_FONT_SIZE_PX,
        fontFamily: readChartFontFamily(),
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
    livePriceLineRef.current = null;

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
        /*
          폭이 생겼는데 처음 구간을 폭 없이 잡아 뒀다면 지금 다시 잡는다 — 숨겨져
          있던 탭이 처음 보이는 순간이 여기다(프레임 재시도는 그전에 상한에 걸린다).
        */
        if (rightEdgePendingRef.current) {
          if (applyDefaultVisibleRange(chartRef.current, ohlcRef.current, intervalRef.current)) {
            rightEdgePendingRef.current = false;
          }
        } else if (pinnedToLiveRef.current) {
          /*
            **폭이 바뀌면 오른쪽 여백을 다시 잰다.**

            여백은 픽셀로 정하고 봉 단위로 환산해 두는 값이라(`rightEdgePad`), 칸이
            좁아지면 봉 간격이 줄어 그 봉 수로는 모자라게 된다 — 목록에서 상세로
            넘어가며 폭이 바뀌는 화면(랭킹·마켓 레이더)에서 「2026-09」가 그렇게
            다시 잘렸다. **보고 있던 배율(span)은 그대로 두고** 오른쪽 끝만 다시
            맞춘다. 사용자가 과거로 밀어 둔 상태면(`pinnedToLiveRef`가 false) 손대지
            않는다 — 그 자리를 흔드는 것이 잘린 라벨보다 나쁘다.
          */
          const range = chartRef.current.timeScale().getVisibleLogicalRange();
          const barCount = ohlcRef.current.length;
          if (range && barCount > 0) {
            followLiveEdge(chartRef.current, barCount, range.to - range.from, intervalRef.current);
          }
        }
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
      livePriceLineRef.current = null;
      volumeSeriesRef.current = null;
      setTooltip(null);
    };
  }, [height, interval, currency, daily, chartStyle]);

  /*
    **현재가 표는 헤더와 같은 값에 묶는다** (2026-09-05 사용자 지적 4 — `livePrice` prop 참조).

    시리즈가 스스로 적는 「마지막 봉 종가」를 끄고, 우리가 만든 선 하나가 그 자리를
    대신한다. 색은 전일 대비 방향을 따른다 — 마지막 봉의 종가가 아니라 **그 봉의
    시가**와 견준다. 일봉에서 그 둘의 차이가 곧 그날의 등락이고, 헤더가 빨강일 때
    표만 파랑인 일이 생기지 않는다.

    `livePrice`를 안 받는 화면(미니 차트 등)은 예전 그대로 시리즈에 맡긴다.
  */
  useEffect(() => {
    const series = priceSeriesRef.current;
    if (!series) return;

    if (!(livePrice > 0)) {
      if (livePriceLineRef.current) {
        series.removePriceLine(livePriceLineRef.current);
        livePriceLineRef.current = null;
      }
      series.applyOptions({ lastValueVisible: true, priceLineVisible: true });
      return;
    }

    series.applyOptions({ lastValueVisible: false, priceLineVisible: false });

    const lastBar = ohlcRef.current[ohlcRef.current.length - 1];
    const rising = !lastBar || livePrice >= lastBar.open;
    const options = {
      price: livePrice,
      color: rising ? UP_COLOR : DOWN_COLOR,
      lineWidth: 1 as const,
      lineStyle: LineStyle.Dashed,
      axisLabelVisible: true,
      title: "",
    };

    if (livePriceLineRef.current) livePriceLineRef.current.applyOptions(options);
    else livePriceLineRef.current = series.createPriceLine(options);
  }, [livePrice, candles, chartStyle, interval, currency, daily, height]);

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
          followLiveEdge(chart, ohlc.length, span, interval);
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
      const exact = applyDefaultVisibleRange(chart, ohlc, interval);
      shouldFitRef.current = false;
      pinnedToLiveRef.current = true;
      // 폭을 모른 채 잡았으면 폭이 생기는 대로 다시 잡는다(위 ref 주석 참조).
      if (!exact) retryRightEdge();
    } else if (pinned && ohlc.length > prevCount && prevCount > 0 && !isDailyCandleInterval(interval)) {
      const span = visibleRange
        ? visibleRange.to - visibleRange.from
        : intradayInitialVisibleBars(interval);
      followLiveEdge(chart, ohlc.length, span, interval);
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
  }, [candles, interval, chartStyle, retryRightEdge]);

  return (
    /*
      최소 높이는 **이 차트가 실제로 그려지는 높이**다.

      예전에는 `min-h-[240px]`로 못박혀 있었다. 그런데 lightweight-charts는 위에서
      넘긴 `height`만큼의 캔버스를 만들므로, 그보다 작은 높이를 요청한 화면에서는
      캔버스 아래에 **차이만큼의 빈 칸**이 남았다 — 모바일 개요 탭의 일봉 카드가
      200px 차트에 240px 상자라 카드 밑단에 40px짜리 흰 띠가 붙어 있었다.

      값을 `height`에서 가져오면 두 숫자가 어긋날 자리가 없어진다.
    */
    <div className="relative h-full w-full" style={{ minHeight: height }}>
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
          <p className="text-cell font-semibold">{tooltip.timeLabel}</p>
          <dl className="mt-1.5 space-y-0.5 font-num text-micro leading-relaxed">
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
