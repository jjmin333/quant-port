"use client";

import { useEffect, useMemo, useState } from "react";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { AnalysisTabBar } from "@/components/analysis/analysis-tab-bar";
import { ScoreRankingPanel } from "@/components/analysis/score-ranking-panel";
import { StockSideRail } from "@/components/analysis/stock-side-rail";
import { PageColumns } from "@/components/layout/page-columns";
import { AIAnalysisTab } from "@/components/analysis/ai-analysis-tab";
import { EarningsTab } from "@/components/analysis/earnings-tab";
import { InsiderTab } from "@/components/analysis/insider-tab";
import {
  CompanyInfoTab,
  FinancialsChartsTab,
  ValuationTab,
} from "@/components/analysis/lazy-tabs";
import { QuoteDailyChartModal } from "@/components/quotes/quote-daily-chart-modal";
import { PageScrollShell } from "@/components/layout/page-content-shell";
import { useHeaderSearch } from "@/components/providers/header-search-provider";
import { useShellMode } from "@/components/providers/shell-mode-provider";
import { useStockScore } from "@/hooks/use-stock-score";
import { useStockSummary } from "@/hooks/use-stock-summary";
import { fetchFundamentalsClient } from "@/lib/fundamentals-api";
import type { FundamentalsData } from "@/lib/fundamentals-types";
import { analysisSymbolHref, parseAnalysisTab, type AnalysisTabId } from "@/lib/analysis-tabs";
import { useRankingView } from "@/lib/ranking-view";
import {
  inferMarketCountryFromStock,
  marketRadarSymbolHref,
} from "@/lib/market-radar-config";
import { fetchQuoteDetailClient, fetchStockClient } from "@/lib/market-api";
import type { QuoteDetail, StockInfo } from "@/lib/quotes-types";
import { StockNewsPanel } from "@/components/stocks/stock-news-panel";
import { StockSymbolActionBar } from "@/components/stocks/stock-symbol-action-bar";
import { useWatchlistToggle } from "@/hooks/use-watchlist-toggle";

type StockRow = StockInfo & { fetchError?: string };

function stockNewsMarket(stock: { assetType?: string; currency?: string }): "domestic" | "us" {
  if (stock.assetType === "us" || stock.currency === "USD") return "us";
  return "domestic";
}

type AnalysisClientProps = {
  defaultSymbol: string | null;
};

