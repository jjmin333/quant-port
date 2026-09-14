"use client";

import { Card } from "@/components/ui/card";
import {
  METRICS_GRID_VISIBILITY,
  StockMetricsGrid,
  type MetricsRailBreakpoint,
} from "@/components/stock-workspace/stock-metrics-column";
import { StockAISummaryCard } from "@/components/analysis/stock-ai-summary-card";
import { StockScoreCard } from "@/components/analysis/stock-score-card";
import { QuoteDailyChart } from "@/components/quotes/quote-daily-chart";
import type { FundamentalsData } from "@/lib/fundamentals-types";
import type { StockScoreData } from "@/lib/stock-score-api";
import type { StockSummaryData } from "@/lib/stock-summary-types";
import type { QuoteDetail, StockInfo } from "@/lib/quotes-types";
import { cn } from "@/lib/utils";

type StockOverviewTabProps = {
  stock: StockInfo;
  quote: QuoteDetail | null;
  fundamentals: FundamentalsData | null;
  compact?: boolean;
  /** Hide chart and live quote summary (종목 분석). */
  analysisMode?: boolean;
  /** Must mirror StockMetricsColumn's rail breakpoint — the grid stands in below it. */
  railFrom?: MetricsRailBreakpoint;
  aiSummary?: StockSummaryData | null;
  aiSummaryLoading?: boolean;
  /** 종합 점수 — 개요 맨 위에 선다. 부모가 한 번만 받아 여러 탭이 나눠 쓴다. */
  score?: StockScoreData | null;
  scoreLoading?: boolean;
};

export function StockOverviewTab({
  stock,
  quote,
  fundamentals,
  compact = false,
  analysisMode = false,
  railFrom = "lg",
  aiSummary = null,
  aiSummaryLoading = false,
  score = null,
  scoreLoading = false,
}: StockOverviewTabProps) {
  return (
    <div className={cn("space-y-3 p-3", compact && "p-2")}>
      {/*
        **점수가 먼저다** (2026-09-03 사용자 결정). 거의 모든 종목에 있는 값이고,
        AI 요약은 4%의 종목에만 있다 — 있는 쪽을 화면의 첫 자리에 둔다.
      */}
      <StockScoreCard score={score} loading={scoreLoading} />

      {/*
        AI 분석 탭과 **같은 카드**다 (2026-09-13 사용자 요청 2). 오른쪽 위 토글로
        「간단」(요약 한 문단)과 「상세」(이어진 본문)를 오간다 — 기본값은 간단.
        `hideWithoutLLM`은 여기서만 켠다: 이유는 카드 머리글에.

        분석 탭으로 보내는 링크는 뺐다 (2026-09-14 사용자 요청 2) — 상세 버튼이
        그 탭의 본문을 여기서 그대로 펴므로 같은 일을 하는 문이 둘이었다.
      */}
      <StockAISummaryCard
        data={aiSummary}
        loading={aiSummaryLoading}
        compact={compact}
        hideWithoutLLM
        // 종목명·코드를 되풀이하는 태그를 걸러내려면 무엇이 그 값인지 알아야 한다.
        symbol={stock.symbol}
        name={stock.name}
      />

      {!analysisMode && quote && quote.currentPrice > 0 && (
        <QuoteDailyChart
          symbol={stock.symbol}
          currency={quote.currency}
          livePrice={quote.currentPrice}
          compact={compact}
          // 마감 중에는 일봉이 자라지 않는다 — 2초 스냅샷 폴링을 걷는다.
          live={quote.marketSession !== "closed"}
        />
      )}

      {/* Below the rail's breakpoint this grid is the only place
          시세/거래/재무/52주 범위 appear. */}
      {!analysisMode && quote && (
        <StockMetricsGrid quote={quote} className={METRICS_GRID_VISIBILITY[railFrom]} />
      )}

      {analysisMode && fundamentals && (
        <Card className="p-3">
          <h2 className="text-section font-semibold text-[var(--text-primary)]">재무 하이라이트</h2>
          <dl className="mt-2 space-y-1.5 text-cell">
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
