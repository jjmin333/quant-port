import { CurrencyKRW, CurrencyKRWPlain } from "@/components/ui/currency-krw";
import { StatCard } from "@/components/ui/stat-card";
import { PortfolioSourceBadge } from "@/components/dashboard/portfolio-status";
import { formatPercent } from "@/lib/format";
import type { PortfolioSummary } from "@/lib/portfolio-types";
import { trendTextClass } from "@/lib/trend-colors";
import { cn } from "@/lib/utils";

export function AssetSummary({ summary }: { summary: PortfolioSummary }) {
  const {
    totalAssetsKRW,
    totalReturnRate,
    todayChangeRate,
    todayProfitKRW,
    totalCashKRW,
    totalProfitKRW,
    source,
  } = summary;

  const profitPositive = totalProfitKRW >= 0;

  return (
    <section className="min-w-[var(--dashboard-content-min)] shrink-0 rounded-2xl border border-[var(--border)] bg-gradient-to-br from-[var(--surface)] to-[var(--surface-elevated)] p-4 sm:p-6">
      <div>
        <p className="text-xs font-medium uppercase tracking-wider text-[var(--text-muted)]">
          총 자산
        </p>
        <div className="mt-1">
          <CurrencyKRW value={totalAssetsKRW} size="hero" />
        </div>
        <PortfolioSourceBadge provider={source.provider} fetchedAt={source.fetchedAt} />
        <p className="mt-0.5 text-[10px] text-[var(--text-muted)]">
          매매기준율 환산 · 수수료/세금 반영 평가금 기준 (토스 앱과 동일)
        </p>
        <div className="mt-3 flex flex-wrap items-center gap-x-3 gap-y-1">
          <span
            className={cn(
              "shrink-0 whitespace-nowrap font-sans text-lg font-semibold tabular-nums",
              trendTextClass(todayChangeRate),
            )}
          >
            오늘 {formatPercent(todayChangeRate)}
          </span>
          <CurrencyKRWPlain
            value={todayProfitKRW}
            size="lg"
            signed
            compact
            className={trendTextClass(todayChangeRate, true)}
          />
        </div>
      </div>

      <div className="mt-6 grid grid-cols-2 gap-3 lg:grid-cols-4">
        <StatCard
          label="총 수익률"
          value={
            <span className="font-sans text-base font-semibold tabular-nums xl:text-xl">
              {formatPercent(totalReturnRate, true)}
            </span>
          }
          trend={profitPositive ? "up" : "down"}
        />
        <StatCard
          label="예수금"
          value={<CurrencyKRW value={totalCashKRW} size="xl" compact />}
        />
        <StatCard
          label="총 손익"
          value={<CurrencyKRWPlain value={totalProfitKRW} size="xl" signed compact />}
          trend={profitPositive ? "up" : "down"}
        />
        <StatCard
          label="오늘 수익"
          value={<CurrencyKRWPlain value={todayProfitKRW} size="xl" signed compact />}
          trend={todayChangeRate >= 0 ? "up" : "down"}
        />
      </div>
    </section>
  );
}
