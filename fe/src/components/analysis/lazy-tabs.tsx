"use client";

import dynamic from "next/dynamic";

/**
 * These three tabs are the only recharts consumers in the analysis / stock
 * workspace views, and only one tab is mounted at a time. Loading them lazily
 * keeps recharts out of the initial bundle for everyone who never opens a
 * chart tab.
 */

function TabFallback() {
  return (
    <div className="flex min-h-[240px] items-center justify-center text-xs text-[var(--text-muted)]">
      불러오는 중…
    </div>
  );
}

export const CompanyInfoTab = dynamic(
  () =>
    import("@/components/analysis/company-info-tab").then((m) => ({
      default: m.CompanyInfoTab,
    })),
  { loading: TabFallback, ssr: false },
);

export const ValuationTab = dynamic(
  () =>
    import("@/components/analysis/valuation-tab").then((m) => ({
      default: m.ValuationTab,
    })),
  { loading: TabFallback, ssr: false },
);

export const FinancialsChartsTab = dynamic(
  () =>
    import("@/components/analysis/financials-charts-tab").then((m) => ({
      default: m.FinancialsChartsTab,
    })),
  { loading: TabFallback, ssr: false },
);
