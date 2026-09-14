"use client";

import { IndexSparkline } from "@/components/market/index-sparkline";
import { useInView } from "@/hooks/use-in-view";
import { useSparklineCandles } from "@/hooks/use-sparkline-candles";
import { cn } from "@/lib/utils";

type MarketRadarSparklineProps = {
  symbol: string;
  changeRate: number;
  className?: string;
};

function SparklineInner({ symbol, changeRate }: { symbol: string; changeRate: number }) {
  const { points, loading } = useSparklineCandles(symbol, true);

  if (points.length < 2) {
    return <span className="text-micro text-[var(--text-muted)]">{loading ? "…" : ""}</span>;
  }

  return <IndexSparkline points={points} changeRate={changeRate} />;
}

export function MarketRadarSparkline({ symbol, changeRate, className }: MarketRadarSparklineProps) {
  const { ref, inView } = useInView("120px");

  return (
    <div ref={ref} className={cn("flex h-7 w-[5.5rem] items-center justify-end", className)}>
      {inView ? <SparklineInner symbol={symbol} changeRate={changeRate} /> : null}
    </div>
  );
}

export { parseRankingChangeRate as parseItemChangeRate } from "@/lib/market-radar-config";
