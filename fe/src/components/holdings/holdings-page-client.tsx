"use client";

import Link from "next/link";
import { PageHeader } from "@/components/layout/header";
import { useHeaderSearch } from "@/components/providers/header-search-provider";
import { PortfolioRefreshStatus } from "@/components/dashboard/portfolio-status";
import { Card } from "@/components/ui/card";
import { CurrencyKRW } from "@/components/ui/currency-krw";
import { usePortfolioPolling } from "@/hooks/use-portfolio-polling";
import { seedExchangeRate } from "@/hooks/use-exchange-rate";
import { resolveHoldingDisplay, type HoldingDisplay } from "@/lib/holding-display";
import type { PortfolioSummary } from "@/lib/portfolio-types";
import { sortHoldingsByValue } from "@/lib/asset-allocation";
import { SessionChangeCell } from "@/components/holdings/session-change-display";
import { formatDateKST, formatPercent, formatPrice, type PriceFormatOptions } from "@/lib/format";
import { trendTextClass } from "@/lib/trend-colors";

import { cn } from "@/lib/utils";

function priceFormatOptions(display: HoldingDisplay, kind: "avg" | "market" | "spot"): PriceFormatOptions | undefined {
  if (display.currency !== "USD") return undefined;
  if (kind === "avg") return { krwValue: display.avgPrice * display.purchaseFxRate };
  if (kind === "market") return { krwValue: display.marketValueKRW };
  return { exchangeRate: display.exchangeRate };
}

function PriceValue({
  display,
  value,
  kind = "spot",
  size = "lg",
}: {
  display: HoldingDisplay;
  value: number;
  kind?: "avg" | "market" | "spot";
  size?: "sm" | "lg";
}) {
  if (display.currency === "USD") {
    return (
      <p className={cn("font-mono font-semibold", size === "lg" ? "text-lg" : "text-sm")}>
        {formatPrice(value, "USD", priceFormatOptions(display, kind))}
      </p>
    );
  }
  return <CurrencyKRW value={value} size={size} compact={size === "sm"} />;
}

