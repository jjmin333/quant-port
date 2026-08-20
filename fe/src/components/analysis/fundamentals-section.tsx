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
import type { StockInfo } from "@/lib/quotes-types";
import { cn } from "@/lib/utils";

type FundamentalsSectionProps = {
  stock: StockInfo;
  data: FundamentalsData | null;
  loading: boolean;
};

/** @deprecated Use tab panels — kept for financials redirect page */
export function FundamentalsSection({ stock, data, loading }: FundamentalsSectionProps) {
  return (
    <FundamentalsIndicatorsPanel
      stock={stock}
      data={data}
      loading={loading}
      panel={data ? mapFundamentalsToPanel(data, stock) : null}
    />
  );
}

type IndicatorsPanelProps = {
  stock: StockInfo;
  data: FundamentalsData | null;
  loading?: boolean;
  panel: StockFundamentalsPanel | null;
};

export function FundamentalsIndicatorsPanel({
  stock,
  data,
  loading = false,
  panel: panelProp,
}: IndicatorsPanelProps) {
  if (loading) {
    return (
      <Card className="flex min-h-[160px] items-center justify-center p-6 text-xs text-[var(--text-muted)]">
        지표 데이터 로딩 중…
      </Card>
    );
  }

  if (!data) {
    return (
      <Card className="flex min-h-[160px] items-center justify-center p-6 text-xs text-[var(--text-muted)]">
        지표 데이터를 불러오지 못했습니다.
      </Card>
    );
  }

  if (data.coverageTier === "not_applicable") {
    return (
      <Card className="border-dashed p-5 text-center">
        <Badge variant="info">{COVERAGE_TIER_LABEL.not_applicable}</Badge>
        <p className="mt-3 text-xs text-[var(--text-secondary)]">
          {data.message ?? "재무제표 미제공 상품입니다."}
        </p>
      </Card>
    );
  }

  if (data.coverageTier === "unavailable") {
    return (
      <Card className="p-5 text-center">
        <Badge variant="warning">{COVERAGE_TIER_LABEL.unavailable}</Badge>
        <p className="mt-3 text-xs text-[var(--text-secondary)]">공시 재무 데이터가 없습니다.</p>
      </Card>
    );
  }

  const panel = panelProp ?? mapFundamentalsToPanel(data, stock);
  if (!panel) return null;

  const scoreTone =
    panel.score >= 75 ? "text-emerald-400" : panel.score >= 55 ? "text-violet-400" : "text-amber-400";
  const tierVariant =
    panel.coverageTier === "full" ? "profit" : panel.coverageTier === "basic" ? "warning" : "info";

  return (
    <Card className="p-0">
      <div className="flex items-center justify-between gap-3 border-b border-[var(--border)] px-3 py-2.5">
        <div className="min-w-0">
          <p className="text-[10px] text-[var(--text-muted)]">종합 지표</p>
          <p className="line-clamp-1 text-xs text-[var(--text-secondary)]">{panel.summary}</p>
        </div>
        <div className="flex shrink-0 items-center gap-2">
          <ScoreRing score={panel.score} />
          <div className="text-right">
            <p className={cn("text-lg font-bold tabular-nums leading-none", scoreTone)}>
              {panel.score > 0 ? `+${panel.score}` : "—"}
            </p>
            <p className={cn("text-[11px] font-semibold", scoreTone)}>{panel.rating}</p>
          </div>
        </div>
      </div>

      <div className="border-b border-[var(--border)] bg-blue-500/5 px-3 py-2">
        <div className="flex flex-wrap items-center gap-1.5">
          <Badge variant="info" className="text-[10px]">
            핵심 요약
          </Badge>
          <Badge variant={tierVariant} className="text-[10px]">
            {COVERAGE_TIER_LABEL[panel.coverageTier]}
          </Badge>
        </div>
        <div className="mt-1.5 flex flex-wrap gap-1">
          {panel.tags.map((tag) => (
            <span
              key={tag}
              className="rounded-full bg-[var(--surface-elevated)] px-2 py-0.5 text-[10px] text-[var(--text-muted)]"
            >
              {tag}
            </span>
          ))}
        </div>
      </div>

      <div className="grid gap-2 p-3 sm:grid-cols-2">
        {panel.metrics.map((metric) => (
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
        "rounded-lg border border-[var(--border)] bg-[var(--surface-elevated)]/40 p-2.5",
        !metric.available && "opacity-70",
      )}
    >
      <div className="flex items-start justify-between gap-2">
        <span className="text-[11px] text-[var(--text-secondary)]">{metric.label}</span>
        <span className={cn("text-[10px] font-medium", metricToneClass(metric.tone))}>
          {metric.verdict}
        </span>
      </div>
      <p className="mt-1 font-mono text-lg font-semibold tabular-nums text-[var(--text-primary)]">
        {metric.value}
      </p>
      {metric.available && (
        <div className="mt-2 h-1 overflow-hidden rounded-full bg-[var(--surface-elevated)]">
          <div
            className={cn("h-full rounded-full", metricGaugeClass(metric.tone))}
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
    <svg viewBox="0 0 36 36" className="h-9 w-9 -rotate-90" aria-hidden>
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
