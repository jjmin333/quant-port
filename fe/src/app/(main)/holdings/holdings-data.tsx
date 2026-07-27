import { PortfolioErrorBanner } from "@/components/dashboard/portfolio-status";
import { HoldingsPageClient } from "@/components/holdings/holdings-page-client";
import { fetchPortfolioDiagnostics, fetchPortfolioSummary } from "@/lib/api-client";

export async function HoldingsData() {
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

  if (errorMessage) {
    return (
      <main className="flex-1 overflow-y-auto p-6">
        <div className="mx-auto max-w-5xl">
          <PortfolioErrorBanner
            message={errorMessage}
            diagnostics={diagnostics}
            loginNext="/holdings"
          />
        </div>
      </main>
    );
  }

  if (!summary) return null;

  return <HoldingsPageClient initialSummary={summary} />;
}
