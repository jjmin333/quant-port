"use client";

import { useCallback, useState } from "react";
import { EtfCheckCard } from "@/components/analysis/etf-check-card";
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
import { useCommunityVisible } from "@/hooks/use-community-visible";
import type { FundamentalsData } from "@/lib/fundamentals-types";
import type { ScoreChangeSummary } from "@/lib/score-explain-api";
import type { StockScoreData } from "@/lib/stock-score-api";
import type { StockSummaryData } from "@/lib/stock-summary-types";
import type { QuoteDetail, StockInfo } from "@/lib/quotes-types";
import { Card } from "@/components/ui/card";
import { NextEarningsCard, StockWhyLine } from "@/components/stock-workspace/stock-overview-parts";
import { StockRedditOverviewCard } from "@/components/community/stock-reddit-panel";
import type { StockSignals } from "@/lib/stock-signals-api";
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
  /** 해외 종목의 레딧 자료 — 워크스페이스가 「레딧」 탭과 함께 한 번 받는다. 없으면 카드를 안 그린다. */
  reddit?: StockSignals | null;
  onOpenReddit?: () => void;
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
  reddit = null,
  onOpenReddit,
}: StockOverviewTabProps) {
  const communityVisible = useCommunityVisible();
  /*
    국내 ETF는 점수 자리에 「ETF 점검」을 둔다 (2026-09-24 사용자 결정). 점검이 없으면
    (상장 20거래일 미만 등) 원래 점수 카드로 돌아간다 — 빈 자리를 남기지 않는다.
  */
  const [etfMissingFor, setEtfMissingFor] = useState<string | null>(null);
  const onEtfMissing = useCallback(() => setEtfMissingFor(stock.symbol), [stock.symbol]);
  const showEtfCheck = isDomesticEtf(stock) && etfMissingFor !== stock.symbol;

  /*
    AI 분석 탭과 **같은 카드**다 (2026-09-13 사용자 요청 2). 오른쪽 위 토글로
    「간단」(요약 한 문단)과 「상세」(이어진 본문)를 오간다 — 개요 기본값은 간단(10-13 요청 5).
    `hideWithoutLLM`은 여기서만 켠다: 이유는 카드 머리글에.
  */
  const aiCard = (
    <StockAISummaryCard
      data={aiSummary}
      loading={aiSummaryLoading}
      compact={compact}
      hideWithoutLLM
      defaultDetail="brief"
      // 종목명·코드를 되풀이하는 태그를 걸러내려면 무엇이 그 값인지 알아야 한다.
      symbol={stock.symbol}
      name={stock.name}
    />
  );
  const showScenario = scenarioLoading || Boolean(scenario && !scenario.unavailable);

  /*
    순서 (2026-10-13 요청 2·4·5·6 — 시안 `docs/mockups/stock-overview-v2.html`, 10-10 A안을 고침):
    최근 핵심 사건 → AI 요약(간단) → 점수 → 다음 실적 | 90일 범위 → 애널리스트 → 일봉·지표·커뮤니티 → 레딧(해외).
    관심 등록 안내·질문 칩은 뺐다.
  */
  return (
    <div className={cn("space-y-3 p-3", compact && "p-2")}>
      <StockWhyLine symbol={stock.symbol} />

      {aiCard}

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
      {communityVisible && (
        <StockCommunityCard symbol={stock.symbol} name={stock.name} posts={communityPosts} />
      )}

      {/* 레딧 분위기 — 해외 종목에 레딧 자료가 있을 때만, 맨 아래(10-13 요청 6). */}
      {reddit && <StockRedditOverviewCard signals={reddit} onOpen={onOpenReddit} />}

    </div>
  );
}
