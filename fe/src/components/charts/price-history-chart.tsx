"use client";

import { useMemo } from "react";
import { formatPercent, formatPrice } from "@/lib/format";
import type { QuoteCurrency } from "@/lib/quotes-types";
import { cn } from "@/lib/utils";

/**
 * 모달용 종가 추이 차트 — 관심종목 퀵룩과 보유종목 상세가 함께 쓴다.
 *
 * 이전 스파크라인의 문제는 **눈금이 없다는 것**이었다. 세로를 늘 최저~최고로 꽉 채우니
 * 0.5% 등락도 40% 급등과 똑같이 극적으로 보였고, 금액이 어디에도 없어 그 급등이 몇
 * %인지 화면에서 알 방법이 없었다. 게다가 선이 상자 위아래 가장자리에 정확히 닿아 잘렸다.
 *
 * 그래서 셋을 더한다 —
 *  1. **최고·최저 금액 라벨**과 위아래 여백(선이 잘리지 않는다)
 *  2. **기간 변화율**을 제목 줄 오른쪽에(급등의 크기를 한 번에 말한다)
 *  3. **시작가 기준선**(회색 점선) — 지금이 40일 전보다 위인지 아래인지
 *
 * 금액 라벨을 SVG `<text>`가 아니라 옆 HTML 칸으로 뺀 이유: SVG를 가로로 늘리면
 * `<text>`도 같이 늘어나 글자가 찌그러진다. 세로 배율은 1이라(뷰박스 높이 = CSS 높이)
 * 뷰박스 y좌표를 그대로 px로 써서 라벨 위치를 맞출 수 있다.
 */

const PLOT_W = 340;
const PAD_Y = 10;

type Props = {
  points: number[];
  /** 각 점의 시각(ISO). 있으면 시작·종료 날짜를 아래에 적는다. */
  times?: string[];
  currency: QuoteCurrency;
  /** 평단처럼 겹쳐 그릴 기준선. 표시 범위 밖이면 그리지 않는다. */
  markerPrice?: number | null;
  markerLabel?: string;
  title?: string;
  height?: number;
  className?: string;
  /**
   * 지수는 통화가 아니다. `currency`로 찍으면 KOSPI 7,097이 「7,097원」이 된다 —
   * 원화 7천원짜리가 아니라 지수 포인트다.
   */
  valueFormat?: "price" | "plain";
};

