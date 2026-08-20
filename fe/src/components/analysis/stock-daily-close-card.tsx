"use client";

import { useEffect, useMemo, useState } from "react";
import {
  SectionCard,
  SectionEmpty,
  SectionSkeleton,
} from "@/components/ui/section-card";
import { fetchStockDailyCloses } from "@/lib/daily-close-api";
import { formatPercent, formatPrice } from "@/lib/format";
import type { DailyCloseRow } from "@/lib/daily-close-types";
import { trendTextClass } from "@/lib/trend-colors";
import { cn } from "@/lib/utils";

/**
 * 일별 종가 추이 — 차트 + 표 (옵션 3 "둘 다", 08-11 채택).
 *
 * 차트는 흐름을, 표는 정확한 날짜·값을 맡는다. 하나만 두면 "지난주 어디였더라"에
 * 답을 못 하거나(차트만) 추세가 안 보인다(표만).
 *
 * 면적 그래프의 색은 **구간 전체의 방향**(첫 종가 → 마지막 종가)으로 정한다. 마지막
 * 하루의 등락으로 칠하면 한 달 내내 오른 종목이 마지막 날 하락했다고 파랗게 나온다.
 */
const DAYS = 30;
const W = 300;
const H = 84;

export function StockDailyCloseCard({
  symbol,
  currency,
}: {
  symbol: string;
  currency: "KRW" | "USD";
}) {
  const [rows, setRows] = useState<DailyCloseRow[] | null>(null);

  useEffect(() => {
    let alive = true;
    fetchStockDailyCloses(symbol, DAYS)
      .then((series) => {
        if (alive) setRows(series.rows ?? []);
      })
      .catch(() => {
        if (alive) setRows([]);
      });
    return () => {
      alive = false;
    };
  }, [symbol]);

  // API는 최신순으로 준다. 차트는 시간순이어야 왼쪽이 과거가 된다.
  const chrono = useMemo(
    () => (rows ? [...rows].reverse().filter((r) => r.close > 0) : []),
    [rows],
  );

  const geometry = useMemo(() => {
    if (chrono.length < 2) return null;
    const closes = chrono.map((r) => r.close);
    const min = Math.min(...closes);
    const max = Math.max(...closes);
    const range = max - min || 1;
    const step = W / (closes.length - 1);
    const pts = closes.map((v, i) => ({
      x: i * step,
      y: H - ((v - min) / range) * (H - 8) - 4,
    }));
    const line = pts.map((p, i) => `${i === 0 ? "M" : "L"}${p.x.toFixed(1)},${p.y.toFixed(1)}`).join(" ");
    const area = `${line} L${W},${H} L0,${H} Z`;
    const spanRate = ((closes[closes.length - 1] - closes[0]) / closes[0]) * 100;
    return { line, area, spanRate, first: chrono[0], last: chrono[chrono.length - 1] };
  }, [chrono]);

  return (
    <SectionCard
      title={`최근 ${DAYS}거래일 종가`}
      href={`/stocks/${encodeURIComponent(symbol)}`}
      hrefLabel="일봉"
    >
      {rows === null ? (
        <SectionSkeleton rows={6} />
      ) : chrono.length === 0 ? (
        <SectionEmpty text="일별 종가 데이터가 없습니다" />
      ) : (
        <>
          {geometry && (
            <>
              <svg
                viewBox={`0 0 ${W} ${H}`}
                preserveAspectRatio="none"
                className="block h-[84px] w-full"
                role="img"
                aria-label={`최근 ${chrono.length}거래일 종가 추이 ${formatPercent(geometry.spanRate)}`}
              >
                <defs>
                  <linearGradient id={`dcg-${symbol}`} x1="0" y1="0" x2="0" y2="1">
                    <stop
                      offset="0%"
                      stopColor={geometry.spanRate >= 0 ? "var(--profit)" : "var(--loss)"}
                      stopOpacity="0.22"
                    />
                    <stop
                      offset="100%"
                      stopColor={geometry.spanRate >= 0 ? "var(--profit)" : "var(--loss)"}
                      stopOpacity="0"
                    />
                  </linearGradient>
                </defs>
                <path d={geometry.area} fill={`url(#dcg-${symbol})`} />
                <path
                  d={geometry.line}
                  fill="none"
                  stroke={geometry.spanRate >= 0 ? "var(--profit)" : "var(--loss)"}
                  strokeWidth="1.6"
                  strokeLinejoin="round"
                  strokeLinecap="round"
                  vectorEffect="non-scaling-stroke"
                />
              </svg>
              <div className="mt-1 flex items-center justify-between text-[11px] text-muted-foreground">
                <span>
                  {geometry.first.tradeDate.slice(5)} ·{" "}
                  {formatPrice(geometry.first.close, currency)}
                </span>
                <span>
                  {geometry.last.tradeDate.slice(5)} ·{" "}
                  <span className="text-foreground">
                    {formatPrice(geometry.last.close, currency)}
                  </span>{" "}
                  <span className={trendTextClass(geometry.spanRate)}>
                    {formatPercent(geometry.spanRate)}
                  </span>
                </span>
              </div>
            </>
          )}

          <div className="mt-2 border-t pt-1.5">
            <div className="flex h-[22px] items-center gap-2 border-b px-1 text-[11px] text-muted-foreground">
              <span className="flex-1">날짜</span>
              <span className="w-[74px] text-right">종가</span>
              <span className="w-[58px] text-right">전일비</span>
            </div>
            <ul className="min-w-0">
              {rows.slice(0, 6).map((r) => (
                <li
                  key={r.tradeDate}
                  className="flex h-[26px] items-center gap-2 px-1 text-xs"
                >
                  <span className="flex-1 truncate">
                    {r.tradeDate.slice(5)}{" "}
                    <span className="text-[11px] text-muted-foreground">({r.weekday})</span>
                  </span>
                  <span className="w-[74px] text-right font-mono tabular-nums">
                    {formatPrice(r.close, currency)}
                  </span>
                  <span
                    className={cn(
                      "w-[58px] text-right font-mono tabular-nums",
                      r.changeRate == null
                        ? "text-muted-foreground"
                        : trendTextClass(r.changeRate),
                    )}
                  >
                    {r.changeRate == null ? "—" : formatPercent(r.changeRate)}
                  </span>
                </li>
              ))}
            </ul>
          </div>
        </>
      )}
    </SectionCard>
  );
}
