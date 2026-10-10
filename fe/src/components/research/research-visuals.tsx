"use client";

import { Building2, PiggyBank } from "lucide-react";
import { useMemo } from "react";
import { StockAvatar } from "@/components/stocks/stock-avatar";

import { candleTimeSortKey } from "@/lib/chart-time";
import type { CandleInterval } from "@/lib/quotes-types";
import type { ResearchCall } from "@/lib/research-types";
import { cn } from "@/lib/utils";
import { PriceDisplay } from "@/components/ui/price-display";

/**
 * 리포트 목록·모달의 시각 조각.
 *
 * 이 화면의 가장 큰 낭비는 **발행 시점 캔들이 이미 응답에 들어 있는데 목록이 쓰지
 * 않는 것**이었다(`call.chart.bars`). 목표가·손절가·기준가도 전부 같은 응답에 있다.
 * 즉 아래 그림들은 추가 요청 0으로 그려진다.
 */

const PLOT_W = 320;

type Series = { pre: string; post: string; markX: number; markY: number };

/**
 * 발행 시점 스파크 — 발행 전은 회색, 발행 후는 검은 선, 발행 지점에 점 하나.
 * 목표가·손절가는 파선으로 겹친다.
 *
 * 캔들 정렬은 `candleTimeSortKey`로 **먼저 정렬한 뒤** 자른다. 일봉 응답 순서가
 * 출처마다 다르기 때문이다(DB 경로는 오름차순, Toss tail은 최신이 앞) — 정렬 없이
 * 자르면 가장 오래된 구간을 최신인 척 그리게 된다.
 */
export function ResearchSparkline({
  call,
  height = 56,
  className,
  showLegend = true,
}: {
  call: ResearchCall;
  height?: number;
  className?: string;
  showLegend?: boolean;
}) {
  const interval = (call.chart?.interval || "1d") as CandleInterval;
  /*
   * 목록 응답은 `closes`(종가만), 상세 응답은 `bars`(OHLCV)를 준다 — 이 그림이 읽는
   * 값은 어느 쪽이든 시각과 종가뿐이라 둘을 같은 모양으로 받아 쓴다.
   */
  const rawPoints = call.chart?.closes ?? call.chart?.bars;
  const bars = useMemo(
    () =>
      [...(rawPoints ?? [])].sort((a, b) =>
        candleTimeSortKey(a.time, interval).localeCompare(candleTimeSortKey(b.time, interval)),
      ),
    [rawPoints, interval],
  );

  // 메모 안에서는 원시값만 읽는다 — `call.x?.y`를 메모 본문에 두면 React 컴파일러가
  // 기존 메모화를 보존하지 못한다고 판단해 최적화를 통째로 건너뛴다.
  const { basePrice, targetPrice, stopPrice, publishedAt } = call;
  const currentPrice = call.current?.price;

  const chart = useMemo((): (Series & { yFor: (v: number) => number }) | null => {
    const closes = bars.map((b) => b.close).filter((v) => Number.isFinite(v) && v > 0);
    if (closes.length < 2) return null;

    // 목표·손절선이 축 밖으로 나가면 파선이 안 보인다. 값 범위에 함께 넣는다.
    const extra = [basePrice, targetPrice, stopPrice, currentPrice].filter(
      (v): v is number => typeof v === "number" && v > 0,
    );
    const all = [...closes, ...extra];
    const min = Math.min(...all);
    const max = Math.max(...all);
    const span = max - min || Math.abs(max) || 1;
    const pad = 4;
    const yFor = (v: number) => pad + (1 - (v - min) / span) * (height - pad * 2);
    const xFor = (i: number) => (i / (closes.length - 1)) * PLOT_W;

    /*
     * 발행 지점은 발행일과 같거나 그보다 앞선 마지막 봉이다. 못 찾으면(발행 후 봉이
     * 아직 없거나 날짜 형식이 다르면) 마지막 봉을 쓴다 — 점을 아예 안 찍는 것보다
     * "여기서부터"가 보이는 편이 낫다.
     */
    const pubKey = candleTimeSortKey(publishedAt, interval);
    let cut =
      bars.findIndex((b) => candleTimeSortKey(b.time, interval).localeCompare(pubKey) > 0) - 1;
    if (cut < 0) cut = closes.length - 1;
    cut = Math.max(0, Math.min(cut, closes.length - 1));

    const path = (from: number, to: number) =>
      closes
        .slice(from, to + 1)
        .map((v, i) => `${i === 0 ? "M" : "L"}${xFor(from + i).toFixed(1)},${yFor(v).toFixed(1)}`)
        .join(" ");

    return {
      pre: path(0, cut),
      post: path(cut, closes.length - 1),
      markX: xFor(cut),
      markY: yFor(closes[cut]),
      yFor,
    };
  }, [bars, interval, height, basePrice, targetPrice, stopPrice, currentPrice, publishedAt]);

  if (!chart) return null;

  const currency = call.currency ?? "KRW";

  return (
    <div className={cn("min-w-0", className)}>
      <svg
        viewBox={`0 0 ${PLOT_W} ${height}`}
        preserveAspectRatio="none"
        className="block h-auto w-full"
        style={{ height }}
        aria-hidden
      >
        {call.targetPrice !== undefined && (
          <line
            x1="0"
            x2={PLOT_W}
            y1={chart.yFor(call.targetPrice)}
            y2={chart.yFor(call.targetPrice)}
            stroke="var(--profit)"
            strokeWidth="1.2"
            strokeDasharray="5 3"
          />
        )}
        {call.stopPrice !== undefined && (
          <line
            x1="0"
            x2={PLOT_W}
            y1={chart.yFor(call.stopPrice)}
            y2={chart.yFor(call.stopPrice)}
            stroke="var(--loss)"
            strokeWidth="1.2"
            strokeDasharray="5 3"
          />
        )}
        <path
          d={chart.pre}
          fill="none"
          stroke="var(--text-muted)"
          strokeWidth="1.3"
          opacity="0.55"
          vectorEffect="non-scaling-stroke"
        />
        <path
          d={chart.post}
          fill="none"
          stroke="var(--text-primary)"
          strokeWidth="1.7"
          vectorEffect="non-scaling-stroke"
        />
        <circle cx={chart.markX} cy={chart.markY} r="3.2" fill="var(--brand)" />
      </svg>

      {showLegend && (
        <p className="mt-1 flex flex-wrap items-center gap-x-2 text-micro text-[var(--text-muted)]">
          <span className="inline-flex items-center gap-1">
            <span className="inline-block size-1.5 rounded-full bg-[var(--brand)]" />
            발행 <PriceDisplay value={call.basePrice} currency={currency} size="inherit" />
          </span>
          {call.targetPrice !== undefined && (
            <span className="text-[var(--profit)]">
              ┈ 목표 <PriceDisplay value={call.targetPrice} currency={currency} size="inherit" />
            </span>
          )}
          {call.stopPrice !== undefined && (
            <span className="text-[var(--loss)]">
              ┈ 손절 <PriceDisplay value={call.stopPrice} currency={currency} size="inherit" />
            </span>
          )}
        </p>
      )}
    </div>
  );
}

