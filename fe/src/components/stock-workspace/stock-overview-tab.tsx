"use client";

import { Card } from "@/components/ui/card";
import {
  METRICS_GRID_VISIBILITY,
  StockMetricsGrid,
  type MetricsRailBreakpoint,
} from "@/components/stock-workspace/stock-metrics-column";
import { StockAISummaryLine } from "@/components/stock-workspace/stock-ai-summary-line";
import { QuoteDailyChart } from "@/components/quotes/quote-daily-chart";
import type { FundamentalsData } from "@/lib/fundamentals-types";
import type { StockSummaryData } from "@/lib/stock-summary-types";
import type { StockWorkspaceTabId } from "@/lib/stock-workspace-tabs";
import type { QuoteDetail, StockInfo } from "@/lib/quotes-types";
import { cn } from "@/lib/utils";

type StockOverviewTabProps = {
  stock: StockInfo;
  quote: QuoteDetail | null;
  fundamentals: FundamentalsData | null;
  onNavigate: (tab: StockWorkspaceTabId) => void;
  compact?: boolean;
  /** Hide chart and live quote summary (종목 분석). */
  analysisMode?: boolean;
  /** Must mirror StockMetricsColumn's rail breakpoint — the grid stands in below it. */
  railFrom?: MetricsRailBreakpoint;
  aiSummary?: StockSummaryData | null;
  aiSummaryLoading?: boolean;
};

export function StockOverviewTab({
  stock,
  quote,
  fundamentals,
  onNavigate,
  compact = false,
  analysisMode = false,
  railFrom = "lg",
  aiSummary = null,
  aiSummaryLoading = false,
}: StockOverviewTabProps) {
  return (
    <div className={cn("space-y-3 p-3", compact && "p-2")}>
      <StockAISummaryLine
        data={aiSummary}
        loading={aiSummaryLoading}
        compact={compact}
        onOpenAnalysis={() => onNavigate("ai")}
      />

      {!analysisMode && quote && quote.currentPrice > 0 && (
        <QuoteDailyChart
          symbol={stock.symbol}
          currency={quote.currency}
          livePrice={quote.currentPrice}
          compact={compact}
        />
      )}

      {/* Below the rail's breakpoint this grid is the only place
          시세/거래/재무/52주 범위 appear. */}
      {!analysisMode && quote && (
        <StockMetricsGrid quote={quote} className={METRICS_GRID_VISIBILITY[railFrom]} />
      )}

      {analysisMode && fundamentals && (
        <Card className="p-3">
          <h2 className="text-xs font-semibold text-[var(--text-primary)]">재무 하이라이트</h2>
          <dl className="mt-2 space-y-1.5 text-[11px]">
            {fundamentals.roe != null && fundamentals.roe !== 0 && (
              <div className="flex justify-between">
                <dt className="text-[var(--text-muted)]">ROE</dt>
                <dd className="font-medium tabular-nums">{fundamentals.roe.toFixed(2)}%</dd>
              </div>
            )}
            {fundamentals.epsBasic != null && fundamentals.epsBasic !== 0 && (
              <div className="flex justify-between">
                <dt className="text-[var(--text-muted)]">EPS</dt>
                <dd className="font-medium tabular-nums">{fundamentals.epsBasic.toFixed(2)}</dd>
              </div>
            )}
          </dl>
        </Card>
      )}
    </div>
  );
}
