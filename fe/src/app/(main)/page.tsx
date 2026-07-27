import { PageHeader } from "@/components/layout/header";
import { DashboardClient } from "@/components/dashboard/dashboard-client";
import { MarketOverview } from "@/components/market/market-overview";
import { PortfolioErrorBanner } from "@/components/dashboard/portfolio-status";
import { ACCOUNT_FEATURES_VISIBLE } from "@/lib/account-features";
import {
  fetchDashboardInsights,
  fetchPortfolioDiagnostics,
  fetchPortfolioSummary,
} from "@/lib/api-client";

export default async function HomePage() {
  let summary = null;
  let insights = null;
  let errorMessage: string | null = null;
  let diagnostics = null;

  if (ACCOUNT_FEATURES_VISIBLE) {
    try {
      summary = await fetchPortfolioSummary();
      insights = await fetchDashboardInsights();
    } catch (err) {
      errorMessage = err instanceof Error ? err.message : "알 수 없는 오류";
      try {
        diagnostics = await fetchPortfolioDiagnostics();
      } catch {
        // ignore secondary failure
      }
    }
  }

  return (
    <>
      <PageHeader
        title="홈"
        subtitle={
          ACCOUNT_FEATURES_VISIBLE
            ? "토스증권 계좌 실시간 연동"
            : "시장 현황 · 관심종목 · 뉴스"
        }
      />
      <main className="flex-1 overflow-x-auto overflow-y-auto p-6">
        <div className="mx-auto min-w-[var(--dashboard-content-min)] max-w-7xl space-y-6">
          <MarketOverview />

          {ACCOUNT_FEATURES_VISIBLE && errorMessage && (
            <PortfolioErrorBanner
              message={errorMessage}
              diagnostics={diagnostics}
              loginNext="/"
            />
          )}

          {ACCOUNT_FEATURES_VISIBLE && summary && insights && (
            <DashboardClient initialSummary={summary} initialInsights={insights} />
          )}
        </div>
      </main>
    </>
  );
}
