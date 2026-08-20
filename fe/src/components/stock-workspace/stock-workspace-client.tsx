"use client";

import { useEffect, useMemo, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { AIAnalysisTab } from "@/components/analysis/ai-analysis-tab";
import { EarningsTab } from "@/components/analysis/earnings-tab";
import { InsiderTab } from "@/components/analysis/insider-tab";
import {
  CompanyInfoTab,
  FinancialsChartsTab,
  ValuationTab,
} from "@/components/analysis/lazy-tabs";
import { useAuth } from "@/components/providers/auth-provider";
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
import {
  addWatchlistClient,
  fetchQuoteDetailClient,
  fetchStockClient,
  fetchWatchlistClient,
  removeWatchlistClient,
} from "@/lib/market-api";
import type { QuoteDetail, StockInfo } from "@/lib/quotes-types";
import {
  coerceAnalysisTab,
  parseStockWorkspaceTab,
  stockWorkspaceHref,
  stockWorkspaceTabsForMode,
  type StockWorkspaceMode,
  type StockWorkspaceTabId,
} from "@/lib/stock-workspace-tabs";
import { cn } from "@/lib/utils";

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
        className="text-[11px] font-medium text-[var(--brand)] hover:underline"
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
  loginNext?: string;
  /** full: chart + live quotes (마켓 레이더). analysis: fundamentals only (종목 분석). */
  mode?: StockWorkspaceMode;
  /** Optional tab bar override (e.g. 마켓 레이더 chart tab label). */
  tabs?: readonly { id: StockWorkspaceTabId; label: string }[];
  /** Called after watchlist toggle succeeds — e.g. sync market radar list star. */
  onWatchlistChange?: (symbol: string, watchlisted: boolean) => void;
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
  loginNext = "/market",
  mode = "full",
  tabs: tabsOverride,
  onWatchlistChange,
  railFrom = "lg",
}: StockWorkspaceClientProps) {
  const isAnalysis = mode === "analysis";
  const tabs = tabsOverride ?? stockWorkspaceTabsForMode(mode);
  const router = useRouter();
  const searchParams = useSearchParams();
  const { session } = useAuth();
  const exchangeRate = useExchangeRate(true);
  const { fundamentalsPollMs } = useMarketDataSettings();
  const { detail: quote, orderBook, trades, loading: quoteLoading } = useQuoteDetail(
    isAnalysis ? "" : symbol,
  );
  // Fetched here, not inside the header line: the header renders that line
  // twice (wide vs narrow layout) and each copy would otherwise fetch.
  const { data: aiSummary, loading: aiSummaryLoading } = useStockSummary(symbol);
  // Fetched once here: the AI 분석 tab shows the whole breakdown while several
  // other tabs show only their own axis.
  const { data: stockScore, loading: stockScoreLoading } = useStockScore(symbol);

  const parsedUrlTab = parseStockWorkspaceTab(searchParams.get("tab"));
  const urlTab = isAnalysis ? coerceAnalysisTab(parsedUrlTab) : parsedUrlTab;
  const [embeddedTab, setEmbeddedTab] = useState<StockWorkspaceTabId>("overview");
  const [stock, setStock] = useState<StockInfo | null>(null);
  const [quoteSnapshot, setQuoteSnapshot] = useState<QuoteDetail | null>(null);
  const [fundamentals, setFundamentals] = useState<FundamentalsData | null>(null);
  const [fundamentalsLoading, setFundamentalsLoading] = useState(false);
  const [stockLoading, setStockLoading] = useState(false);
  const [watchlisted, setWatchlisted] = useState(false);
  const [watchlistBusy, setWatchlistBusy] = useState(false);

  const rawTab = controlledTab ?? (embedded ? embeddedTab : urlTab);
  const effectiveTab = isAnalysis ? coerceAnalysisTab(rawTab) : rawTab;
  const displayQuote = isAnalysis ? quoteSnapshot : quote;

  useEffect(() => {
    if (!symbol) return;
    let cancelled = false;
    setStockLoading(true);
    Promise.all([
      fetchStockClient(symbol),
      isAnalysis ? fetchQuoteDetailClient(symbol).catch(() => null) : Promise.resolve(null),
    ])
      .then(([stockRow, detail]) => {
        if (cancelled) return;
        setStock(stockRow);
        if (isAnalysis && detail) setQuoteSnapshot(detail);
        else if (isAnalysis) setQuoteSnapshot(null);
      })
      .catch(() => {
        if (!cancelled) {
          setStock({
            symbol,
            name: symbol,
            market: "—",
            currency: "KRW",
          });
        }
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
    if (!session) {
      setWatchlisted(false);
      return;
    }
    fetchWatchlistClient()
      .then((list) => setWatchlisted(list.some((item) => item.symbol === symbol)))
      .catch(() => setWatchlisted(false));
  }, [symbol, session]);

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

  async function toggleWatchlist() {
    if (!session) {
      router.push(`/login?next=${encodeURIComponent(loginNext)}`);
      return;
    }
    if (watchlistBusy) return;

    const prev = watchlisted;
    const next = !watchlisted;
    setWatchlisted(next);
    onWatchlistChange?.(symbol, next);
    setWatchlistBusy(true);

    try {
      if (next) await addWatchlistClient(symbol);
      else await removeWatchlistClient(symbol);
    } catch {
      setWatchlisted(prev);
      onWatchlistChange?.(symbol, prev);
    } finally {
      setWatchlistBusy(false);
    }
  }

  const analysisLoading = stockLoading || fundamentalsLoading;
  // Every tab scrolls except 차트, whose terminal owns its own scroll regions.
  // Restricting this to 개요 silently clipped the taller tabs on desktop —
  // 내부자 거래 lost ~910px of 1521px with no scrollbar to reach it.
  const scrollMain = effectiveTab !== "chart";

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
        watchlisted={watchlisted}
        watchlistBusy={watchlistBusy}
        onToggleWatchlist={toggleWatchlist}
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
              onNavigate={setTab}
              compact={compact}
              analysisMode={isAnalysis}
              railFrom={railFrom}
              aiSummary={aiSummary}
              aiSummaryLoading={aiSummaryLoading}
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
                score={stockScore}
                scoreLoading={stockScoreLoading}
              />
            </div>
          )}

          {!isAnalysis && effectiveTab === "chart" && (
            <div className="flex min-h-0 flex-1 flex-col">
              {/* shrink-0 so the badge never steals height from the terminal,
                  which owns its own scrolling below. */}
              {stockScore?.breakdown?.technical != null && (
                <div className="shrink-0 px-3 pt-2">
                  <TabScore tab="chart" score={stockScore} onOpenAnalysis={() => setTab("ai")} />
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
              <TabScore tab="valuation" score={stockScore} onOpenAnalysis={() => setTab("ai")} />
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
              <TabScore tab="financials" score={stockScore} onOpenAnalysis={() => setTab("ai")} />
              <FinancialsChartsTab
                stock={stockRow}
                fundamentals={fundamentals}
                loading={analysisLoading}
              />
            </div>
          )}

          {effectiveTab === "earnings" && (
            <div className="space-y-2 p-3">
              <TabScore tab="earnings" score={stockScore} onOpenAnalysis={() => setTab("ai")} />
              <EarningsTab symbol={symbol} currency={stockRow.currency} fundamentals={fundamentals} />
            </div>
          )}

          {effectiveTab === "insider" && (
            <div className="space-y-2 p-3">
              <TabScore tab="insider" score={stockScore} onOpenAnalysis={() => setTab("ai")} />
              <InsiderTab symbol={symbol} currency={stockRow.currency} />
            </div>
          )}

          {effectiveTab === "news" && (
            <div className="space-y-2 p-3">
              <TabScore tab="news" score={stockScore} onOpenAnalysis={() => setTab("ai")} />
              <StockNewsPanel
                symbol={symbol}
                name={stockRow.name}
                market={stockNewsMarket(stockRow)}
              />
            </div>
          )}

          {!isAnalysis && quoteLoading && !displayQuote && effectiveTab !== "chart" && (
            <p className="p-4 text-center text-xs text-[var(--text-muted)]">시세 로딩 중…</p>
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
