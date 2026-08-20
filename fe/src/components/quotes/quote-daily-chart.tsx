"use client";

import { useRef } from "react";
import { Card } from "@/components/ui/card";
import { QuoteChartSlot } from "@/components/quotes/quote-chart-slot";
import type { CandlestickChartHandle } from "@/components/quotes/candlestick-chart";
import { ChartZoomToolbar } from "@/components/quotes/chart-zoom-toolbar";
import { useQuoteCandles } from "@/hooks/use-quote-candles";
import { useQuotesChartPrefs } from "@/hooks/use-quotes-chart-prefs";
import type { QuoteCurrency } from "@/lib/quotes-types";
import { cn } from "@/lib/utils";

type QuoteDailyChartProps = {
  symbol: string;
  currency: QuoteCurrency;
  livePrice: number;
  compact?: boolean;
  height?: number;
  /** 왼쪽 끝으로 스크롤하면 과거 일봉을 더 불러온다. 모달처럼 차트가 본문일 때. */
  enableLoadMore?: boolean;
  className?: string;
};

/**
 * 실시간 시세 개요 탭의 일봉 차트.
 *
 * `/quotes` 개요와 AI 추천 랭킹 모달이 같은 그림을 그린다. 여기서만 고치면
 * 두 화면의 일봉이 어긋나지 않는다.
 */
export function QuoteDailyChart({
  symbol,
  currency,
  livePrice,
  compact = false,
  height,
  enableLoadMore = false,
  className,
}: QuoteDailyChartProps) {
  const chartRef = useRef<CandlestickChartHandle>(null);
  const { chartStyle } = useQuotesChartPrefs();
  const { candles, loading, hasMore, loadingMore, loadMore } = useQuoteCandles(
    symbol,
    "1d",
    livePrice,
  );
  const chartHeight = height ?? (compact ? 200 : 240);

  return (
    <Card className={cn("overflow-hidden p-0", className)}>
      <div className="flex items-center justify-between border-b border-[var(--border)] px-3 py-2">
        <h2 className="text-xs font-medium text-[var(--text-secondary)]">일봉 차트</h2>
        <ChartZoomToolbar
          className="scale-90"
          onZoomIn={() => chartRef.current?.zoomIn()}
          onZoomOut={() => chartRef.current?.zoomOut()}
          onReset={() => chartRef.current?.resetZoom()}
        />
      </div>
      <div className={cn("p-1", compact ? "min-h-[200px]" : "min-h-[240px]")}>
        <QuoteChartSlot
          chartRef={chartRef}
          candles={candles}
          loading={loading}
          currency={currency}
          interval="1d"
          chartStyle={chartStyle}
          height={chartHeight}
          hasMore={enableLoadMore ? hasMore : false}
          loadingMore={enableLoadMore ? loadingMore : false}
          onLoadMore={enableLoadMore ? loadMore : undefined}
        />
      </div>
    </Card>
  );
}
