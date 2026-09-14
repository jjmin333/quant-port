"use client";

import { useEffect, useMemo, useState } from "react";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { SearchX } from "lucide-react";
import { AIAnalysisTab } from "@/components/analysis/ai-analysis-tab";
import { EarningsTab } from "@/components/analysis/earnings-tab";
import { InsiderTab } from "@/components/analysis/insider-tab";
import {
  CompanyInfoTab,
  FinancialsChartsTab,
  ValuationTab,
} from "@/components/analysis/lazy-tabs";
import { StockChartTerminal } from "@/components/stock-workspace/stock-chart-terminal";
import {
  StockMetricsColumn,
  type MetricsRailBreakpoint,
} from "@/components/stock-workspace/stock-metrics-column";
import { StockOverviewTab } from "@/components/stock-workspace/stock-overview-tab";
import { StockWorkspaceHeader } from "@/components/stock-workspace/stock-workspace-header";
import { StockWorkspaceTabBar } from "@/components/stock-workspace/stock-workspace-tab-bar";
import { StockNewsPanel } from "@/components/stocks/stock-news-panel";
import { useExchangeRate } from "@/hooks/use-exchange-rate";
import { useMarketDataSettings } from "@/hooks/use-market-data-settings";
import { AxisScoreBadge } from "@/components/analysis/stock-score-card";
import { TAB_SCORE_AXIS, type StockScoreData } from "@/lib/stock-score-api";
import { useStockScore } from "@/hooks/use-stock-score";
import { useStockSummary } from "@/hooks/use-stock-summary";
import { useQuoteDetail } from "@/hooks/use-quote-detail";
import { fetchFundamentalsClient } from "@/lib/fundamentals-api";
import type { FundamentalsData } from "@/lib/fundamentals-types";
import { fetchQuoteDetailClient, fetchStockClient } from "@/lib/market-api";
import type { QuoteDetail, StockInfo } from "@/lib/quotes-types";
import {
  coerceAnalysisTab,
  parseStockWorkspaceTab,
  stockWorkspaceHref,
  stockWorkspaceTabsForMode,
  type StockWorkspaceMode,
  type StockWorkspaceTabId,
} from "@/lib/stock-workspace-tabs";
import { PortfolioFetchError } from "@/lib/api-client";
import { rememberStock } from "@/lib/recent-stocks";
import { cn } from "@/lib/utils";
import { buttonVariants } from "@/components/ui/button";

function stockNewsMarket(stock: { assetType?: string; currency?: string }): "domestic" | "us" {
  if (stock.assetType === "us" || stock.currency === "USD") return "us";
  return "domestic";
}

/**
 * Each tab shows the one score axis it is actually about — 차트은 추세, 밸류에이션은
 * 가치 — rather than repeating the composite everywhere. Renders nothing when
 * that axis has no data, so a tab never shows an empty score chip.
 */
function TabScore({
  tab,
  score,
  onOpenAnalysis,
}: {
  tab: string;
  score: StockScoreData | null;
  onOpenAnalysis: () => void;
}) {
  const axis = TAB_SCORE_AXIS[tab];
  if (!axis || score?.breakdown?.[axis] == null) return null;
  return (
    <div className="flex flex-wrap items-center gap-2">
      <AxisScoreBadge score={score} axis={axis} />
      <button
        type="button"
        onClick={onOpenAnalysis}
        className="text-cell font-medium text-[var(--brand)] hover:underline"
      >
        점수 상세 →
      </button>
    </div>
  );
}

type StockWorkspaceClientProps = {
  symbol: string;
  /** Controlled tab (embedded /quotes). Omit to use URL routing on /stocks/[symbol]. */
  activeTab?: StockWorkspaceTabId;
  onTabChange?: (tab: StockWorkspaceTabId) => void;
  embedded?: boolean;
  compact?: boolean;
  onBack?: () => void;
  className?: string;
  /** full: chart + live quotes (마켓 레이더). analysis: fundamentals only (종목 분석). */
  mode?: StockWorkspaceMode;
  /** Optional tab bar override (e.g. 마켓 레이더 chart tab label). */
  tabs?: readonly { id: StockWorkspaceTabId; label: string }[];
  /**
   * Width at which the 240px metrics rail appears. Pass "xl" when the page
   * already spends a column on something else (마켓 레이더's ranking list),
   * otherwise the rail starves the tab content at laptop widths.
   */
  railFrom?: MetricsRailBreakpoint;
};

