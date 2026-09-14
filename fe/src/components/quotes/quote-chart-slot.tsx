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
  /**
   * 축 오른쪽 끝의 현재가 표에 쓸 값 — **화면 헤더가 적는 그 값**을 그대로 넘긴다
   * (2026-09-05 사용자 지적 4). 자세한 이유는 `CandlestickChart`의 `livePrice` 참조.
   */
  livePrice?: number;
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
  livePrice = 0,
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
        livePrice={livePrice}
      />
    );
  }

  /*
    빈 상태도 **차트와 같은 높이**를 쓴다. 240px로 못박아 두면 그보다 낮은 차트를
    쓰는 화면(모바일 개요 200px)에서 데이터가 들어오는 순간 카드가 40px 줄어든다.
  */
  return (
    <div
      className="flex h-full items-center justify-center text-cell text-[var(--text-muted)]"
      style={{ minHeight: height }}
    >
      {loading ? "차트 로딩 중…" : "차트 데이터 없음"}
    </div>
  );
}