/**
 * 「손절 — 발행가 — 현재가 — 목표」를 한 축에 올린 게이지.
 *
 * `toTargetPct`·`toStopPct`가 응답에 이미 있지만 그건 숫자다. 좌표로 세워야
 * "목표까지 아직 한참" 과 "손절선 코앞"이 한눈에 갈린다.
 */
export function ResearchLevelAxis({ call, className }: { call: ResearchCall; className?: string }) {
  const current = call.current?.price;
  const marks = [call.basePrice, call.targetPrice, call.stopPrice, current].filter(
    (v): v is number => typeof v === "number" && v > 0,
  );
  if (marks.length < 2 || current === undefined) return null;

  const min = Math.min(...marks);
  const max = Math.max(...marks);
  const pad = (max - min) * 0.08 || Math.abs(max) * 0.02 || 1;
  const lo = min - pad;
  const hi = max + pad;
  const at = (v: number) => ((v - lo) / (hi - lo)) * 100;

  const basePos = at(call.basePrice);
  const nowPos = at(current);
  const gaining = call.direction === "sell" ? current < call.basePrice : current >= call.basePrice;

  return (
    <div className={cn("min-w-0", className)}>
      <div className="relative mt-3 h-1.5 rounded-full bg-[var(--surface-elevated)]">
        <span
          className={cn(
            "absolute inset-y-0 rounded-full",
            gaining ? "bg-[var(--brand)]/70" : "bg-[var(--loss)]/60",
          )}
          style={{
            left: `${Math.min(basePos, nowPos)}%`,
            width: `${Math.max(Math.abs(nowPos - basePos), 0.5)}%`,
          }}
        />
        {call.stopPrice !== undefined && (
          <span
            aria-hidden
            className="absolute -top-1 h-3.5 w-0 border-r-2 border-dashed border-[var(--loss)]"
            style={{ left: `${at(call.stopPrice)}%` }}
          />
        )}
        {call.targetPrice !== undefined && (
          <span
            aria-hidden
            className="absolute -top-1 h-3.5 w-0 border-r-2 border-dashed border-[var(--profit)]"
            style={{ left: `${at(call.targetPrice)}%` }}
          />
        )}
        <span
          aria-hidden
          className="absolute -top-0.5 h-2.5 w-0.5 rounded-full bg-[var(--text-muted)]"
          style={{ left: `${basePos}%` }}
        />
        <span
          aria-hidden
          className="absolute -top-1.5 h-4.5 w-[3px] rounded-full bg-[var(--text-primary)]"
          style={{ left: `${nowPos}%` }}
        />
      </div>
      <div className="mt-1 flex items-center justify-between gap-2 text-micro text-[var(--text-muted)]">
        <span className="truncate">
          발행 <PriceDisplay value={call.basePrice} currency={call.currency ?? "KRW"} size="inherit" />
        </span>
        <span className="shrink-0 font-num tabular-nums">
          {call.toTargetPct !== undefined && (
            <span className="text-[var(--profit)]">
              목표까지 {call.toTargetPct > 0 ? "+" : ""}
              {call.toTargetPct.toFixed(1)}%
            </span>
          )}
          {call.toStopPct !== undefined && (
            <span className="ml-2 text-[var(--loss)]">손절 {call.toStopPct.toFixed(1)}%</span>
          )}
        </span>
      </div>
    </div>
  );
}

