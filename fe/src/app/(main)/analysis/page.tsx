import { Suspense } from "react";
import { PortfolioErrorBanner } from "@/components/dashboard/portfolio-status";
import { AnalysisClient } from "@/components/analysis/analysis-client";
import {
  fetchDashboardInsights,
  fetchPortfolioDiagnostics,
  fetchPortfolioSummary,
} from "@/lib/api-client";
import { fetchDisclosuresTodayClient, fetchStocksBatchClient } from "@/lib/market-api";
import { sortHoldingsByValue } from "@/lib/asset-allocation";
import { uniqueStocksBySymbol } from "@/lib/stock-utils";
import type { StockInfo } from "@/lib/quotes-types";

export default async function AnalysisPage() {
  let summary = null;
  let insights = null;
  let disclosures: Awaited<ReturnType<typeof fetchDisclosuresTodayClient>> = [];
  let errorMessage: string | null = null;
  let diagnostics = null;

  try {
    [summary, insights] = await Promise.all([
      fetchPortfolioSummary(),
      fetchDashboardInsights(),
    ]);
  } catch (err) {
    errorMessage = err instanceof Error ? err.message : "알 수 없는 오류";
    try {
      diagnostics = await fetchPortfolioDiagnostics();
    } catch {
      // ignore
    }
  }

  try {
    disclosures = await fetchDisclosuresTodayClient();
  } catch {
    // optional
  }

  const holdings = summary ? uniqueStocksBySymbol(sortHoldingsByValue(summary.holdings)) : [];
  let stocks: (StockInfo & { fetchError?: string })[] = [];

  if (holdings.length > 0) {
    try {
      const bySymbol = new Map(
        (await fetchStocksBatchClient(holdings.map((h) => h.symbol))).map((s) => [s.symbol, s]),
      );
      stocks = holdings.map((holding) => {
        const stock = bySymbol.get(holding.symbol);
        if (stock) return stock;
        return {
          symbol: holding.symbol,
          name: holding.name,
          market: "—",
          currency: holding.currency,
          fetchError: "종목 정보 없음",
        };
      });
    } catch (err) {
      stocks = holdings.map((holding) => ({
        symbol: holding.symbol,
        name: holding.name,
        market: "—",
        currency: holding.currency,
        fetchError: err instanceof Error ? err.message : "조회 실패",
      }));
    }
  }

  return (
    <>
      {errorMessage && (
        <div className="px-6 pt-6">
          <div className="mx-auto max-w-6xl">
            <PortfolioErrorBanner message={errorMessage} diagnostics={diagnostics} />
          </div>
        </div>
      )}
      <Suspense
        fallback={
          <main className="flex flex-1 items-center justify-center p-8 text-sm text-[var(--text-muted)]">
            분석 화면을 불러오는 중…
          </main>
        }
      >
        <AnalysisClient
          holdings={stocks}
          initialInsights={insights}
          initialDisclosures={disclosures}
        />
      </Suspense>
    </>
  );
}
