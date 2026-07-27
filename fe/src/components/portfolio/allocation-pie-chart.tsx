"use client";

import { useEffect, useRef, useState } from "react";
import { Cell, Pie, PieChart, Tooltip } from "recharts";
import { Card, CardHeader, CardTitle } from "@/components/ui/card";
import type { GenericAllocationSlice } from "@/lib/portfolio-allocation";
import { formatKRW } from "@/lib/format";

const CHART_MAX = 176;
const CHART_MIN = 120;

function AllocationTooltip({
  active,
  payload,
}: {
  active?: boolean;
  payload?: Array<{ payload: GenericAllocationSlice }>;
}) {
  if (!active || !payload?.length) return null;
  const item = payload[0].payload;

  return (
    <div className="rounded-lg border border-[var(--border)] bg-[var(--surface-elevated)] px-3 py-2 text-xs shadow-lg">
      <p className="font-medium text-[var(--text-primary)]">{item.name}</p>
      <p className="mt-1 tabular-nums text-[var(--text-secondary)]">
        {item.value}% · {formatKRW(item.amountKRW)}
      </p>
    </div>
  );
}

function useContainerChartSize(containerRef: React.RefObject<HTMLDivElement | null>) {
  const [chartSize, setChartSize] = useState(CHART_MAX);

  useEffect(() => {
    const container = containerRef.current;
    if (!container) return;

    const update = () => {
      const width = container.clientWidth;
      const isWide = width >= 420;
      if (!isWide) {
        setChartSize(CHART_MIN);
        return;
      }
      setChartSize(Math.round(Math.min(CHART_MAX, Math.max(CHART_MIN, width * 0.38))));
    };

    update();
    const observer = new ResizeObserver(update);
    observer.observe(container);
    return () => observer.disconnect();
  }, [containerRef]);

  return chartSize;
}

export function AllocationPieChart({
  title,
  slices,
}: {
  title: string;
  slices: GenericAllocationSlice[];
}) {
  const containerRef = useRef<HTMLDivElement>(null);
  const [mounted, setMounted] = useState(false);
  const chartSize = useContainerChartSize(containerRef);

  useEffect(() => {
    setMounted(true);
  }, []);

  const innerRadius = chartSize * 0.28;
  const outerRadius = chartSize * 0.42;

  if (slices.length === 0) {
    return (
      <Card className="h-full min-w-0 overflow-hidden">
        <CardHeader>
          <CardTitle>{title}</CardTitle>
        </CardHeader>
        <p className="text-sm text-[var(--text-muted)]">보유 자산 데이터가 없습니다.</p>
      </Card>
    );
  }

  return (
    <Card className="@container h-full min-w-0 overflow-hidden">
      <CardHeader>
        <CardTitle>{title}</CardTitle>
      </CardHeader>

      <div
        ref={containerRef}
        className="flex min-w-0 flex-col items-stretch gap-4 @min-[420px]:flex-row @min-[420px]:items-center"
      >
        <div
          className="mx-auto flex shrink-0 items-center justify-center @min-[420px]:mx-0"
          style={{ width: chartSize, height: chartSize }}
        >
          {mounted ? (
            <PieChart width={chartSize} height={chartSize}>
              <Pie
                data={slices}
                cx={chartSize / 2}
                cy={chartSize / 2}
                innerRadius={innerRadius}
                outerRadius={outerRadius}
                paddingAngle={2}
                dataKey="amountKRW"
                stroke="transparent"
                isAnimationActive={false}
              >
                {slices.map((entry) => (
                  <Cell key={entry.key} fill={entry.color} />
                ))}
              </Pie>
              <Tooltip content={<AllocationTooltip />} />
            </PieChart>
          ) : (
            <div
              className="animate-pulse rounded-full bg-[var(--surface-elevated)]"
              style={{ width: chartSize, height: chartSize }}
              aria-hidden
            />
          )}
        </div>

        <ul className="flex min-w-0 flex-1 flex-col gap-2">
          {slices.map((item) => (
            <li
              key={item.key}
              className="flex min-w-0 items-center justify-between gap-2 text-sm"
            >
              <span className="flex min-w-0 items-center gap-2 text-[var(--text-secondary)]">
                <span
                  className="h-2.5 w-2.5 shrink-0 rounded-full"
                  style={{ backgroundColor: item.color }}
                />
                <span className="whitespace-nowrap">{item.name}</span>
              </span>
              <span className="shrink-0 font-sans tabular-nums text-[var(--text-primary)]">
                {item.value}%
              </span>
            </li>
          ))}
        </ul>
      </div>
    </Card>
  );
}
