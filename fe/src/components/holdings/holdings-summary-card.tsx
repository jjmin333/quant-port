"use client";

import { useState } from "react";
import { Wallet } from "lucide-react";
import { CashBalanceDialog } from "@/components/holdings/cash-balance-dialog";
import { Card } from "@/components/ui/card";
import { formatKRW, formatPercent, formatPriceChange } from "@/lib/format";
import type { HoldingsTotals } from "@/lib/holdings-calc";
import { trendTextClass } from "@/lib/trend-colors";
import { cn } from "@/lib/utils";

type StatItem = {
  key: string;
  label: string;
  value: string;
  sub?: string;
  tone?: string;
};

/**
 * 본문 최상단 요약 — 총 자산을 크게, 보조 지표는 아래 한 줄에 **카드 전체 너비**로 균등 배치.
 * 테두리·회색 칸 없이 글씨만 쓰고, 구분은 얇은 가로선 하나로만 한다.
 */
export function HoldingsSummaryCard({
  totals,
  loading,
  exchangeRate,
  onChanged,
}: {
  totals: HoldingsTotals | null;
  loading: boolean;
  exchangeRate: number;
  onChanged?: () => void;
}) {
  const [cashOpen, setCashOpen] = useState(false);

  if (loading) {
    return (
      <Card className="p-4 md:p-5">
        <div className="flex items-center justify-between">
          <div className="h-4 w-16 animate-pulse rounded bg-muted" />
          <div className="h-7 w-14 animate-pulse rounded bg-muted" />
        </div>
        <div className="mt-2 h-9 w-56 animate-pulse rounded bg-muted" />
        <div className="mt-4 border-t pt-4">
          <div className="grid grid-cols-2 gap-4 sm:grid-cols-3 lg:grid-cols-5">
            {Array.from({ length: 5 }).map((_, i) => (
              <div key={i} className="h-12 animate-pulse rounded bg-muted" />
            ))}
          </div>
        </div>
      </Card>
    );
  }

  if (!totals || (totals.pricedCount === 0 && !totals.hasCash)) {
    return (
      <>
        <Card className="p-6 text-center text-xs text-muted-foreground">
          <div className="mb-3 flex justify-center gap-2">
            <CashButton onClick={() => setCashOpen(true)} />
          </div>
          보유 종목이나 현금을 등록하면 총 자산과 손익이 여기에 표시됩니다.
        </Card>
        <CashBalanceDialog
          open={cashOpen}
          onOpenChange={setCashOpen}
          exchangeRate={exchangeRate}
          onSaved={onChanged}
        />
      </>
    );
  }

  const showTotalAssets = totals.hasCash;
  const headlineLabel = showTotalAssets ? "총 자산" : "총 평가금액";
  const headlineCanShow = showTotalAssets ? totals.canTotalAssets : totals.canTotal;
  const headlineValue = showTotalAssets ? totals.totalAssetsKRW : totals.marketValueKRW;

  const dayTone = trendTextClass(totals.dayPnlKRW);
  const pnlTone = totals.unrealizedPnlKRW == null ? "" : trendTextClass(totals.unrealizedPnlKRW);

  const cashUsdKRW =
    totals.cashUSD > 0 && exchangeRate > 0 ? totals.cashUSD * exchangeRate : null;

  const statItems: StatItem[] = [];

  if (totals.pricedCount > 0) {
    statItems.push(
      {
        key: "market",
        label: "주식 평가",
        value: totals.canTotal ? formatKRW(totals.marketValueKRW) : "—",
      },
      {
        key: "cost",
        label: "총 매입금액",
        value: totals.costBasisKRW == null ? "—" : formatKRW(totals.costBasisKRW),
        sub: `${totals.count}종목 · 국내 ${totals.krwCount} · 해외 ${totals.usdCount}`,
      },
      {
        key: "pnl",
        label: "평가손익",
        value:
          totals.unrealizedPnlKRW == null
            ? "—"
            : formatPriceChange(Math.round(totals.unrealizedPnlKRW), "KRW"),
        sub: totals.returnRate == null ? undefined : formatPercent(totals.returnRate),
        tone: pnlTone,
      },
    );
  }

  if (totals.hasCash) {
    statItems.push(
      { key: "cash-krw", label: "원화 현금", value: formatKRW(totals.cashKRW) },
      {
        key: "cash-usd",
        label: "달러 현금",
        value: `$${totals.cashUSD.toLocaleString()}`,
        sub:
          cashUsdKRW != null
            ? formatKRW(Math.round(cashUsdKRW))
            : totals.cashUSD > 0
              ? "환율 대기"
              : undefined,
      },
    );
  }

  return (
    <>
      <Card className="p-4 md:p-5">
        <div className="flex items-center justify-between gap-3">
          <p className="text-[11px] font-medium text-muted-foreground">{headlineLabel}</p>
          <CashButton onClick={() => setCashOpen(true)} />
        </div>

        <div className="mt-1 min-w-0">
          {headlineCanShow ? (
            <>
              <p className="font-mono text-[32px] leading-tight font-bold tracking-tight tabular-nums">
                {formatKRW(headlineValue)}
              </p>
              {totals.pricedCount > 0 ? (
                <p className={cn("mt-0.5 font-mono text-[13px] tabular-nums", dayTone)}>
                  오늘 {formatPriceChange(Math.round(totals.dayPnlKRW), "KRW")} (
                  {formatPercent(totals.dayPnlRate)})
                </p>
              ) : (
                <p className="mt-0.5 text-[11px] text-muted-foreground">
                  등록된 주식 종목이 없습니다
                </p>
              )}
            </>
          ) : (
            <>
              <p className="font-mono text-[32px] leading-tight font-bold tabular-nums text-muted-foreground">
                —
              </p>
              <p className="mt-0.5 text-[11px] text-muted-foreground">
                환율을 불러오는 중이라 달러 자산을 합산하지 못했습니다
              </p>
            </>
          )}
        </div>

        {statItems.length > 0 && (
          <div
            className={cn(
              "mt-4 grid gap-x-6 gap-y-4 border-t border-border/60 pt-4",
              statItems.length === 1 && "grid-cols-1",
              statItems.length === 2 && "grid-cols-2",
              statItems.length === 3 && "grid-cols-2 sm:grid-cols-3",
              statItems.length === 4 && "grid-cols-2 lg:grid-cols-4",
              statItems.length >= 5 && "grid-cols-2 sm:grid-cols-3 lg:grid-cols-5",
            )}
          >
            {statItems.map(({ key, ...item }) => (
              <Stat key={key} {...item} />
            ))}
          </div>
        )}
      </Card>

      <CashBalanceDialog
        open={cashOpen}
        onOpenChange={setCashOpen}
        exchangeRate={exchangeRate}
        onSaved={onChanged}
      />
    </>
  );
}

function CashButton({ onClick }: { onClick: () => void }) {
  return (
    <button
      type="button"
      onClick={onClick}
      className="flex shrink-0 items-center gap-1 rounded-md border px-2 py-1 text-[11px] text-muted-foreground hover:bg-accent hover:text-foreground"
    >
      <Wallet className="size-3" />
      현금
    </button>
  );
}

function Stat({ label, value, sub, tone }: StatItem) {
  return (
    <div className="min-w-0">
      <p className="text-[11px] text-muted-foreground">{label}</p>
      <p className={cn("font-mono text-[15px] font-semibold tabular-nums", tone)}>{value}</p>
      <p
        className={cn(
          "min-h-[14px] font-mono text-[10px] tabular-nums text-muted-foreground",
          tone,
        )}
      >
        {sub ?? "\u00a0"}
      </p>
    </div>
  );
}
