"use client";

import { useMemo } from "react";

import type { Stamp } from "@/lib/stamp-types";
import { PriceDisplay } from "@/components/ui/price-display";

const W = 300;
const H = 64;
const PAD = 4;

/**
 * The bars frozen at stamping time (solid) continued by the closes each horizon
 * bucket resolved to (dashed). Those closes are the only post-stamp prices the
 * stamp itself knows about, so the chart never implies data we did not store.
 */
export function StampSparkline({ stamp }: { stamp: Stamp }) {
  const model = useMemo(() => {
    const frozen = stamp.chart.bars.map((b) => b.close).filter((v) => v > 0);
    if (frozen.length < 2) return null;

    const seen = new Set<string>();
    const after = stamp.returns
      .filter((r) => r.price != null && r.asOfDate)
      .sort((a, b) => (a.asOfDate! < b.asOfDate! ? -1 : 1))
      .filter((r) => {
        if (seen.has(r.asOfDate!)) return false;
        seen.add(r.asOfDate!);
        return true;
      });

    const values = [...frozen, ...after.map((r) => r.price as number)];
    const min = Math.min(...values);
    const max = Math.max(...values);
    const span = max - min || 1;
    const stepX = (W - PAD * 2) / Math.max(1, values.length - 1);

    const point = (v: number, i: number): [number, number] => [
      PAD + i * stepX,
      PAD + (H - PAD * 2) * (1 - (v - min) / span),
    ];

    return {
      frozenPoints: frozen.map(point),
      afterPoints: after.map((r, i) => point(r.price as number, frozen.length + i)),
      stampPoint: point(frozen[frozen.length - 1], frozen.length - 1),
      labels: after.map((r) => r.horizon),
      min,
      max,
    };
  }, [stamp]);

  if (!model) return null;
  const currency = stamp.currency === "USD" ? "USD" : "KRW";
  const path = (pts: [number, number][]) => pts.map(([x, y]) => `${x},${y}`).join(" ");

  return (
    <div>
      <svg viewBox={`0 0 ${W} ${H}`} className="h-16 w-full" preserveAspectRatio="none" role="img">
        <polyline
          points={path(model.frozenPoints)}
          fill="none"
          stroke="var(--text-muted)"
          strokeWidth="1.2"
          vectorEffect="non-scaling-stroke"
        />
        {model.afterPoints.length > 0 && (
          <polyline
            points={path([model.stampPoint, ...model.afterPoints])}
            fill="none"
            stroke="var(--brand)"
            strokeWidth="1.2"
            strokeDasharray="3 2"
            vectorEffect="non-scaling-stroke"
          />
        )}
        <line
          x1={model.stampPoint[0]}
          y1={PAD}
          x2={model.stampPoint[0]}
          y2={H - PAD}
          stroke="var(--brand)"
          strokeWidth="1"
          strokeOpacity="0.4"
          vectorEffect="non-scaling-stroke"
        />
        <circle cx={model.stampPoint[0]} cy={model.stampPoint[1]} r="2.5" fill="var(--brand)" />
        {model.afterPoints.map(([x, y], i) => (
          <circle key={model.labels[i]} cx={x} cy={y} r="2" fill="var(--brand)" fillOpacity="0.7" />
        ))}
      </svg>
      <div className="mt-1 flex justify-between text-micro text-[var(--text-muted)]">
        <span><PriceDisplay value={model.min} currency={currency} size="inherit" /></span>
        <span>
          실선 = 스탬프 시점까지 · 점선 = 이후 {model.labels.join("/") || "구간 없음"}
        </span>
        <span><PriceDisplay value={model.max} currency={currency} size="inherit" /></span>
      </div>
    </div>
  );
}
