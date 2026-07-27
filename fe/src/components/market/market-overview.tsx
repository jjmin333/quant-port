"use client";

import Link from "next/link";
import { PriceDisplay } from "@/components/ui/price-display";
import { formatPercent } from "@/lib/format";
import type { MarketIndex } from "@/lib/quotes-types";
import { useMarketIndices } from "@/hooks/use-market-indices";
import { cn } from "@/lib/utils";
import { trendTextClass } from "@/lib/trend-colors";
import { IndexSparkline } from "@/components/market/index-sparkline";

const GROUPS: { title: string; ids: string[] }[] = [
  { title: "국내", ids: ["kospi", "kosdaq", "usd-krw"] },
  {
    title: "미국 · 변동성",
    ids: ["nasdaq", "nasdaq100", "nq100", "sp500", "dow", "vix", "sox"],
  },
  { title: "원자재 · 암호화폐", ids: ["btc", "gold"] },
];

function IndexValueDisplay({ index }: { index: MarketIndex }) {
  if (index.value <= 0) {
    return <span className="text-[var(--text-muted)]">—</span>;
  }

  if (index.currency === "USD") {
    return (
      <PriceDisplay
        value={index.value}
        currency="USD"
        size="lg"
        className="text-xl font-semibold tracking-tight"
      />
    );
  }

  if (index.id === "btc") {
    return (
      <PriceDisplay
        value={index.value}
        currency="KRW"
        size="lg"
        className="text-xl font-semibold tracking-tight"
      />
    );
  }

  return (
    <span className="font-sans text-xl font-semibold tabular-nums tracking-tight text-[var(--text-primary)]">
      {index.value.toLocaleString("ko-KR", { maximumFractionDigits: 2 })}
    </span>
  );
}

function IndexBadge({ badge }: { badge?: string }) {
  if (!badge) return null;
  return (
    <span className="rounded bg-[var(--surface-elevated)] px-1.5 py-0.5 text-[10px] text-[var(--text-muted)]">
      {badge}
    </span>
  );
}

function formatIndexTimestamp(index: MarketIndex): string | null {
  if (!index.updatedAt) return null;
  const at = new Date(index.updatedAt);
  if (Number.isNaN(at.getTime())) return null;
  const time = at.toLocaleTimeString("ko-KR", { hour: "2-digit", minute: "2-digit" });
  if (index.dataKind === "eod") {
    const date = at.toLocaleDateString("ko-KR", { month: "numeric", day: "numeric" });
    return `${date} ${time} 종가`;
  }
  return `${time} 갱신`;
}

function DataKindBadge({ dataKind }: { dataKind?: MarketIndex["dataKind"] }) {
  if (!dataKind) return null;
  const label = dataKind === "eod" ? "종가" : "실시간";
  return (
    <span
      className={cn(
        "rounded px-1.5 py-0.5 text-[10px]",
        dataKind === "eod"
          ? "bg-[var(--surface-elevated)] text-[var(--text-muted)]"
          : "bg-[var(--accent-muted)] text-[var(--accent)]",
      )}
      title={
        dataKind === "eod"
          ? "전일 또는 최근 거래일 종가 기준"
          : "실시간 또는 장중 시세 (Finnhub·Upbit·금 spot)"
      }
    >
      {label}
    </span>
  );
}

