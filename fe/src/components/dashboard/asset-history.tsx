"use client";

import { useCallback, useEffect, useState } from "react";
import { Area, AreaChart, Tooltip, XAxis, YAxis } from "recharts";
import { Card, CardHeader, CardTitle } from "@/components/ui/card";
import { fetchPortfolioHistoryClient } from "@/lib/api-client";
import { formatKRW } from "@/lib/format";
import type { HistoryRange } from "@/lib/insights-types";

const CHART_WIDTH = 600;
const CHART_HEIGHT = 192;

const RANGES: { key: HistoryRange; label: string }[] = [
  { key: "1d", label: "일" },
  { key: "1w", label: "주" },
  { key: "1m", label: "월" },
  { key: "1y", label: "년" },
];

type ChartPoint = { period: string; value: number };

export function AssetHistoryChart({ currentTotal }: { currentTotal: number }) {
  const [mounted, setMounted] = useState(false);
  const [width, setWidth] = useState(CHART_WIDTH);
  const [range, setRange] = useState<HistoryRange>("1m");
  const [data, setData] = useState<ChartPoint[]>([]);
  const [loading, setLoading] = useState(true);

  const loadHistory = useCallback(async (selected: HistoryRange) => {
    setLoading(true);
    try {
      const history = await fetchPortfolioHistoryClient(selected);
      const points =
        history.points.length > 0
          ? history.points.map((p) => ({
              period: p.label || new Date(p.at).toLocaleDateString("ko-KR", { month: "numeric", day: "numeric" }),
              value: p.value,
            }))
          : currentTotal > 0
            ? [{ period: "현재", value: currentTotal }]
            : [];
      setData(points);
    } catch {
      setData(currentTotal > 0 ? [{ period: "현재", value: currentTotal }] : []);
    } finally {
      setLoading(false);
    }
  }, [currentTotal]);

  useEffect(() => {
    setMounted(true);

    const updateWidth = () => {
      const container = document.getElementById("asset-history-chart");
      if (container) {
        setWidth(Math.max(container.clientWidth, 280));
      }
    };

    updateWidth();
    window.addEventListener("resize", updateWidth);
    return () => window.removeEventListener("resize", updateWidth);
  }, []);

  useEffect(() => {
    void loadHistory(range);
  }, [range, loadHistory]);

  return (
    <Card className="min-w-[var(--dashboard-content-min)] shrink-0">
      <CardHeader>
        <CardTitle>자산 변화</CardTitle>
        <div className="flex gap-1">
          {RANGES.map(({ key, label }) => (
            <button
              key={key}
              type="button"
              onClick={() => setRange(key)}
              className={`rounded-md px-2 py-0.5 text-xs hover:bg-[var(--surface-elevated)] hover:text-[var(--text-primary)] ${
                range === key
                  ? "bg-[var(--surface-elevated)] text-[var(--text-primary)]"
                  : "text-[var(--text-muted)]"
              }`}
            >
              {label}
            </button>
          ))}
        </div>
      </CardHeader>
      <div id="asset-history-chart" className="h-48 w-full">
        {mounted && !loading ? (
          data.length > 0 ? (
            <AreaChart width={width} height={CHART_HEIGHT} data={data}>
              <defs>
                <linearGradient id="assetGradient" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="0%" stopColor="#3B82F6" stopOpacity={0.3} />
                  <stop offset="100%" stopColor="#3B82F6" stopOpacity={0} />
                </linearGradient>
              </defs>
              <XAxis
                dataKey="period"
                axisLine={false}
                tickLine={false}
                tick={{ fill: "#64748B", fontSize: 11 }}
              />
              <YAxis
                axisLine={false}
                tickLine={false}
                tick={{ fill: "#64748B", fontSize: 11 }}
                tickFormatter={(v) => `${(Number(v) / 100_000_000).toFixed(1)}억`}
                width={40}
                domain={["auto", "auto"]}
              />
              <Tooltip
                contentStyle={{
                  background: "#1A2030",
                  border: "1px solid #2D3748",
                  borderRadius: "8px",
                  fontSize: "12px",
                }}
                formatter={(value) => [formatKRW(Number(value)), "총자산"]}
              />
              <Area
                type="monotone"
                dataKey="value"
                stroke="#3B82F6"
                strokeWidth={2}
                fill="url(#assetGradient)"
                isAnimationActive={false}
              />
            </AreaChart>
          ) : (
            <div className="flex h-full items-center justify-center text-sm text-[var(--text-muted)]">
              스냅샷 데이터가 쌓이면 차트가 표시됩니다
            </div>
          )
        ) : (
          <div className="h-full w-full animate-pulse rounded-lg bg-[var(--surface-elevated)]" />
        )}
      </div>
    </Card>
  );
}