export function StockWorkspaceClient({
  symbol,
  activeTab: controlledTab,
  onTabChange,
  embedded = false,
  compact = false,
  onBack,
  className,
  mode = "full",
  tabs: tabsOverride,
  railFrom = "lg",
}: StockWorkspaceClientProps) {
  const isAnalysis = mode === "analysis";
  const tabs = tabsOverride ?? stockWorkspaceTabsForMode(mode);
  const router = useRouter();
  const searchParams = useSearchParams();
  const exchangeRate = useExchangeRate(true);
  const { fundamentalsPollMs } = useMarketDataSettings();
  // Fetched here, not inside the header line: the header renders that line
  // twice (wide vs narrow layout) and each copy would otherwise fetch.
  const { data: aiSummary, loading: aiSummaryLoading } = useStockSummary(symbol);
  // Fetched once here: 개요 탭이 전체 분해를 그리고, 나머지 탭은 자기 축 하나만
  // 배지로 보여 준다.
  const { data: stockScore, loading: stockScoreLoading } = useStockScore(symbol);

  const parsedUrlTab = parseStockWorkspaceTab(searchParams.get("tab"));
  const urlTab = isAnalysis ? coerceAnalysisTab(parsedUrlTab) : parsedUrlTab;
  const [embeddedTab, setEmbeddedTab] = useState<StockWorkspaceTabId>("overview");
  const [stock, setStock] = useState<StockInfo | null>(null);
  const [quoteSnapshot, setQuoteSnapshot] = useState<QuoteDetail | null>(null);
  const [fundamentals, setFundamentals] = useState<FundamentalsData | null>(null);
  const [fundamentalsLoading, setFundamentalsLoading] = useState(false);
  const [stockLoading, setStockLoading] = useState(false);
  /** 카탈로그에 없는 심볼(404). 로딩과 구분해야 화면이 멈추지 않는다. */
  const [stockMissing, setStockMissing] = useState(false);
  /*
    관심 등록·해제 버튼은 상세 헤더에서 뺐다(2026-09-03 사용자 결정) — 하트는
    랭킹 목록의 행마다 이미 있다. 그 상태는 이제 공유 저장소(`lib/watchlist-store`)가
    한 벌로 들고 있어, 어느 화면에서 누르든 나머지 화면이 같은 프레임에 따라온다.
  */

  const rawTab = controlledTab ?? (embedded ? embeddedTab : urlTab);
  const effectiveTab = isAnalysis ? coerceAnalysisTab(rawTab) : rawTab;
  /*
    `wantsOrderBook`은 **호가창을 실제로 그리는 탭일 때만** true다.

    호가창은 아래 `effectiveTab === "chart"` 분기에서만 렌더된다. 그런데 예전에는
    상세 화면에 들어가기만 하면 서버가 그 종목의 웹소켓 호가까지 구독했다 —
    개요·AI·재무 탭을 보는 동안에도 호가 프레임이 계속 들어왔다는 뜻이다
    (실측: 호가 프레임이 체결의 4.3배). 이 플래그가 그 구독을 실제 시청과 묶는다.
  */
  const { detail: quote, orderBook, trades, loading: quoteLoading } = useQuoteDetail(
    isAnalysis ? "" : symbol,
    { wantsOrderBook: !isAnalysis && effectiveTab === "chart" },
  );
  const displayQuote = isAnalysis ? quoteSnapshot : quote;

  useEffect(() => {
    if (!symbol) return;
    let cancelled = false;
    setStockLoading(true);
    setStockMissing(false);
    Promise.all([
      fetchStockClient(symbol),
      isAnalysis ? fetchQuoteDetailClient(symbol).catch(() => null) : Promise.resolve(null),
    ])
      .then(([stockRow, detail]) => {
        if (cancelled) return;
        setStock(stockRow);
        // 검색이 아니라 링크·랭킹으로 들어온 것도 「최근 본 종목」이다.
        if (stockRow?.symbol) rememberStock(stockRow);
        if (isAnalysis && detail) setQuoteSnapshot(detail);
        else if (isAnalysis) setQuoteSnapshot(null);
      })
      .catch((err) => {
        if (cancelled) return;
        /*
          **없는 종목과 일시적 실패를 가른다.**

          예전에는 어떤 실패든 `{symbol, name: symbol, market: "—"}` 라는 가짜 종목을
          만들어 워크스페이스를 그대로 그렸다. 그러면 시세·요약·차트가 전부 오지
          않으므로 화면은 「시세 로딩 중…」에서 멈춘 채 끝나지 않았다(실측:
          `/stocks/ZZZZZZ` 를 22초 넘게 관찰해도 그대로).

          404는 「이 심볼은 존재하지 않는다」는 확정된 답이므로 로딩이 아니라
          「찾을 수 없음」으로 말해야 한다. 그 밖의 실패(500·네트워크)는 종목이
          있을 수도 있으니 기존 폴백을 유지한다.
        */
        if (err instanceof PortfolioFetchError && err.status === 404) {
          setStock(null);
          setStockMissing(true);
          return;
        }
        setStock({
          symbol,
          name: symbol,
          market: "—",
          currency: "KRW",
        });
      })
      .finally(() => {
        if (!cancelled) setStockLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [symbol, isAnalysis]);

  useEffect(() => {
    if (!symbol) {
      setFundamentals(null);
      return;
    }
    let cancelled = false;

    const loadFundamentals = () => {
      setFundamentalsLoading(true);
      fetchFundamentalsClient(symbol)
        .then((data) => {
          if (!cancelled) setFundamentals(data);
        })
        .catch(() => {
          if (!cancelled) setFundamentals(null);
        })
        .finally(() => {
          if (!cancelled) setFundamentalsLoading(false);
        });
    };

    loadFundamentals();
    const id = window.setInterval(loadFundamentals, fundamentalsPollMs);
    return () => {
      cancelled = true;
      window.clearInterval(id);
    };
  }, [symbol, fundamentalsPollMs]);


  useEffect(() => {
    if (embedded && controlledTab == null) {
      setEmbeddedTab("overview");
    }
  }, [symbol, embedded, controlledTab]);

  const stockRow = useMemo(
    () =>
      stock ?? {
        symbol,
        name: symbol,
        market: "—",
        currency: "KRW" as const,
      },
    [stock, symbol],
  );

  function setTab(tab: StockWorkspaceTabId) {
    const next = isAnalysis ? coerceAnalysisTab(tab) : tab;
    if (onTabChange) {
      onTabChange(next);
      return;
    }
    if (embedded) {
      setEmbeddedTab(next);
      return;
    }
    router.replace(stockWorkspaceHref(symbol, next), { scroll: false });
  }

  const analysisLoading = stockLoading || fundamentalsLoading;
  // Every tab scrolls except 차트, whose terminal owns its own scroll regions.
  // Restricting this to 개요 silently clipped the taller tabs on desktop —
  // 내부자 거래 lost ~910px of 1521px with no scrollbar to reach it.
  const scrollMain = effectiveTab !== "chart";

  /*
    존재하지 않는 심볼. 탭·차트·시세를 그려 봐야 전부 빈 값이므로 워크스페이스 자체를
    그리지 않고 「찾을 수 없음」과 출구만 준다.
  */
  if (stockMissing) {
    return (
      <div
        className={cn(
          "flex min-h-0 min-w-0 flex-1 flex-col items-center justify-center gap-4 bg-[var(--background)] px-6 py-16 text-center",
          className,
        )}
      >
        <div className="grid size-11 place-items-center rounded-full bg-[var(--surface-elevated)] text-[var(--text-secondary)]">
          <SearchX className="h-5 w-5" />
        </div>
        <div className="space-y-1.5">
          <h2 className="text-base font-semibold text-[var(--text-primary)]">
            종목을 찾을 수 없습니다
          </h2>
          <p className="max-w-[420px] text-cell leading-relaxed text-[var(--text-muted)]">
            <span className="font-num font-semibold text-[var(--text-secondary)]">{symbol}</span>{" "}
            는 등록된 종목이 아닙니다. 종목코드나 티커를 다시 확인해 주세요.
          </p>
        </div>
        <div className="flex flex-wrap items-center justify-center gap-2">
          <Link
            href="/analysis"
            className={cn(buttonVariants({ variant: "default", size: "sm" }), "bg-[var(--brand)] text-white hover:opacity-90")}
          >
            종목 찾아보기
          </Link>
          {onBack && (
            <button
              type="button"
              onClick={onBack}
              className={cn(buttonVariants({ variant: "outline", size: "sm" }), "border-[var(--border)] text-[var(--text-secondary)] hover:bg-[var(--surface-elevated)]")}
            >
              뒤로
            </button>
          )}
        </div>
      </div>
    );
  }

  return (
    <div
      className={cn(
        "flex min-h-0 min-w-0 flex-col overflow-hidden bg-[var(--background)]",
        className,
      )}
    >
      <StockWorkspaceHeader
        stock={stockRow}
        quote={displayQuote}
        exchangeRate={exchangeRate}
        loading={stockLoading && !stock}
        onBack={onBack}
        compact={compact}
        showLiveQuote={!isAnalysis}
      />

      <StockWorkspaceTabBar active={effectiveTab} onChange={setTab} tabs={tabs} />

      <div className="flex min-h-0 flex-1 overflow-hidden">
        {effectiveTab === "overview" && (
          <StockMetricsColumn
            quote={displayQuote}
            fundamentals={fundamentals}
            variant={isAnalysis ? "fundamentals" : "full"}
            railFrom={railFrom}
          />
        )}

        <div
          className={cn(
            "flex min-h-0 min-w-0 flex-1 flex-col",
            scrollMain ? "overflow-y-auto overflow-x-hidden" : "overflow-hidden",
          )}
        >
          {effectiveTab === "overview" && (
            <StockOverviewTab
              stock={stockRow}
              quote={displayQuote}
              fundamentals={fundamentals}
              compact={compact}
              analysisMode={isAnalysis}
              railFrom={railFrom}
              aiSummary={aiSummary}
              aiSummaryLoading={aiSummaryLoading}
              score={stockScore}
              scoreLoading={stockScoreLoading}
            />
          )}

          {effectiveTab === "ai" && (
            <div className="p-3">
              <AIAnalysisTab
                symbol={symbol}
                currency={stockRow.currency}
                data={aiSummary}
                loading={aiSummaryLoading}
                compact={compact}
              />
            </div>
          )}

          {!isAnalysis && effectiveTab === "chart" && (
            <div className="flex min-h-0 flex-1 flex-col">
              {/* shrink-0 so the badge never steals height from the terminal,
                  which owns its own scrolling below. */}
              {stockScore?.breakdown?.technical != null && (
                <div className="shrink-0 px-3 pt-2">
                  <TabScore tab="chart" score={stockScore} onOpenAnalysis={() => setTab("overview")} />
                </div>
              )}
              <StockChartTerminal
                symbol={symbol}
                detail={displayQuote}
                orderBook={orderBook}
                trades={trades}
                loading={quoteLoading && !displayQuote}
                compact={compact}
                className="min-h-0 flex-1"
              />
            </div>
          )}

          {/* The per-tab AI 요약 banners are gone: the headline now sits atop
              the 개요 tab and the full text in the AI 분석 tab, so repeating a
              section of it above every tab only ate vertical space. */}
          {effectiveTab === "company" && (
            <div className="p-3">
              <CompanyInfoTab stock={stockRow} quote={quote} fundamentals={fundamentals} />
            </div>
          )}

          {effectiveTab === "valuation" && (
            <div className="space-y-2 p-3">
              <TabScore tab="valuation" score={stockScore} onOpenAnalysis={() => setTab("overview")} />
              <ValuationTab
                stock={stockRow}
                fundamentals={fundamentals}
                quote={displayQuote}
                loading={analysisLoading}
              />
            </div>
          )}

          {effectiveTab === "financials" && (
            <div className="space-y-2 p-3">
              <TabScore tab="financials" score={stockScore} onOpenAnalysis={() => setTab("overview")} />
              <FinancialsChartsTab
                stock={stockRow}
                fundamentals={fundamentals}
                loading={analysisLoading}
              />
            </div>
          )}

          {effectiveTab === "earnings" && (
            <div className="space-y-2 p-3">
              <TabScore tab="earnings" score={stockScore} onOpenAnalysis={() => setTab("overview")} />
              <EarningsTab symbol={symbol} currency={stockRow.currency} fundamentals={fundamentals} />
            </div>
          )}

          {effectiveTab === "insider" && (
            <div className="space-y-2 p-3">
              <TabScore tab="insider" score={stockScore} onOpenAnalysis={() => setTab("overview")} />
              <InsiderTab symbol={symbol} currency={stockRow.currency} />
            </div>
          )}

          {effectiveTab === "news" && (
            <div className="space-y-2 p-3">
              <TabScore tab="news" score={stockScore} onOpenAnalysis={() => setTab("overview")} />
              <StockNewsPanel
                symbol={symbol}
                name={stockRow.name}
                market={stockNewsMarket(stockRow)}
              />
            </div>
          )}

          {!isAnalysis && quoteLoading && !displayQuote && effectiveTab !== "chart" && (
            <p className="p-4 text-center text-cell text-[var(--text-muted)]">시세 로딩 중…</p>
          )}
        </div>
      </div>
    </div>
  );
}

/** Parse tab from URL search params for /stocks/[symbol] page */
export function useStockWorkspaceTabFromUrl(tabParam: string | null): StockWorkspaceTabId {
  return parseStockWorkspaceTab(tabParam);
}
