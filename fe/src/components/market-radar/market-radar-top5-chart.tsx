"use client";

import { useEffect, useMemo, useState } from "react";
import {
  CartesianGrid,
  Line,
  LineChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";
import { fetchCandlesClient } from "@/lib/market-api";
import type { EnrichedRankingItem } from "@/lib/market-radar-types";
import type { Candle } from "@/lib/quotes-types";
import { cn } from "@/lib/utils";

type MarketRadarTop5ChartProps = {
  items: EnrichedRankingItem[];
  /** Taller plot + larger legend text for touch viewports. */
  compact?: boolean;
  className?: string;
};

type SeriesRow = Record<string, number | string> & { time: string };

const SERIES_COLORS = ["#8b5cf6", "#06b6d4", "#f59e0b", "#10b981", "#f43f5e"];
const CHART_WINDOW_DAYS = 60;

export function MarketRadarTop5Chart({ items, compact = false, className }: MarketRadarTop5ChartProps) {
  const top5 = useMemo(() => items.slice(0, 5), [items]);
  const [series, setSeries] = useState<SeriesRow[]>([]);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    if (top5.length === 0) {
      setSeries([]);
      return;
    }
    let cancelled = false;
    setLoading(true);
    Promise.all(top5.map((item) => fetchCandlesClient(item.symbol, "1d")))
      .then((candleSets) => {
        if (cancelled) return;
        setSeries(buildAlignedSeries(top5, candleSets));
      })
      .catch(() => {
        if (!cancelled) setSeries([]);
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [top5]);

  if (top5.length === 0) return null;

  return (
    <div className={cn("shrink-0 border-t border-[var(--border)] bg-[var(--surface)] p-3", className)}>
      <div className="flex items-baseline justify-between gap-2">
        <h3 className="text-xs font-medium text-[var(--text-secondary)]">TOP 5 상대 수익률</h3>
        <span className="text-[10px] text-[var(--text-muted)]">100 = 기간 시작</span>
      </div>

      {loading && series.length === 0 ? (
        <p className="mt-2 text-[11px] text-[var(--text-muted)]">차트 로딩 중…</p>
      ) : series.length === 0 ? (
        <p className="mt-2 text-[11px] text-[var(--text-muted)]">일봉 데이터가 부족합니다.</p>
      ) : (
        <>
          <div className={cn("mt-2 w-full", compact ? "h-[168px]" : "h-[132px]")}>
            <ResponsiveContainer width="100%" height="100%">
              <LineChart data={series} margin={{ top: 4, right: 6, left: 0, bottom: 0 }}>
                <CartesianGrid stroke="var(--border)" strokeDasharray="3 3" vertical={false} />
                <XAxis
                  dataKey="time"
                  tickFormatter={formatAxisDate}
                  tick={{ fontSize: 9, fill: "var(--text-muted)" }}
                  minTickGap={28}
                  interval="preserveStartEnd"
                  axisLine={false}
                  tickLine={false}
                />
                <YAxis
                  domain={["auto", "auto"]}
                  tick={{ fontSize: 10, fill: "var(--text-muted)" }}
                  width={34}
                  axisLine={false}
                  tickLine={false}
                />
                <Tooltip content={<Top5Tooltip items={top5} />} />
                {top5.map((item, idx) => (
                  <Line
                    key={item.symbol}
                    type="monotone"
                    dataKey={item.symbol}
                    name={seriesLabel(item)}
                    stroke={SERIES_COLORS[idx % SERIES_COLORS.length]}
                    dot={false}
                    strokeWidth={1.75}
                    connectNulls
                  />
                ))}
              </LineChart>
            </ResponsiveContainer>
          </div>

          <ul className="mt-2 flex flex-wrap gap-1.5">
            {top5.map((item, idx) => (
              <li
                key={item.symbol}
                className={cn(
                  "inline-flex max-w-full items-center gap-1 rounded-md border border-[var(--border)] bg-[var(--surface-elevated)] text-[var(--text-secondary)]",
                  compact ? "px-2 py-1 text-[11px]" : "px-1.5 py-0.5 text-[10px]",
                )}
              >
                <span
                  className="inline-block h-2 w-2 shrink-0 rounded-full"
                  style={{ backgroundColor: SERIES_COLORS[idx % SERIES_COLORS.length] }}
                  aria-hidden
                />
                <span className="font-semibold tabular-nums text-[var(--text-primary)]">{item.rank}</span>
                <span className="truncate">{displayName(item)}</span>
              </li>
            ))}
          </ul>
        </>
      )}
    </div>
  );
}

function Top5Tooltip({
  active,
  payload,
  label,
  items,
}: {
  active?: boolean;
  payload?: Array<{ dataKey?: string | number; value?: number | string; color?: string }>;
  label?: string;
  items: EnrichedRankingItem[];
}) {
  if (!active || !payload?.length) return null;

  const sorted = [...payload]
    .filter((entry) => entry.value != null && Number.isFinite(Number(entry.value)))
    .sort((a, b) => Number(b.value) - Number(a.value));

  return (
    <div className="rounded-lg border border-[var(--border)] bg-[var(--surface)] px-2.5 py-2 text-[11px] shadow-lg">
      <p className="mb-1.5 font-medium text-[var(--text-primary)]">{formatTooltipDate(String(label ?? ""))}</p>
      <ul className="space-y-0.5">
        {sorted.map((entry) => {
          const symbol = String(entry.dataKey ?? "");
          const item = items.find((row) => row.symbol === symbol);
          return (
            <li key={symbol} className="flex items-center justify-between gap-3">
              <span className="flex min-w-0 items-center gap-1.5 text-[var(--text-secondary)]">
                <span
                  className="inline-block h-1.5 w-1.5 shrink-0 rounded-full"
                  style={{ backgroundColor: entry.color }}
                  aria-hidden
                />
                <span className="truncate">
                  {item ? `${item.rank}. ${displayName(item)}` : symbol}
                </span>
              </span>
              <span className="shrink-0 tabular-nums font-medium text-[var(--text-primary)]">
                {Number(entry.value).toFixed(1)}
              </span>
            </li>
          );
        })}
      </ul>
    </div>
  );
}

function buildAlignedSeries(items: EnrichedRankingItem[], candleSets: Candle[][]): SeriesRow[] {
  const normalized = items.map((item, idx) => ({
    symbol: item.symbol,
    byDate: normalizeCandlesByDate(candleSets[idx] ?? []),
  }));

  const dateSet = new Set<string>();
  for (const row of normalized) {
    for (const date of row.byDate.keys()) dateSet.add(date);
  }
  const dates = [...dateSet].sort();
  if (dates.length < 2) return [];

  const window = dates.slice(-CHART_WINDOW_DAYS);
  return window.map((date) => {
    const row: SeriesRow = { time: date };
    for (const { symbol, byDate } of normalized) {
      const value = byDate.get(date);
      if (value != null) row[symbol] = value;
    }
    return row;
  });
}

function normalizeCandlesByDate(candles: Candle[]): Map<string, number> {
  const sorted = [...candles].sort((a, b) => a.time.localeCompare(b.time));
  const closes = sorted.map((c) => c.close).filter((v) => v > 0);
  if (closes.length === 0) return new Map();

  const base = closes[0];
  const byDate = new Map<string, number>();
  for (const candle of sorted) {
    if (candle.close <= 0) continue;
    const date = candle.time.slice(0, 10);
    byDate.set(date, (candle.close / base) * 100);
  }
  return byDate;
}

function displayName(item: EnrichedRankingItem): string {
  const name = item.name?.trim();
  if (name && name !== item.symbol) return name;
  return item.symbol;
}

function seriesLabel(item: EnrichedRankingItem): string {
  return `${item.rank}. ${displayName(item)}`;
}

function formatAxisDate(raw: string): string {
  if (!/^\d{4}-\d{2}-\d{2}/.test(raw)) return raw;
  return raw.slice(5);
}

function formatTooltipDate(raw: string): string {
  if (!/^\d{4}-\d{2}-\d{2}/.test(raw)) return raw;
  const [year, month, day] = raw.split("-");
  return `${year}.${month}.${day}`;
}
