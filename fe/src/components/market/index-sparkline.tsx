"use client";

import { cn } from "@/lib/utils";

type IndexSparklineProps = {
  points: number[];
  changeRate: number;
  className?: string;
};

function buildSparklinePath(points: number[], width: number, height: number): string {
  const min = Math.min(...points);
  const max = Math.max(...points);
  const range = max - min || 1;
  const step = points.length > 1 ? width / (points.length - 1) : width;

  return points
    .map((value, index) => {
      const x = index * step;
      const y = height - ((value - min) / range) * height;
      return `${index === 0 ? "M" : "L"}${x.toFixed(2)},${y.toFixed(2)}`;
    })
    .join(" ");
}

export function IndexSparkline({ points, changeRate, className }: IndexSparklineProps) {
  if (points.length < 2) return null;

  const width = 88;
  const height = 28;
  const path = buildSparklinePath(points, width, height);
  const stroke =
    changeRate >= 0 ? "var(--profit)" : changeRate < 0 ? "var(--loss)" : "var(--text-muted)";

  return (
    <svg
      viewBox={`0 0 ${width} ${height}`}
      className={cn("w-full max-w-[5.5rem] opacity-80", className)}
      aria-hidden
    >
      <path
        d={path}
        fill="none"
        stroke={stroke}
        strokeWidth="1.5"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
    </svg>
  );
}