export function PriceHistoryChart({
  points,
  times,
  currency,
  markerPrice,
  markerLabel = "평단",
  title = "최근 흐름",
  height = 96,
  className,
  valueFormat = "price",
}: Props) {
  const formatValue = (v: number) =>
    valueFormat === "plain"
      ? v.toLocaleString("ko-KR", { maximumFractionDigits: 2 })
      : formatPrice(v, currency);

  const chart = useMemo(() => {
    const values = points.filter((v) => Number.isFinite(v) && v > 0);
    if (values.length < 2) return null;

    const min = Math.min(...values);
    const max = Math.max(...values);
    // 완전한 횡보(min === max)도 선 하나는 그려야 한다 — 0으로 나누지 않도록.
    const span = max - min || Math.abs(max) || 1;
    const plotH = height - PAD_Y * 2;
    const yFor = (v: number) => PAD_Y + (1 - (v - min) / span) * plotH;
    const xFor = (i: number) => (i / (values.length - 1)) * PLOT_W;

    const line = values
      .map((v, i) => `${i === 0 ? "M" : "L"}${xFor(i).toFixed(1)},${yFor(v).toFixed(1)}`)
      .join(" ");
    // 면적은 선을 따라가다 바닥으로 떨어져 닫는다. 바닥은 상자 끝(height)이라
    // 그라데이션이 아래로 자연스럽게 사라진다.
    const area = `${line} L${PLOT_W},${height} L0,${height} Z`;

    const first = values[0];
    const last = values[values.length - 1];
    const changeRate = first > 0 ? ((last - first) / first) * 100 : 0;

    const marker =
      markerPrice != null && markerPrice >= min && markerPrice <= max ? yFor(markerPrice) : null;

    return {
      line,
      area,
      min,
      max,
      last,
      lastY: yFor(last),
      baselineY: yFor(first),
      changeRate,
      rising: last >= first,
      marker,
      count: values.length,
    };
  }, [points, height, markerPrice]);

  if (!chart) {
    return (
      <div className={className}>
        <ChartTitleRow title={title} />
        <div className="animate-pulse rounded bg-muted" style={{ height }} />
      </div>
    );
  }

  const stroke = chart.rising ? "var(--profit)" : "var(--loss)";
  const gradientId = `phc-${chart.rising ? "up" : "down"}`;

  return (
    <div className={className}>
      <ChartTitleRow
        title={title}
        right={
          <span
            className={cn(
              "font-mono text-[11px] font-bold tabular-nums",
              chart.rising ? "text-[var(--profit)]" : "text-[var(--loss)]",
            )}
          >
            {formatPercent(chart.changeRate)}
          </span>
        }
      />

      <div className="flex items-stretch gap-2">
        <svg
          viewBox={`0 0 ${PLOT_W} ${height}`}
          preserveAspectRatio="none"
          className="min-w-0 flex-1"
          style={{ height }}
          aria-hidden
        >
          <defs>
            <linearGradient id={gradientId} x1="0" y1="0" x2="0" y2="1">
              <stop offset="0%" stopColor={stroke} stopOpacity="0.2" />
              <stop offset="100%" stopColor={stroke} stopOpacity="0" />
            </linearGradient>
          </defs>

          {/* 최고·최저 눈금 */}
          <line
            x1="0"
            y1={PAD_Y}
            x2={PLOT_W}
            y2={PAD_Y}
            stroke="var(--border)"
            strokeDasharray="3 3"
            vectorEffect="non-scaling-stroke"
          />
          <line
            x1="0"
            y1={height - PAD_Y}
            x2={PLOT_W}
            y2={height - PAD_Y}
            stroke="var(--border)"
            strokeDasharray="3 3"
            vectorEffect="non-scaling-stroke"
          />

          <path d={chart.area} fill={`url(#${gradientId})`} />

          {/* 기간 시작가 — 지금이 그 위인지 아래인지 */}
          <line
            x1="0"
            y1={chart.baselineY}
            x2={PLOT_W}
            y2={chart.baselineY}
            stroke="var(--text-muted)"
            strokeDasharray="2 3"
            opacity="0.6"
            vectorEffect="non-scaling-stroke"
          />

          {chart.marker != null && (
            <line
              x1="0"
              y1={chart.marker}
              x2={PLOT_W}
              y2={chart.marker}
              stroke="var(--text-secondary)"
              strokeDasharray="4 2"
              vectorEffect="non-scaling-stroke"
            />
          )}

          <path
            d={chart.line}
            fill="none"
            stroke={stroke}
            strokeWidth="1.6"
            strokeLinecap="round"
            strokeLinejoin="round"
            vectorEffect="non-scaling-stroke"
          />
          {/* 끝점 — 지금 어디에 있는지 눈이 붙잡을 곳 */}
          <circle cx={PLOT_W} cy={chart.lastY} r="2.6" fill={stroke} vectorEffect="non-scaling-stroke" />
        </svg>

        {/* 금액 눈금. 세로 배율이 1이라 뷰박스 y를 px로 그대로 쓴다. */}
        <div
          className="relative w-[4.75rem] shrink-0 font-mono text-[10px] tabular-nums text-[var(--text-muted)]"
          style={{ height }}
        >
          <span className="absolute right-0 leading-none" style={{ top: PAD_Y - 4 }}>
            {formatValue(chart.max)}
          </span>
          {chart.marker != null && (
            <span
              className="absolute right-0 leading-none text-[var(--text-secondary)]"
              style={{ top: chart.marker - 4 }}
            >
              {markerLabel}
            </span>
          )}
          <span className="absolute right-0 leading-none" style={{ top: height - PAD_Y - 4 }}>
            {formatValue(chart.min)}
          </span>
        </div>
      </div>

      <div className="flex items-baseline justify-between font-mono text-[10px] text-[var(--text-muted)]">
        <span>{formatChartDate(times, 0)}</span>
        {/*
          "N거래일"이라고 쓰지 않는다. 일별 종가가 빠짐없이 쌓인 종목만 그 말이 맞는데,
          커버리지가 성근 종목은 40개 봉이 8개월에 걸쳐 있다(실측: 12-16 ~ 08-14).
          세는 것은 봉의 개수이므로 그렇게만 말하고, 기간은 양옆 날짜가 답한다.
        */}
        <span className="font-sans">종가 {chart.count}개</span>
        <span className="pr-[4.75rem]">{formatChartDate(times, -1)}</span>
      </div>
    </div>
  );
}

function ChartTitleRow({ title, right }: { title: string; right?: React.ReactNode }) {
  return (
    <div className="mb-1 flex items-baseline justify-between gap-2">
      <span className="text-[11px] font-semibold text-muted-foreground">{title}</span>
      {right}
    </div>
  );
}

/** `index` 는 0(처음) 또는 -1(마지막). 시각이 없으면 빈 칸으로 둔다. */
function formatChartDate(times: string[] | undefined, index: number): string {
  if (!times || times.length === 0) return "";
  const raw = index < 0 ? times[times.length - 1] : times[index];
  if (!raw) return "";
  const d = new Date(raw);
  if (Number.isNaN(d.getTime())) return "";
  return d.toLocaleDateString("en-CA", { timeZone: "Asia/Seoul" }).slice(5);
}
