import { PageHeader } from "@/components/layout/header";
import { AllocationPieChart } from "@/components/portfolio/allocation-pie-chart";
import { PortfolioErrorBanner } from "@/components/dashboard/portfolio-status";
import { fetchPortfolioDiagnostics, fetchPortfolioSummary } from "@/lib/api-client";
import {
  calculateCountryAllocation,
  calculateSectorAllocation,
} from "@/lib/portfolio-allocation";

export default async function PortfolioPage() {
  let summary = null;
  let errorMessage: string | null = null;
  let diagnostics = null;

  try {
    summary = await fetchPortfolioSummary();
  } catch (err) {
    errorMessage = err instanceof Error ? err.message : "알 수 없는 오류";
    try {
      diagnostics = await fetchPortfolioDiagnostics();
    } catch {
      // ignore
    }
  }

  const countrySlices = summary ? calculateCountryAllocation(summary) : [];
  const sectorSlices = summary ? calculateSectorAllocation(summary) : [];

  return (
    <>
      <PageHeader title="포트폴리오 분석" subtitle="섹터 · 국가별 자산 배분" />
      <main className="flex-1 overflow-y-auto p-6">
        <div className="mx-auto min-w-[var(--dashboard-content-min)] max-w-5xl space-y-6">
          {errorMessage && (
            <PortfolioErrorBanner message={errorMessage} diagnostics={diagnostics} />
          )}

          {summary && (
            <div className="grid gap-6 lg:grid-cols-2">
              <AllocationPieChart title="국가별 비중" slices={countrySlices} />
              <AllocationPieChart title="자산 유형별 비중" slices={sectorSlices} />
            </div>
          )}
        </div>
      </main>
    </>
  );
}