function IndexCard({ index }: { index: MarketIndex }) {
  const hasValue = index.value > 0;
  const sparkline = index.sparkline ?? [];
  const indexUpdated = formatIndexTimestamp(index);

  return (
    <div className="flex min-h-[8.5rem] flex-col justify-between rounded-xl border border-[var(--border)] bg-[var(--surface)] p-4 shadow-sm">
      <div className="flex items-start justify-between gap-2">
        <div className="flex min-w-0 flex-col gap-1">
          <span className="text-sm font-medium text-[var(--text-muted)]">{index.name}</span>
          {indexUpdated && (
            <span className="text-[10px] text-[var(--text-muted)]">{indexUpdated}</span>
          )}
        </div>
        <div className="flex shrink-0 flex-wrap items-center justify-end gap-1">
          <DataKindBadge dataKind={index.dataKind} />
          <IndexBadge badge={index.badge} />
        </div>
      </div>
      <div className="mt-2 flex items-end justify-between gap-3">
        <div>
          <p className="text-[var(--text-primary)]">
            <IndexValueDisplay index={index} />
          </p>
          <p
            className={cn(
              "mt-1 font-sans text-sm tabular-nums",
              hasValue ? trendTextClass(index.changeRate) : "text-[var(--text-muted)]",
            )}
          >
            {hasValue ? formatPercent(index.changeRate) : "—"}
          </p>
        </div>
        {hasValue && sparkline.length >= 2 && (
          <IndexSparkline points={sparkline} changeRate={index.changeRate} />
        )}
      </div>
    </div>
  );
}

function groupIndices(all: MarketIndex[], ids: string[]): MarketIndex[] {
  const byId = new Map(all.map((i) => [i.id, i]));
  return ids.map((id) => byId.get(id)).filter((i): i is MarketIndex => i != null);
}

export function MarketOverview() {
  const { indices, updatedAt } = useMarketIndices();
  const hasLiveData = indices.some((i) => i.value > 0);

  return (
    <section className="space-y-4">
      <div className="flex flex-wrap items-end justify-between gap-3">
        <div>
          <h2 className="text-lg font-semibold text-[var(--text-primary)]">글로벌 시장 한눈에</h2>
          <p className="mt-1 text-sm text-[var(--text-muted)]">
            국내·미국 지수, 환율, 비트코인, 금 등 주요 지표를 확인합니다. 종가·실시간 배지로 데이터 성격을 구분합니다.
            {!hasLiveData && (
              <span className="ml-1 text-[var(--text-muted)]">· 데이터 갱신 중…</span>
            )}
            {updatedAt && (
              <span className="ml-1 text-[var(--text-muted)]">
                · {new Date(updatedAt).toLocaleTimeString("ko-KR", { hour: "2-digit", minute: "2-digit" })} 기준
              </span>
            )}
          </p>
        </div>
        <Link
          href="/quotes"
          className="text-sm font-medium text-[var(--accent)] hover:underline"
        >
          실시간 시세 →
        </Link>
      </div>

      <div className="space-y-6">
        {GROUPS.map((group) => {
          const items = groupIndices(indices, group.ids);
          if (items.length === 0) return null;
          return (
            <div key={group.title}>
              <h3 className="mb-3 text-xs font-semibold uppercase tracking-wide text-[var(--text-muted)]">
                {group.title}
              </h3>
              <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 2xl:grid-cols-5">
                {items.map((index) => (
                  <IndexCard key={index.id} index={index} />
                ))}
              </div>
            </div>
          );
        })}
      </div>
    </section>
  );
}

export function MarketOverviewCompact() {
  const { indices } = useMarketIndices();

  return (
    <div className="flex shrink-0 overflow-x-auto border-b border-[var(--border)] bg-[var(--surface)]">
      <div className="flex min-w-max items-center py-2">
        {indices.map((index) => {
          const hasValue = index.value > 0;
          return (
            <div
              key={index.id}
              className="flex shrink-0 items-center gap-3 whitespace-nowrap border-r border-[var(--border)] px-4 last:border-r-0"
            >
              <span className="text-xs font-medium text-[var(--text-muted)]">{index.name}</span>
              <span className="text-sm font-semibold text-[var(--text-primary)]">
                <IndexValueDisplay index={index} />
              </span>
              <span
                className={cn(
                  "font-sans text-xs tabular-nums",
                  hasValue ? trendTextClass(index.changeRate) : "text-[var(--text-muted)]",
                )}
              >
                {hasValue ? formatPercent(index.changeRate) : "—"}
              </span>
            </div>
          );
        })}
      </div>
    </div>
  );
}
