import { PageHeader } from "@/components/layout/header";
import { RiskAlerts } from "@/components/dashboard/risk-alerts";
import { PortfolioErrorBanner } from "@/components/dashboard/portfolio-status";
import { Card, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { fetchDashboardInsights, fetchPortfolioDiagnostics, fetchPortfolioSummary } from "@/lib/api-client";
import { calculateRiskSummary } from "@/lib/portfolio-allocation";
import { formatPercent } from "@/lib/format";
import { trendTextClass } from "@/lib/trend-colors";
import { cn } from "@/lib/utils";

export default async function RiskPage() {
  let insights = null;
  let summary = null;
  let errorMessage: string | null = null;
  let diagnostics = null;

  try {
    [insights, summary] = await Promise.all([
      fetchDashboardInsights(),
      fetchPortfolioSummary(),
    ]);
  } catch (err) {
    errorMessage = err instanceof Error ? err.message : "알 수 없는 오류";
    try {
      diagnostics = await fetchPortfolioDiagnostics();
    } catch {
      // ignore
    }
  }

  const riskSummary = summary ? calculateRiskSummary(summary) : null;

  return (
    <>
      <PageHeader title="리스크 분석" subtitle="포트폴리오 리스크 지표 및 알림" />
      <main className="flex-1 overflow-y-auto p-6">
        <div className="mx-auto min-w-[var(--dashboard-content-min)] max-w-5xl space-y-6">
          {errorMessage && (
            <PortfolioErrorBanner message={errorMessage} diagnostics={diagnostics} />
          )}

          {riskSummary && (
            <Card>
              <CardHeader>
                <CardTitle>포트폴리오 리스크 요약</CardTitle>
                <Badge variant="info">{riskSummary.holdingsCount}종목</Badge>
              </CardHeader>
              <dl className="grid gap-3 sm:grid-cols-2">
                <div className="rounded-lg bg-[var(--surface-elevated)] px-4 py-3">
                  <dt className="text-xs text-[var(--text-muted)]">최대 종목 비중</dt>
                  <dd className="mt-1 text-sm font-medium text-[var(--text-primary)]">
                    {riskSummary.topName}{" "}
                    <span className="font-mono text-amber-400">
                      {riskSummary.topWeight.toFixed(1)}%
                    </span>
                  </dd>
                </div>
                <div className="rounded-lg bg-[var(--surface-elevated)] px-4 py-3">
                  <dt className="text-xs text-[var(--text-muted)]">미국 자산 비중</dt>
                  <dd className="mt-1 font-mono text-sm font-medium text-[var(--text-primary)]">
                    {riskSummary.usWeight.toFixed(1)}%
                  </dd>
                </div>
                <div className="rounded-lg bg-[var(--surface-elevated)] px-4 py-3">
                  <dt className="text-xs text-[var(--text-muted)]">누적 수익률</dt>
                  <dd
                    className={cn(
                      "mt-1 font-mono text-sm font-medium",
                      trendTextClass(riskSummary.totalReturnRate),
                    )}
                  >
                    {formatPercent(riskSummary.totalReturnRate)}
                  </dd>
                </div>
                <div className="rounded-lg bg-[var(--surface-elevated)] px-4 py-3">
                  <dt className="text-xs text-[var(--text-muted)]">오늘 변동</dt>
                  <dd
                    className={cn(
                      "mt-1 font-mono text-sm font-medium",
                      trendTextClass(riskSummary.todayChangeRate),
                    )}
                  >
                    {formatPercent(riskSummary.todayChangeRate)}
                  </dd>
                </div>
              </dl>
            </Card>
          )}

          {insights && <RiskAlerts alerts={insights.risks} />}
        </div>
      </main>
    </>
  );
}
