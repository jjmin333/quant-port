"use client";

import Link from "next/link";
import { Card, CardHeader, CardTitle } from "@/components/ui/card";
import { CurrencyKRW } from "@/components/ui/currency-krw";
import { SessionChangeCell } from "@/components/holdings/session-change-display";
import { formatDateKST, formatPercent, formatPrice, type PriceFormatOptions } from "@/lib/format";
import { resolveHoldingDisplay, type HoldingDisplay } from "@/lib/holding-display";
import type { UnifiedHolding } from "@/lib/portfolio-types";
import { sortHoldingsByValue } from "@/lib/asset-allocation";
import { trendTextClass } from "@/lib/trend-colors";
import { cn } from "@/lib/utils";

function priceFormatOptions(display: HoldingDisplay, kind: "avg" | "spot"): PriceFormatOptions | undefined {
  if (display.currency !== "USD") return undefined;
  if (kind === "avg") return { krwValue: display.avgPrice * display.purchaseFxRate };
  return { exchangeRate: display.exchangeRate };
}

function PriceCell({
  display,
  value,
  kind = "spot",
}: {
  display: HoldingDisplay;
  value: number;
  kind?: "avg" | "spot";
}) {
  if (display.currency === "USD") {
    return (
      <span className="font-mono text-sm">
        {formatPrice(value, "USD", priceFormatOptions(display, kind))}
      </span>
    );
  }
  return <CurrencyKRW value={value} size="sm" compact />;
}

export function HoldingsPreview({
  holdings = [],
  exchangeRate = 0,
  avgPurchaseFxRate = 0,
}: {
  holdings?: UnifiedHolding[];
  exchangeRate?: number;
  avgPurchaseFxRate?: number;
}) {
  const sorted = sortHoldingsByValue(holdings).slice(0, 6);
  const showFxNote = holdings.some((h) => h.currency === "USD");

  return (
    <Card className="w-full min-w-0 shrink-0">
      <CardHeader>
        <div className="flex flex-wrap items-center gap-3">
          <CardTitle>보유종목 통합</CardTitle>
        </div>
        <div className="flex flex-col items-end gap-1">
          <Link href="/holdings" className="text-xs text-[var(--accent)] hover:underline">
            전체 보기 ({holdings.length})
          </Link>
          {showFxNote && (
            <span className="text-[10px] text-[var(--text-muted)]">
              USD/KRW {exchangeRate.toLocaleString("ko-KR")} · 매입 {(avgPurchaseFxRate ?? exchangeRate).toFixed(2)}
            </span>
          )}
        </div>
      </CardHeader>
      {sorted.length === 0 ? (
        <p className="px-5 pb-5 text-sm text-[var(--text-muted)]">보유 종목이 없습니다.</p>
      ) : (
        <div className="overflow-x-auto px-1 pb-1">
          <table className="w-full min-w-[960px] text-sm">
            <thead>
              <tr className="border-b border-[var(--border)] text-left text-[var(--text-muted)]">
                <th className="px-4 pb-2 font-medium">종목</th>
                <th className="px-3 pb-2 font-medium">총 보유</th>
                <th className="px-3 pb-2 font-medium">평균단가</th>
                <th className="px-3 pb-2 font-medium">현재가</th>
                <th className="px-3 pb-2 font-medium">전일대비</th>
                <th className="px-3 pb-2 font-medium">시간외</th>
                <th className="px-3 pb-2 font-medium">수익률</th>
                <th className="px-3 pb-2 font-medium">보유중 고/저</th>
              </tr>
            </thead>
            <tbody>
              {sorted.map((stock) => {
                const display = resolveHoldingDisplay(stock, exchangeRate);
                const hasRange =
                  (display.sincePurchaseHigh ?? 0) > 0 && (display.sincePurchaseLow ?? 0) > 0;

                return (
                  <tr
                    key={stock.symbol}
                    className="border-b border-[var(--border)]/50 last:border-0"
                  >
                    <td className="px-4 py-3">
                      <Link
                        href={`/stocks/${stock.symbol}`}
                        className="block truncate font-medium text-[var(--text-primary)] hover:text-[var(--accent)]"
                      >
                        {stock.name}
                      </Link>
                      <p className="truncate text-xs text-[var(--text-muted)]">
                        {stock.accounts.map((a) => `${a.broker} ${a.shares}주`).join(" · ")}
                      </p>
                    </td>
                    <td className="whitespace-nowrap px-3 py-3 font-mono text-[var(--text-primary)]">
                      {stock.totalShares}주
                    </td>
                    <td className="whitespace-nowrap px-3 py-3 text-[var(--text-secondary)]">
                      <PriceCell display={display} value={display.avgPrice} kind="avg" />
                    </td>
                    <td className="whitespace-nowrap px-3 py-3">
                      <PriceCell display={display} value={display.currentPrice} />
                    </td>
                    <td className="whitespace-nowrap px-3 py-3">
                      <SessionChangeCell display={display} variant="regular" />
                    </td>
                    <td className="whitespace-nowrap px-3 py-3">
                      <SessionChangeCell display={display} variant="extended" />
                    </td>
                    <td className={cn("whitespace-nowrap px-3 py-3 font-mono font-medium", trendTextClass(display.returnRate))}>
                      {formatPercent(display.returnRate)}
                    </td>
                    <td className="whitespace-nowrap px-3 py-3 text-xs">
                      {hasRange ? (
                        <div className="space-y-1">
                          <p>
                            <span className="text-[var(--text-muted)]">고 </span>
                            <span className="font-mono text-[var(--profit)]">
                              <PriceCell display={display} value={display.sincePurchaseHigh!} />
                            </span>
                            {display.sincePurchaseHighDate && (
                              <span className="ml-1 text-[10px] text-[var(--text-muted)]">
                                {formatDateKST(display.sincePurchaseHighDate)}
                              </span>
                            )}
                            {display.sincePurchaseHighReturnRate != null && (
                              <span
                                className={cn(
                                  "ml-1 font-mono text-[10px]",
                                  trendTextClass(display.sincePurchaseHighReturnRate),
                                )}
                              >
                                ({formatPercent(display.sincePurchaseHighReturnRate)})
                              </span>
                            )}
                          </p>
                          <p>
                            <span className="text-[var(--text-muted)]">저 </span>
                            <span className="font-mono text-[var(--loss)]">
                              <PriceCell display={display} value={display.sincePurchaseLow!} />
                            </span>
                            {display.sincePurchaseLowDate && (
                              <span className="ml-1 text-[10px] text-[var(--text-muted)]">
                                {formatDateKST(display.sincePurchaseLowDate)}
                              </span>
                            )}
                            {display.sincePurchaseLowReturnRate != null && (
                              <span
                                className={cn(
                                  "ml-1 font-mono text-[10px]",
                                  trendTextClass(display.sincePurchaseLowReturnRate),
                                )}
                              >
                                ({formatPercent(display.sincePurchaseLowReturnRate)})
                              </span>
                            )}
                          </p>
                        </div>
                      ) : (
                        <span className="text-[var(--text-muted)]">—</span>
                      )}
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      )}
    </Card>
  );
}