export function AnalysisClient({ defaultSymbol }: AnalysisClientProps) {
  const router = useRouter();
  const searchParams = useSearchParams();
  const { isCompact } = useShellMode();
  const { registerStockPickHandler } = useHeaderSearch();
  const symbolParam = searchParams.get("symbol");
  const selectedSymbol = symbolParam ?? defaultSymbol;
  const activeTab = parseAnalysisTab(searchParams.get("tab"));
  const rankingView = useRankingView();

  const [selectedStock, setSelectedStock] = useState<StockRow | null>(null);
  const [quote, setQuote] = useState<QuoteDetail | null>(null);
  const [fundamentals, setFundamentals] = useState<FundamentalsData | null>(null);
  const [fundamentalsLoading, setFundamentalsLoading] = useState(false);
  const [stockLoading, setStockLoading] = useState(false);
  const [quoteModalOpen, setQuoteModalOpen] = useState(false);

  const loading = stockLoading || fundamentalsLoading;
  const { data: aiSummary, loading: aiSummaryLoading, error: aiSummaryError } = useStockSummary(selectedSymbol ?? "");
  const { data: stockScore, loading: stockScoreLoading } = useStockScore(selectedSymbol ?? "");
  const {
    watchlisted,
    busy: watchlistBusy,
    toggleWatchlist,
  } = useWatchlistToggle(selectedSymbol ?? "", {
    loginNext: selectedSymbol ? analysisSymbolHref(selectedSymbol, activeTab) : "/analysis",
  });

  useEffect(() => {
    registerStockPickHandler((symbol) => {
      router.replace(analysisSymbolHref(symbol, "ai"), { scroll: false });
    });
    return () => registerStockPickHandler(null);
  }, [registerStockPickHandler, router]);

  useEffect(() => {
    if (!selectedSymbol) {
      setSelectedStock(null);
      setQuote(null);
      return;
    }

    let cancelled = false;
    setStockLoading(true);
    Promise.all([
      fetchStockClient(selectedSymbol),
      fetchQuoteDetailClient(selectedSymbol).catch(() => null),
    ])
      .then(([stock, detail]) => {
        if (!cancelled) {
          setSelectedStock(stock);
          setQuote(detail);
        }
      })
      .catch(() => {
        if (!cancelled) {
          setSelectedStock({
            symbol: selectedSymbol,
            name: selectedSymbol,
            market: "—",
            currency: "USD",
          });
          setQuote(null);
        }
      })
      .finally(() => {
        if (!cancelled) setStockLoading(false);
      });

    return () => {
      cancelled = true;
    };
  }, [selectedSymbol]);

  useEffect(() => {
    if (!selectedSymbol) {
      setFundamentals(null);
      return;
    }

    let cancelled = false;
    setFundamentalsLoading(true);
    fetchFundamentalsClient(selectedSymbol)
      .then((data) => {
        if (!cancelled) setFundamentals(data);
      })
      .catch(() => {
        if (!cancelled) setFundamentals(null);
      })
      .finally(() => {
        if (!cancelled) setFundamentalsLoading(false);
      });

    return () => {
      cancelled = true;
    };
  }, [selectedSymbol]);

  const stock = useMemo(
    () =>
      selectedStock ?? {
        symbol: selectedSymbol ?? "",
        name: selectedSymbol ?? "",
        market: "—",
        currency: "KRW" as const,
      },
    [selectedStock, selectedSymbol],
  );

  function setTab(tab: AnalysisTabId) {
    if (!selectedSymbol) return;
    router.replace(analysisSymbolHref(selectedSymbol, tab), { scroll: false });
  }

  const liveQuotesHref = selectedSymbol
    ? marketRadarSymbolHref(selectedSymbol, inferMarketCountryFromStock(stock))
    : "/market";

  /*
   * 3단 조립 (docs/UI_SPEC.md §7 · 08-11 전체 폭 전환의 후속).
   *
   * 예전엔 랭킹 패널이 화면 폭을 통째로 가로지르는 배너였고, 그 아래에 탭 본문이
   * 또 한 번 폭을 다 썼다. 전체 폭으로 바뀌면서 20종목짜리 목록 한 줄이 1,500px을
   * 건너가 훑기 어려워졌고, 본문은 본문대로 늘어나기만 했다.
   *
   * 랭킹은 "여기서 다음 종목을 고른다"는 뜻이므로 왼쪽 레일이 제자리다 — 목록에서
   * 고르면 오른쪽 본문이 바뀐다는 관계가 위·아래보다 좌·우일 때 분명하다.
   * 오른쪽 레일은 탭과 무관한 상시 맥락(시세·점수·뉴스·리포트)을 맡는다.
   */
  return (
    <PageScrollShell>
      <PageColumns
        // 랭킹 `넓게` 보기는 한 줄에 이름·시세·스파크라인·등락·점수를 다 놓아서
        // 300px 레일에서는 종목명이 한 글자로 잘린다. 그때만 본문에서 폭을 빌린다.
        wideLead={rankingView === "wide"}
        leadRail={<ScoreRankingPanel activeSymbol={selectedSymbol} className="xl:flex-1" />}
        rail={
          selectedSymbol ? (
            <StockSideRail
              symbol={stock.symbol}
              name={stock.name}
              currency={stock.currency}
              market={stockNewsMarket(stock)}
              quote={quote}
              score={stockScore}
              scoreLoading={stockScoreLoading}
            />
          ) : undefined
        }
      >
        {!selectedSymbol ? (
          <div className="py-12 text-center">
            <h1 className="text-xl font-bold tracking-tight text-[var(--text-primary)]">
              종목 분석
            </h1>
            <p className="mx-auto mt-2 max-w-md text-[13px] leading-relaxed text-[var(--text-muted)]">
              상단 검색창에 종목명 또는 심볼을 입력하거나, 왼쪽 랭킹에서 종목을 골라 주세요.
            </p>
            <Link
              href="/market"
              className="mt-4 inline-block text-xs text-[var(--brand)] hover:underline"
            >
              마켓 레이더로 이동 →
            </Link>
          </div>
        ) : (
          <>
            <div className="mb-3 flex items-start justify-between gap-3">
              <div className="min-w-0">
                <h1 className="truncate text-xl font-bold tracking-tight text-[var(--text-primary)]">
                  {stockLoading ? "…" : stock.name}
                </h1>
                <p className="mt-0.5 text-[12px] text-[var(--text-muted)]">
                  {stock.symbol} · {stock.market} · {stock.currency}
                </p>
              </div>
              <div className="flex shrink-0 flex-col items-end gap-1.5 sm:flex-row sm:items-center">
                <button
                  type="button"
                  onClick={() => setQuoteModalOpen(true)}
                  className="rounded-lg border border-[var(--border)] px-2.5 py-1 text-[11px] text-[var(--text-secondary)] hover:bg-[var(--surface-elevated)]"
                >
                  실시간 시세
                </button>
                <StockSymbolActionBar
                  symbol={stock.symbol}
                  watchlisted={watchlisted}
                  watchlistBusy={watchlistBusy}
                  onToggleWatchlist={toggleWatchlist}
                  price={quote?.currentPrice}
                  currency={quote?.currency}
                  week52High={quote?.week52High}
                  week52Low={quote?.week52Low}
                />
              </div>
            </div>

            <AnalysisTabBar active={activeTab} onChange={setTab} />

            <div className="mt-3">
              {activeTab === "ai" && (
                <AIAnalysisTab
                  symbol={stock.symbol}
                  currency={stock.currency}
                  data={aiSummary}
                  loading={aiSummaryLoading}
                  error={aiSummaryError}
                  compact={isCompact}
                  score={stockScore}
                  scoreLoading={stockScoreLoading}
                />
              )}
              {activeTab === "company" && (
                <CompanyInfoTab stock={stock} quote={quote} fundamentals={fundamentals} />
              )}
              {activeTab === "valuation" && (
                <ValuationTab
                  stock={stock}
                  fundamentals={fundamentals}
                  quote={quote}
                  loading={loading}
                />
              )}
              {activeTab === "financials" && (
                <FinancialsChartsTab stock={stock} fundamentals={fundamentals} loading={loading} />
              )}
              {activeTab === "earnings" && (
                <EarningsTab
                  symbol={stock.symbol}
                  currency={stock.currency}
                  fundamentals={fundamentals}
                />
              )}
              {activeTab === "insider" && (
                <InsiderTab symbol={stock.symbol} currency={stock.currency} />
              )}
              {activeTab === "news" && (
                <StockNewsPanel
                  symbol={stock.symbol}
                  name={stock.name}
                  market={stockNewsMarket(stock)}
                />
              )}
            </div>
          </>
        )}
      </PageColumns>

      <QuoteDailyChartModal
        open={quoteModalOpen}
        onClose={() => setQuoteModalOpen(false)}
        stock={selectedSymbol ? stock : null}
        initialQuote={quote}
        compact={isCompact}
        fullQuotesHref={liveQuotesHref}
      />
    </PageScrollShell>
  );
}
