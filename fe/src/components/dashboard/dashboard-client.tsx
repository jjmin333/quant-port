"use client";

import { AssetSummary } from "@/components/dashboard/asset-summary";
import { BrokerAccounts } from "@/components/dashboard/broker-accounts";
import { AssetAllocationChart } from "@/components/dashboard/asset-allocation";
import { MorningBriefing } from "@/components/dashboard/morning-briefing";
import { HoldingsPreview } from "@/components/dashboard/holdings-preview";
import { AssetHistoryChart } from "@/components/dashboard/asset-history";
import { RiskAlerts } from "@/components/dashboard/risk-alerts";
import { PortfolioRefreshStatus } from "@/components/dashboard/portfolio-status";
import { usePortfolioPolling } from "@/hooks/use-portfolio-polling";
import { useInsightsPolling } from "@/hooks/use-insights-polling";
import type { PortfolioSummary } from "@/lib/portfolio-types";
import type { DashboardInsights } from "@/lib/insights-types";

export function DashboardClient({
  initialSummary,
  initialInsights,
}: {
  initialSummary: PortfolioSummary;
  initialInsights: DashboardInsights;
}) {
  const { summary, isRefreshing, isLiveRefreshing, lastError } = usePortfolioPolling(initialSummary);
  const insights = useInsightsPolling(initialInsights);

  return (
    <>
      <PortfolioRefreshStatus
        summary={summary}
        isRefreshing={isRefreshing}
        isLiveRefreshing={isLiveRefreshing}
        pollError={lastError}
      />

      <AssetSummary summary={summary} />

      <div className="grid min-w-[var(--dashboard-content-min)] shrink-0 gap-6 lg:grid-cols-3">
        <div className="min-w-[var(--dashboard-content-min)] shrink-0 lg:col-span-2">
          <BrokerAccounts accounts={summary.accounts} />
        </div>
        <div className="min-w-[var(--dashboard-content-min)] shrink-0">
          <AssetAllocationChart summary={summary} />
        </div>
      </div>

      <HoldingsPreview
        holdings={summary.holdings}
        exchangeRate={summary.exchangeRate}
        avgPurchaseFxRate={summary.avgPurchaseFxRate}
      />

      <div className="grid min-w-[var(--dashboard-content-min)] shrink-0 gap-6 lg:grid-cols-3">
        <div className="min-w-[var(--dashboard-content-min)] shrink-0 lg:col-span-2">
          <AssetHistoryChart currentTotal={summary.totalAssetsKRW} />
        </div>
        <div className="min-w-[var(--dashboard-content-min)] shrink-0 space-y-6">
          <MorningBriefing briefing={insights.briefing} />
          <RiskAlerts alerts={insights.risks} />
        </div>
      </div>
    </>
  );
}
