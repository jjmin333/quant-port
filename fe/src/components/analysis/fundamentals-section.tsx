"use client";

import { Card } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import {
  COVERAGE_TIER_LABEL,
  type FundamentalsData,
  type StockFundamentalsPanel,
} from "@/lib/fundamentals-types";
import {
  mapFundamentalsToPanel,
  metricGaugeClass,
  metricToneClass,
} from "@/lib/fundamentals-mapper";
import { formatPrice } from "@/lib/format";
import { useExchangeRate } from "@/hooks/use-exchange-rate";
import type { StockInfo } from "@/lib/quotes-types";
import { cn } from "@/lib/utils";

type FundamentalsSectionProps = {
  stock: StockInfo;
  data: FundamentalsData | null;
  loading: boolean;
};

export function FundamentalsSection({ stock, data, loading }: FundamentalsSectionProps) {
  if (loading) {
    return (
      <Card className="flex min-h-[280px] items-center justify-center p-8 text-sm text-[var(--text-muted)]">
        펀더멘털 데이터를 불러오는 중…
      </Card>
    );
  }

  if (!data) {
    return (
      <Card className="flex min-h-[280px] items-center justify-center p-8 text-sm text-[var(--text-muted)]">
        펀더멘털 데이터를 불러오지 못했습니다.
      </Card>
    );
  }

  if (data.coverageTier === "not_applicable") {
    return (
      <Card className="border-dashed border-[var(--border)] p-8 text-center">
        <Badge variant="info">{COVERAGE_TIER_LABEL.not_applicable}</Badge>
        <p className="mt-4 text-sm leading-relaxed text-[var(--text-secondary)]">
          {data.message ??
            `${data.productLabel ?? "ETF"} 상품은 개별 재무제표를 제공하지 않습니다.`}
        </p>
        <p className="mt-2 text-xs text-[var(--text-muted)]">
          {stock.name}({stock.symbol}) · {stock.securityType ?? data.assetType ?? "ETF"}
        </p>
      </Card>
    );
  }

  if (data.coverageTier === "unavailable") {
    return (
      <Card className="p-8 text-center">
        <Badge variant="warning">{COVERAGE_TIER_LABEL.unavailable}</Badge>
        <p className="mt-4 text-sm text-[var(--text-secondary)]">
          공시 재무 데이터가 아직 없습니다. DART/EDGAR 원천 데이터 적재 후 다시 확인해 주세요.
        </p>
      </Card>
    );
  }

  const panel = mapFundamentalsToPanel(data, stock);
  if (!panel) {
    return null;
  }

  return <FundamentalPanel data={panel} catalog={stock} />;
}

function FundamentalPanel({
  data,
  catalog,
}: {
  data: StockFundamentalsPanel;
  catalog: StockInfo;
}) {
  const exchangeRate = useExchangeRate(data.currency === "USD");
  const scoreTone =
    data.score >= 75 ? "text-emerald-400" : data.score >= 55 ? "text-violet-400" : "text-amber-400";
  const tierVariant =
    data.coverageTier === "full"
      ? "profit"
      : data.coverageTier === "basic"
        ? "warning"
        : "info";

  return (
    <Card className="p-0">
      <div className="flex flex-wrap items-start justify-between gap-4 border-b border-[var(--border)] px-5 py-4">
        <div>
          <p className="text-xs text-[var(--text-muted)]">펀더멘털 분석</p>
          <h3 className="mt-1 text-lg font-semibold text-[var(--text-primary)]">{data.name}</h3>
          <p className="text-xs text-[var(--text-muted)]">
            {data.symbol} · {catalog.market} · {catalog.currency}
            {data.fiscalYear ? ` · FY${data.fiscalYear}` : ""}
            {data.price != null
              ? ` · ${formatPrice(data.price, data.currency, data.currency === "USD" ? { exchangeRate } : undefined)}`
              : ""}
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
          {data.dataSource ? (
            <span className="text-[10px] uppercase text-[var(--text-muted)]">
              {data.dataSource} · {data.calcStatus}
            </span>
          ) : null}
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
  );
}

function MetricCard({ metric }: { metric: StockFundamentalsPanel["metrics"][number] }) {
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
