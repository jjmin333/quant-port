"use client";

import { useCallback, useEffect, useState } from "react";
import { Area, AreaChart, Tooltip, XAxis, YAxis } from "recharts";
import { PageHeader } from "@/components/layout/header";
import { Card, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { fetchPortfolioHistoryClient } from "@/lib/api-client";
import { formatKRW } from "@/lib/format";

const CHART_HEIGHT = 240;

type ChartPoint = { period: string; value: number };

export function BacktestClient() {
  const [mounted, setMounted] = useState(false);
  const [width, setWidth] = useState(600);
  const [data, setData] = useState<ChartPoint[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const loadHistory = useCallback(async () => {
    setLoading(true);
    try {
      const history = await fetchPortfolioHistoryClient("1y");
      const points = history.points.map((p) => ({
        period:
          p.label ||
          new Date(p.at).toLocaleDateString("ko-KR", { month: "short", day: "numeric" }),
        value: p.value,
      }));
      setData(points);
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : "히스토리 조회 실패");
      setData([]);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    setMounted(true);
    void loadHistory();

    const updateWidth = () => {
      const container = document.getElementById("backtest-chart");
      if (container) {
        setWidth(Math.max(container.clientWidth, 280));
      }
    };

    updateWidth();
    window.addEventListener("resize", updateWidth);
    return () => window.removeEventListener("resize", updateWidth);
  }, [loadHistory]);

  return (
    <>
      <PageHeader title="백테스트" subtitle="과거 자산 추이 · 전략 시뮬레이션" />
      <main className="flex-1 overflow-y-auto p-6">
        <div className="mx-auto min-w-[var(--dashboard-content-min)] max-w-5xl space-y-6">
          {error && (
            <Card className="border-rose-500/40 bg-rose-500/5 p-4 text-sm text-rose-300">
              {error}
            </Card>
          )}

          <Card>
            <CardHeader>
              <CardTitle>1년 자산 추이</CardTitle>
              <Badge variant="info">실제 스냅샷</Badge>
            </CardHeader>
            <div id="backtest-chart" className="h-60 w-full">
              {mounted && !loading ? (
                data.length > 0 ? (
                  <AreaChart width={width} height={CHART_HEIGHT} data={data}>
                    <defs>
                      <linearGradient id="backtestGradient" x1="0" y1="0" x2="0" y2="1">
                        <stop offset="0%" stopColor="#8B5CF6" stopOpacity={0.3} />
                        <stop offset="100%" stopColor="#8B5CF6" stopOpacity={0} />
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
                      stroke="#8B5CF6"
                      strokeWidth={2}
                      fill="url(#backtestGradient)"
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

          <Card className="border-dashed border-[var(--border)]">
            <CardHeader>
              <CardTitle>전략 시뮬레이션</CardTitle>
              <Badge variant="info">준비 중</Badge>
            </CardHeader>
            <p className="text-sm text-[var(--text-secondary)]">
              적립식 매수, 리밸런싱 등 투자 전략의 과거 수익률 시뮬레이션 기능은 추후 제공 예정입니다.
              현재는 실제 포트폴리오 1년 자산 추이만 표시합니다.
            </p>
          </Card>
        </div>
      </main>
    </>
  );
}