/** 발행 시점 5축 점수 막대. 축이 하나도 없으면 그리지 않는다. */
export function ResearchAxisBars({ call, className }: { call: ResearchCall; className?: string }) {
  const axes = [
    { label: "추세", value: call.evidence?.breakdown?.technical },
    { label: "수익성", value: call.evidence?.breakdown?.quality },
    { label: "성장", value: call.evidence?.breakdown?.growth },
    { label: "가치", value: call.evidence?.breakdown?.valuation },
    { label: "심리", value: call.evidence?.breakdown?.sentiment },
  ].filter((a): a is { label: string; value: number } => typeof a.value === "number");

  if (axes.length === 0) return null;

  return (
    <div className={cn("space-y-1", className)}>
      {axes.map((a) => (
        <div key={a.label} className="flex items-center gap-1.5">
          <span className="w-8 shrink-0 text-micro text-[var(--text-muted)]">{a.label}</span>
          <span className="h-1 flex-1 overflow-hidden rounded-full bg-[var(--surface-elevated)]">
            <span
              className="block h-full rounded-full bg-[var(--brand)]"
              style={{ width: `${Math.max(0, Math.min(100, a.value))}%` }}
            />
          </span>
          <span className="w-5 shrink-0 text-right font-num text-micro tabular-nums text-[var(--text-secondary)]">
            {Math.round(a.value)}
          </span>
        </div>
      ))}
    </div>
  );
}

/**
 * 카드 표지. 작성자가 올린 썸네일이 없으면 **종목 로고**로 대신한다 —
 * 그림 없는 카드가 섞이면 목록이 들쭉날쭉해지고, 그렇다고 모든 작성자에게
 * 이미지를 요구할 이유는 없다.
 */
export function ResearchThumbnail({
  call,
  className,
}: {
  call: ResearchCall;
  className?: string;
}) {
  if (call.thumbnailUrl) {
    return (
      // 사용자 업로드는 크기가 제각각이라 next/image의 사전 크기 지정이 맞지 않는다.
      // eslint-disable-next-line @next/next/no-img-element
      <img
        src={call.thumbnailUrl}
        alt=""
        loading="lazy"
        className={cn("size-full object-cover", className)}
      />
    );
  }
  if (call.status === "note") {
    // 분석 리포트 — 종목 로고 대신 주제 아이콘(2026-10-09 요청 9).
    const Icon = call.topic === "finance" ? PiggyBank : Building2;
    return (
      <span className={cn("flex size-full items-center justify-center bg-[var(--surface-elevated)]", className)}>
        <Icon className="h-8 w-8 text-[var(--text-muted)]" aria-hidden />
      </span>
    );
  }
  return (
    <span
      className={cn(
        "flex size-full items-center justify-center bg-[var(--surface-elevated)]",
        className,
      )}
    >
      <StockAvatar
        symbol={call.symbol}
        name={call.symbolName}
        currency={call.currency}
        size="lg"
      />
    </span>
  );
}
