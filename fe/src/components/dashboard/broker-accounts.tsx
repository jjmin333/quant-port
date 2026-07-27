import Link from "next/link";
import { ChevronRight } from "lucide-react";
import { Card, CardHeader, CardTitle } from "@/components/ui/card";
import { CurrencyKRW } from "@/components/ui/currency-krw";
import { formatPercent } from "@/lib/format";
import type { BrokerAccountSummary } from "@/lib/portfolio-types";
import { trendTextClass } from "@/lib/trend-colors";
import { cn } from "@/lib/utils";

export function BrokerAccounts({ accounts = [] }: { accounts?: BrokerAccountSummary[] }) {
  return (
    <Card className="min-w-[var(--dashboard-content-min)] shrink-0">
      <CardHeader>
        <CardTitle>증권사별 자산</CardTitle>
        <span className="text-xs text-[var(--text-muted)]">토스증권 계좌 기준</span>
      </CardHeader>
      {accounts.length === 0 ? (
        <p className="px-5 pb-5 text-sm text-[var(--text-muted)]">연결된 계좌가 없습니다.</p>
      ) : (
        <ul className="space-y-2">
          {accounts.map((account) => (
            <li key={account.id}>
              <Link
                href={`/holdings?account=${account.accountSeq}`}
                className="group flex min-w-0 shrink-0 items-center justify-between gap-3 rounded-lg border border-transparent px-3 py-2.5 transition-colors hover:border-[var(--border)] hover:bg-[var(--surface-elevated)]"
              >
                <div className="min-w-0 flex-1">
                  <p className="whitespace-nowrap text-sm font-medium text-[var(--text-primary)]">
                    {account.name}
                  </p>
                  <CurrencyKRW value={account.balanceKRW} size="md" compact />
                </div>
                <div className="flex shrink-0 items-center gap-2">
                  <span
                    className={cn(
                      "font-mono text-sm",
                      trendTextClass(account.changeRate),
                    )}
                  >
                    {formatPercent(account.changeRate)}
                  </span>
                  <ChevronRight className="h-4 w-4 text-[var(--text-muted)] group-hover:text-[var(--text-secondary)]" />
                </div>
              </Link>
            </li>
          ))}
        </ul>
      )}
    </Card>
  );
}