function HoldingsList({
  summary,
  syncing,
  filterQuery = "",
}: {
  summary: PortfolioSummary;
  syncing?: boolean;
  filterQuery?: string;
}) {
  const holdings = sortHoldingsByValue(summary.holdings).filter((stock) => {
    const q = filterQuery.trim().toLowerCase();
    if (!q) return true;
    return (
      stock.symbol.toLowerCase().includes(q) ||
      stock.name.toLowerCase().includes(q)
    );
  });

  if (holdings.length === 0) {
    const likelyLoading =
      syncing ||
      (summary.totalAssetsKRW > 0 && summary.accounts.length > 0 && !filterQuery.trim());
    return (
      <Card className="p-6 text-center text-sm text-[var(--text-muted)]">
        {filterQuery.trim()
          ? "검색 결과가 없습니다."
          : likelyLoading
            ? "보유 종목을 불러오는 중…"
            : "보유 종목이 없습니다."}
      </Card>
    );
  }

  return holdings.map((stock) => {
    const display = resolveHoldingDisplay(stock, summary.exchangeRate);
    const hasRange =
      (display.sincePurchaseHigh ?? 0) > 0 && (display.sincePurchaseLow ?? 0) > 0;

    return (
      <Card key={stock.symbol} className="overflow-hidden p-0">
        <div className="flex flex-col gap-4 p-5 sm:flex-row sm:items-start sm:justify-between">
          <div>
            <Link
              href={`/stocks/${stock.symbol}`}
              className="text-lg font-semibold text-[var(--text-primary)] hover:text-[var(--accent)]"
            >
              {stock.name}
            </Link>
            <p className="text-xs text-[var(--text-muted)]">
              {stock.symbol} · {stock.currency}
              {display.purchaseDate && (
                <> · 보유 시작 {formatDateKST(display.purchaseDate)}</>
              )}
            </p>
          </div>
          <div className="flex flex-wrap gap-6">
            <div>
              <p className="text-xs text-[var(--text-muted)]">총 보유</p>
              <p className="font-mono text-lg font-semibold">{stock.totalShares}주</p>
            </div>
            <div>
              <p className="text-xs text-[var(--text-muted)]">평균단가</p>
              <PriceValue display={display} value={display.avgPrice} kind="avg" />
            </div>
            <div>
              <p className="text-xs text-[var(--text-muted)]">현재가</p>
              <PriceValue display={display} value={display.currentPrice} kind="spot" />
            </div>
            <div>
              <p className="text-xs text-[var(--text-muted)]">전일대비</p>
              <SessionChangeCell display={display} variant="regular" size="lg" />
            </div>
            <div>
              <p className="text-xs text-[var(--text-muted)]">시간외</p>
              <SessionChangeCell display={display} variant="extended" size="lg" />
            </div>
            <div>
              <p className="text-xs text-[var(--text-muted)]">평가금액</p>
              <PriceValue display={display} value={display.marketValue} kind="market" />
            </div>
            <div>
              <p className="text-xs text-[var(--text-muted)]">수익률</p>
              <p className={cn("font-mono text-lg font-semibold", trendTextClass(display.returnRate))}>
                {formatPercent(display.returnRate)}
              </p>
            </div>
          </div>
        </div>

        {hasRange && (
          <div className="border-t border-[var(--border)] bg-[var(--surface-elevated)] px-5 py-3">
            <p className="mb-2 text-xs font-medium text-[var(--text-muted)]">현재 보유 기간 고가 / 저가</p>
            <dl className="grid grid-cols-1 gap-3 sm:grid-cols-2">
              <div>
                <dt className="text-xs text-[var(--text-muted)]">최고가</dt>
                <dd className="font-semibold text-[var(--profit)]">
                  <PriceValue display={display} value={display.sincePurchaseHigh!} size="sm" />
                  {display.sincePurchaseHighDate && (
                    <span className="ml-2 text-xs font-normal text-[var(--text-muted)]">
                      {formatDateKST(display.sincePurchaseHighDate)}
                    </span>
                  )}
                  {display.sincePurchaseHighReturnRate != null && (
                    <span
                      className={cn(
                        "ml-2 text-xs font-normal",
                        trendTextClass(display.sincePurchaseHighReturnRate),
                      )}
                    >
                      ({formatPercent(display.sincePurchaseHighReturnRate)})
                    </span>
                  )}
                </dd>
              </div>
              <div>
                <dt className="text-xs text-[var(--text-muted)]">최저가</dt>
                <dd className="font-semibold text-[var(--loss)]">
                  <PriceValue display={display} value={display.sincePurchaseLow!} size="sm" />
                  {display.sincePurchaseLowDate && (
                    <span className="ml-2 text-xs font-normal text-[var(--text-muted)]">
                      {formatDateKST(display.sincePurchaseLowDate)}
                    </span>
                  )}
                  {display.sincePurchaseLowReturnRate != null && (
                    <span
                      className={cn(
                        "ml-2 text-xs font-normal",
                        trendTextClass(display.sincePurchaseLowReturnRate),
                      )}
                    >
                      ({formatPercent(display.sincePurchaseLowReturnRate)})
                    </span>
                  )}
                </dd>
              </div>
            </dl>
          </div>
        )}

        <div className="border-t border-[var(--border)] bg-[var(--surface-elevated)] px-5 py-3">
          <p className="mb-2 text-xs font-medium text-[var(--text-muted)]">증권사별 보유</p>
          <div className="flex flex-wrap gap-3">
            {stock.accounts.map((account) => (
              <span
                key={`${stock.symbol}-${account.broker}`}
                className="rounded-md border border-[var(--border)] bg-[var(--surface)] px-3 py-1.5 text-sm"
              >
                <span className="text-[var(--text-secondary)]">{account.broker}</span>
                <span className="ml-2 font-mono font-medium text-[var(--text-primary)]">
                  {account.shares}주
                </span>
              </span>
            ))}
          </div>
        </div>
      </Card>
    );
  });
}

export function HoldingsPageClient({ initialSummary }: { initialSummary: PortfolioSummary }) {
  seedExchangeRate(initialSummary.exchangeRate);
  const { summary, isRefreshing, isLiveRefreshing, lastError } = usePortfolioPolling(initialSummary);
  const { activeQuery, mode, isSearchActive } = useHeaderSearch();
  const stockFilter = mode === "stock" && isSearchActive ? activeQuery : "";
  const showFxNote = summary.holdings.some((h) => h.currency === "USD");

  return (
    <>
      <PageHeader title="보유종목 통합" subtitle="토스증권 계좌 보유 종목 실시간 조회" />
      <main className="flex-1 overflow-y-auto p-6">
        <div className="mx-auto max-w-5xl space-y-4">
          <PortfolioRefreshStatus
            summary={summary}
            isRefreshing={isRefreshing}
            isLiveRefreshing={isLiveRefreshing}
            pollError={lastError}
          />

          {showFxNote && (
            <p className="px-1 text-xs text-[var(--text-muted)]">
              미국주식은 달러(원) 형식으로 표시됩니다 · USD/KRW{" "}
              {summary.exchangeRate.toLocaleString("ko-KR")} (현재) · 매입{" "}
              {(summary.avgPurchaseFxRate ?? summary.exchangeRate).toFixed(2)} (가중평균)
            </p>
          )}

          <HoldingsList
            summary={summary}
            syncing={isRefreshing || isLiveRefreshing}
            filterQuery={stockFilter}
          />
        </div>
      </main>
    </>
  );
}
