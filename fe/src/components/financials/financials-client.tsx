"use client";

import { useEffect, useState } from "react";
import { PageHeader } from "@/components/layout/header";
import { Card, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { fetchFundamentalsClient } from "@/lib/fundamentals-api";
import {
  COVERAGE_TIER_LABEL,
  type CoverageTier,
  type StockFundamentalsPanel,
} from "@/lib/fundamentals-types";
import {
  mapFundamentalsToPanel,
  metricGaugeClass,
  metricToneClass,
} from "@/lib/fundamentals-mapper";
import { formatListDate, formatPrice } from "@/lib/format";
import type { StockInfo } from "@/lib/quotes-types";
import { cn } from "@/lib/utils";

type StockRow = StockInfo & { fetchError?: string };

type FinancialsClientProps = {
  stocks: StockRow[];
};

export function FinancialsClient({ stocks }: FinancialsClientProps) {
  const [selectedSymbol, setSelectedSymbol] = useState<string | null>(
    stocks[0]?.symbol ?? null,
  );
  const [fundamentals, setFundamentals] = useState<StockFundamentalsPanel | null>(null);
  const [loading, setLoading] = useState(false);
  const [fetchError, setFetchError] = useState<string | null>(null);

  const selectedStock = stocks.find((s) => s.symbol === selectedSymbol) ?? null;

  useEffect(() => {
    if (!selectedStock) {
      setFundamentals(null);
      setFetchError(null);
      return;
    }

    let cancelled = false;
    setLoading(true);
    setFetchError(null);
    setFundamentals(null);

    fetchFundamentalsClient(selectedStock.symbol)
      .then((data) => {
        if (cancelled) return;
        if (data.coverageTier === "unavailable") {
          setFetchError("재무 데이터를 불러올 수 없습니다.");
          return;
        }
        setFundamentals(mapFundamentalsToPanel(data, selectedStock));
      })
      .catch((err) => {
        if (cancelled) return;
        setFetchError(err instanceof Error ? err.message : "펀더멘털 조회 실패");
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });

    return () => {
      cancelled = true;
    };
  }, [selectedStock]);

  return (
    <>
      <PageHeader
        title="재무제표"
        subtitle="보유 종목 펀더멘털 · DART/EDGAR + Toss 실시간 PER/PBR"
      />
      <main className="flex-1 overflow-y-auto p-4 sm:p-6">
        {stocks.length === 0 ? (
          <div className="mx-auto max-w-5xl">
            <Card className="p-6 text-center text-sm text-[var(--text-muted)]">
              보유 종목이 없습니다.
            </Card>
          </div>
        ) : (
          <div className="mx-auto flex min-w-0 max-w-6xl flex-col gap-6 xl:flex-row xl:items-start">
            <Card className="min-w-0 flex-1 overflow-hidden p-0 xl:max-w-md xl:shrink-0">
              <CardHeader className="px-5 pt-5">
                <CardTitle>보유 종목</CardTitle>
                <Badge variant="info">{stocks.length}종목</Badge>
              </CardHeader>
              <p className="px-5 pb-3 text-xs text-[var(--text-muted)]">
                종목을 선택하면 펀더멘털 분석을 확인합니다.
              </p>
              <div className="max-h-[420px] overflow-y-auto border-t border-[var(--border)]">
                <table className="w-full text-sm">
                  <thead className="sticky top-0 bg-[var(--surface)]">
                    <tr className="text-left text-xs text-[var(--text-muted)]">
                      <th className="px-4 py-2 font-medium">종목</th>
                      <th className="hidden px-2 py-2 font-medium sm:table-cell">시장</th>
                      <th className="px-4 py-2 text-right font-medium">상장</th>
                    </tr>
                  </thead>
                  <tbody>
                    {stocks.map((stock) => {
                      const active = stock.symbol === selectedSymbol;
                      return (
                        <tr
                          key={stock.symbol}
                          className={cn(
                            "cursor-pointer border-t border-[var(--border)] transition-colors",
                            active
                              ? "bg-[var(--accent)]/10"
                              : "hover:bg-[var(--surface-elevated)]/60",
                          )}
                          onClick={() => setSelectedSymbol(stock.symbol)}
                        >
                          <td className="px-4 py-3">
                            <p className="font-medium text-[var(--text-primary)]">{stock.name}</p>
                            <p className="text-xs text-[var(--text-muted)]">{stock.symbol}</p>
                            {stock.fetchError && (
                              <p className="text-xs text-rose-400">{stock.fetchError}</p>
                            )}
                          </td>
                          <td className="hidden px-2 py-3 text-xs text-[var(--text-secondary)] sm:table-cell">
                            {stock.market}
                          </td>
                          <td className="px-4 py-3 text-right text-xs tabular-nums text-[var(--text-secondary)]">
                            {stock.listDate ? formatListDate(stock.listDate) : "—"}
                          </td>
                        </tr>
                      );
                    })}
                  </tbody>
                </table>
              </div>
            </Card>

            <div className="min-w-0 flex-1 space-y-4">
              {loading ? (
                <Card className="flex min-h-[320px] items-center justify-center p-8 text-center text-sm text-[var(--text-muted)]">
                  펀더멘털 데이터를 불러오는 중…
                </Card>
              ) : fetchError ? (
                <Card className="flex min-h-[320px] flex-col items-center justify-center gap-2 p-8 text-center text-sm text-[var(--text-muted)]">
                  <p className="text-rose-400">{fetchError}</p>
                  <p className="text-xs">종목 적재 후 다시 시도하거나, 다른 종목을 선택해 보세요.</p>
                </Card>
              ) : fundamentals && selectedStock ? (
                <FundamentalPanel data={fundamentals} catalog={selectedStock} />
              ) : (
                <Card className="flex min-h-[320px] items-center justify-center p-8 text-center text-sm text-[var(--text-muted)]">
                  왼쪽에서 종목을 선택해 주세요.
                </Card>
              )}
            </div>
          </div>
        )}
      </main>
    </>
  );
}

function FundamentalPanel({
  data,
  catalog,
}: {
  data: StockFundamentalsPanel;
  catalog: StockRow;
}) {
  const scoreTone =
    data.score >= 75 ? "text-emerald-400" : data.score >= 55 ? "text-violet-400" : "text-amber-400";
  const tierVariant = tierBadgeVariant(data.coverageTier);

  return (
    <div className="space-y-4">
      <Card className="p-0">
        <div className="flex flex-wrap items-start justify-between gap-4 border-b border-[var(--border)] px-5 py-4">
          <div>
            <p className="text-xs text-[var(--text-muted)]">펀더멘털 분석</p>
            <h2 className="mt-1 text-lg font-semibold text-[var(--text-primary)]">
              {data.name}
            </h2>
            <p className="text-xs text-[var(--text-muted)]">
              {data.symbol} · {catalog.market} · {catalog.currency}
              {data.fiscalYear ? ` · FY${data.fiscalYear}` : ""}
              {data.price != null ? ` · ${formatPrice(data.price, data.currency)}` : ""}
              {catalog.sharesOutstanding ? ` · 발행 ${catalog.sharesOutstanding}` : ""}
            </p>
          </div>
          <div className="flex items-center gap-3">
            <ScoreRing score={data.score} />
            <div className="text-right">
              <p className={cn("text-2xl font-bold tabular-nums", scoreTone)}>
                {data.score > 0 ? `+${data.score}` : "—"}
              </p>
              <p className={cn("text-sm font-semibold", scoreTone)}>{data.rating}</p>
            </div>
          </div>
        </div>

        <div className="border-b border-[var(--border)] bg-blue-500/5 px-5 py-4">
          <div className="mb-2 flex flex-wrap items-center gap-2">
            <Badge variant="info">핵심 요약</Badge>
            <Badge variant={tierVariant}>{COVERAGE_TIER_LABEL[data.coverageTier]}</Badge>
            <span className="text-[10px] uppercase text-[var(--text-muted)]">
              {data.dataSource} · {data.calcStatus}
            </span>
          </div>
          <p className="text-sm leading-relaxed text-[var(--text-secondary)]">{data.summary}</p>
          <div className="mt-3 flex flex-wrap gap-1.5">
            {data.tags.map((tag) => (
              <span
                key={tag}
                className="rounded-full bg-[var(--surface-elevated)] px-2.5 py-0.5 text-[11px] text-[var(--text-muted)]"
              >
                {tag}
              </span>
            ))}
          </div>
        </div>

        <div className="grid gap-3 p-4 sm:grid-cols-2">
          {data.metrics.map((metric) => (
            <MetricCard key={metric.id} metric={metric} />
          ))}
        </div>
      </Card>
    </div>
  );
}

function tierBadgeVariant(tier: CoverageTier): "profit" | "warning" | "info" {
  switch (tier) {
    case "full":
      return "profit";
    case "basic":
      return "warning";
    default:
      return "info";
  }
}

function MetricCard({
  metric,
}: {
  metric: StockFundamentalsPanel["metrics"][number];
}) {
  return (
    <div
      className={cn(
        "rounded-xl border border-[var(--border)] bg-[var(--surface-elevated)]/40 p-4",
        !metric.available && "opacity-70",
      )}
    >
      <div className="flex items-start justify-between gap-2">
        <span className="text-sm text-[var(--text-secondary)]">{metric.label}</span>
        <span className={cn("text-xs font-medium", metricToneClass(metric.tone))}>
          {metric.verdict}
        </span>
      </div>
      <p className="mt-2 font-mono text-2xl font-semibold tabular-nums text-[var(--text-primary)]">
        {metric.value}
      </p>
      {metric.available && (
        <div className="mt-3 h-1.5 overflow-hidden rounded-full bg-[var(--surface-elevated)]">
          <div
            className={cn("h-full rounded-full transition-all", metricGaugeClass(metric.tone))}
            style={{ width: `${Math.min(100, Math.max(4, metric.gauge))}%` }}
          />
        </div>
      )}
    </div>
  );
}

function ScoreRing({ score }: { score: number }) {
  const pct = Math.min(100, Math.max(0, score));
  const dash = `${pct} ${100 - pct}`;
  return (
    <svg viewBox="0 0 36 36" className="h-12 w-12 -rotate-90" aria-hidden>
      <circle
        cx="18"
        cy="18"
        r="15.5"
        fill="none"
        stroke="currentColor"
        strokeWidth="3"
        className="text-[var(--surface-elevated)]"
      />
      <circle
        cx="18"
        cy="18"
        r="15.5"
        fill="none"
        stroke="currentColor"
        strokeWidth="3"
        strokeDasharray={dash}
        className="text-emerald-500"
      />
    </svg>
  );
}
