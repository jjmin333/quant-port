"use client";

import { useCallback, useState } from "react";
import { EtfCheckCard } from "@/components/analysis/etf-check-card";
import { StockQuestionChips } from "@/components/analysis/stock-question-chips";
import { isDomesticEtf } from "@/lib/etf-score-api";
import {
  METRICS_GRID_VISIBILITY,
  StockMetricsGrid,
  type MetricsRailBreakpoint,
} from "@/components/stock-workspace/stock-metrics-column";
import { StockAISummaryCard } from "@/components/analysis/stock-ai-summary-card";
import { StockScoreCard } from "@/components/analysis/stock-score-card";
import { AnalystConsensusSection } from "@/components/analysis/analyst-consensus-card";
import { PriceScenarioSection } from "@/components/stock-workspace/price-scenario-card";
import type { PriceScenario } from "@/lib/price-scenario-api";
import type { CommunityPost } from "@/lib/community-types";
import { QuoteDailyChart } from "@/components/quotes/quote-daily-chart";
import { StockCommunityCard } from "@/components/community/stock-community-card";
import { COMMUNITY_FEATURES_HIDDEN } from "@/lib/hidden-features";
import type { FundamentalsData } from "@/lib/fundamentals-types";
import type { ScoreChangeSummary } from "@/lib/score-explain-api";
import type { StockScoreData } from "@/lib/stock-score-api";
import type { StockSummaryData } from "@/lib/stock-summary-types";
import type { QuoteDetail, StockInfo } from "@/lib/quotes-types";
import { Card } from "@/components/ui/card";
import { NextEarningsCard, StockWhyLine, WatchPrompt } from "@/components/stock-workspace/stock-overview-parts";
import { cn } from "@/lib/utils";

type StockOverviewTabProps = {
  stock: StockInfo;
  quote: QuoteDetail | null;
  fundamentals: FundamentalsData | null;
  compact?: boolean;
  /** Must mirror StockMetricsColumn's rail breakpoint — the grid stands in below it. */
  railFrom?: MetricsRailBreakpoint;
  aiSummary?: StockSummaryData | null;
  aiSummaryLoading?: boolean;
  /** 종합 점수 — 개요 맨 위에 선다. 부모가 한 번만 받아 여러 탭이 나눠 쓴다. */
  score?: StockScoreData | null;
  scoreLoading?: boolean;
  /** 전일 대비 변화 한 줄 — 부모가 한 번 받아 내려 준다. */
  scoreChange?: ScoreChangeSummary | null;
  /** 점수 근거를 보러 AI 분석 탭으로. */
  onOpenExplain?: () => void;
  /** 90일 시나리오 — 종합 점수 카드 안에 선다(2026-10-07 요청 9). */
  scenario?: PriceScenario | null;
  scenarioLoading?: boolean;
  /** 최근 커뮤니티 글 3건 — 워크스페이스가 탭 노출 판단과 함께 한 번 받는다. `null`은 받는 중. */
  communityPosts?: CommunityPost[] | null;
};

export function StockOverviewTab({
  stock,
  quote,
  fundamentals,
  compact = false,
  railFrom = "lg",
  aiSummary = null,
  aiSummaryLoading = false,
  score = null,
  scoreLoading = false,
  scoreChange = null,
  onOpenExplain,
  scenario = null,
  scenarioLoading = false,
  communityPosts = null,
}: StockOverviewTabProps) {
  /*
    국내 ETF는 점수 자리에 「ETF 점검」을 둔다 (2026-09-24 사용자 결정). 점검이 없으면
    (상장 20거래일 미만 등) 원래 점수 카드로 돌아간다 — 빈 자리를 남기지 않는다.
  */
  const [etfMissingFor, setEtfMissingFor] = useState<string | null>(null);
  const onEtfMissing = useCallback(() => setEtfMissingFor(stock.symbol), [stock.symbol]);
  const showEtfCheck = isDomesticEtf(stock) && etfMissingFor !== stock.symbol;

  /*
    AI 분석 탭과 **같은 카드**다 (2026-09-13 사용자 요청 2). 오른쪽 위 토글로
    「간단」(요약 한 문단)과 「상세」(이어진 본문)를 오간다 — 기본값은 상세(10-07).
    `hideWithoutLLM`은 여기서만 켠다: 이유는 카드 머리글에.
  */
  const aiCard = (
    <StockAISummaryCard
      data={aiSummary}
      loading={aiSummaryLoading}
      compact={compact}
      hideWithoutLLM
      // 종목명·코드를 되풀이하는 태그를 걸러내려면 무엇이 그 값인지 알아야 한다.
      symbol={stock.symbol}
      name={stock.name}
    />
  );
  const showScenario = scenarioLoading || Boolean(scenario && !scenario.unavailable);

  /*
    순서 (2026-10-10 사용자 결정 — 시안 `docs/mockups/business-w4-v1.html` A안):
    왜 움직였나 → 점수(레이더·순위) → 다음 실적 | 90일 범위 → 관심 등록 안내 → AI 요약(다 펼침) → …
    이 서비스만의 것(점수·근거)을 첫 화면에 세운다. 10-07에는 LLM 글이 있으면 맨 위였는데, A안을 고르면서
    점수 아래로 내렸다 — 글은 **접지 않는다**(같은 날 사용자 지시).
  */
  return (
    <div className={cn("space-y-3 p-3", compact && "p-2")}>
      <StockWhyLine symbol={stock.symbol} />
      {showEtfCheck ? (
        <EtfCheckCard symbol={stock.symbol} onMissing={onEtfMissing} />
      ) : (
        <StockScoreCard score={score} loading={scoreLoading} change={scoreChange} onOpenExplain={onOpenExplain} />
      )}

      {/* 한 칸만 있으면 전폭으로 선다(auto-fit). */}
      <div className="grid gap-3 sm:grid-cols-[repeat(auto-fit,minmax(16rem,1fr))] empty:hidden">
        <NextEarningsCard symbol={stock.symbol} />
        {showScenario && (
          <Card className="p-0">
            <PriceScenarioSection scenario={scenario} loading={scenarioLoading} />
          </Card>
        )}
      </div>

      <WatchPrompt symbol={stock.symbol} name={stock.name} />

      {aiCard}

      {/* 질문 칩 — 규칙 기반 종목 Q&A(2026-09-24). 국내 ETF는 주식 점수가 없어 「점수가 왜」를 뺀다. */}
      <StockQuestionChips symbol={stock.symbol} hideScore={showEtfCheck} />

      {/* 애널리스트 의견 — 자료 없으면 안 그린다. */}
      <AnalystConsensusSection
        symbol={stock.symbol}
        price={quote?.currentPrice || fundamentals?.price}
        currency={stock.currency === "KRW" ? "KRW" : "USD"}
      />

      {quote && quote.currentPrice > 0 && (
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
      {quote && (
        <StockMetricsGrid quote={quote} className={METRICS_GRID_VISIBILITY[railFrom]} />
      )}

      {/* 이 종목 이야기 — 커뮤니티 입구 (2026-09-27 요청 4). */}
      {!COMMUNITY_FEATURES_HIDDEN && (
        <StockCommunityCard symbol={stock.symbol} name={stock.name} posts={communityPosts} />
      )}

    </div>
  );
}
