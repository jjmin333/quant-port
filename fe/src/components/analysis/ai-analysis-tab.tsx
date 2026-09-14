"use client";

import { DailyCloseTab } from "@/components/analysis/daily-close-tab";
import { MarketBriefing } from "@/components/analysis/market-briefing";
import { StockAISummaryCard } from "@/components/analysis/stock-ai-summary-card";
import type { StockSummaryData } from "@/lib/stock-summary-types";

type AIAnalysisTabProps = {
  symbol: string;
  currency: "KRW" | "USD";
  /** Summary is fetched by the parent so the header line and this tab share one request. */
  data: StockSummaryData | null;
  loading: boolean;
  error?: string | null;
  compact?: boolean;
};

/**
 * The full AI analysis: the summary card plus the rule-based market briefing and
 * the dated score history. It replaces the former 일별 시세 tab, whose close/score
 * list is embedded below — the dates and the reasoning belong together.
 *
 * # 카드 본문은 개요 탭과 공유한다 (2026-09-13 사용자 요청 2)
 *
 * 예전에는 이 파일이 네 섹션 그리드를 **직접** 그렸고 개요 탭은 요약 한 문단을
 * 자기 방식으로 그렸다. 같은 LLM 글의 요약과 본문이 서로 다른 카드 모양을 하고
 * 있었던 것이다. 지금은 `StockAISummaryCard` 하나가 둘을 다 들고, 오른쪽 위
 * 세그먼트 컨트롤이 어느 쪽을 볼지 정한다.
 *
 * 여기서는 `hideWithoutLLM`을 켜지 않는다 — 「AI 분석」 탭을 연 사람에게 빈 화면을
 * 주는 것이 규칙 기반 글을 보여 주는 것보다 나쁘다. `onOpenAnalysis`도 안 넘긴다:
 * 이미 그 탭이다.
 *
 * # 순서: AI 분석이 맨 위다 (2026-09-11 사용자 요청 1)
 *
 * 탭 이름이 「AI 분석」인데 먼저 오는 카드가 규칙 기반 시장 분석이면, 탭을 연
 * 사람이 찾던 것이 두 번째로 밀린다. 시장 분석은 그 글의 배경이므로 아래가
 * 제자리다. 종합 점수 카드는 개요 탭으로 옮겼다(2026-09-03) — 점수는 거의 모든
 * 종목에 있는데 이 탭의 본문은 4%의 종목에만 있어서, 가장 널리 쓰이는 숫자가
 * 가장 드물게 열리는 탭 뒤에 있었다.
 */
export function AIAnalysisTab({
  symbol,
  currency,
  data,
  loading,
  error,
  compact = false,
}: AIAnalysisTabProps) {
  return (
    <div className="space-y-3">
      <StockAISummaryCard
        data={data}
        loading={loading}
        error={error}
        symbol={symbol}
        compact={compact}
      />

      <MarketBriefing briefing={data?.briefing} currency={currency} />

      <DailyCloseTab symbol={symbol} currency={currency} compact={compact} />
    </div>
  );
}
