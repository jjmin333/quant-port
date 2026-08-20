"use client";

import type { RefObject } from "react";
import {
  CandlestickChart,
  type CandlestickChartHandle,
} from "@/components/quotes/candlestick-chart";
import type { Candle, CandleInterval, ChartStyle } from "@/lib/quotes-types";

type QuoteChartSlotProps = {
  chartRef?: RefObject<CandlestickChartHandle | null>;
  candles: Candle[];
  loading: boolean;
  currency: "KRW" | "USD";
  interval: CandleInterval;
  chartStyle: ChartStyle;
  height?: number;
  hasMore?: boolean;
  loadingMore?: boolean;
  onLoadMore?: () => void;
};

export function QuoteChartSlot({
  chartRef,
  candles,
  loading,
  currency,
  interval,
  chartStyle,
  height = 320,
  hasMore = false,
  loadingMore = false,
  onLoadMore,
}: QuoteChartSlotProps) {
  if (candles.length > 0) {
    return (
      <CandlestickChart
        ref={chartRef}
        candles={candles}
        currency={currency}
        interval={interval}
        chartStyle={chartStyle}
        height={height}
        hasMore={hasMore}
        loadingMore={loadingMore}
        onLoadMore={onLoadMore}
      />
    );
  }

  return (
    <div className="flex h-full min-h-[240px] items-center justify-center text-xs text-[var(--text-muted)]">
      {loading ? "차트 로딩 중…" : "차트 데이터 없음"}
    </div>
  );
}
