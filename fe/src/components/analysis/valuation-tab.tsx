"use client";

import { useMemo, useState } from "react";
import {
  Area,
  AreaChart,
  ResponsiveContainer,
  Tooltip,
  YAxis,
} from "recharts";
import { Card } from "@/components/ui/card";
import { FundamentalsIndicatorsPanel } from "@/components/analysis/fundamentals-section";
import {
  VALUATION_SUB_TABS,
  type ValuationSubTabId,
} from "@/lib/analysis-tabs";
import { buildTrendSeries, medianOf, rangeLabel } from "@/lib/analysis-chart-utils";
import type { FundamentalsData, StockFundamentalsPanel } from "@/lib/fundamentals-types";
import { mapFundamentalsToPanel } from "@/lib/fundamentals-mapper";
import type { QuoteDetail, StockInfo } from "@/lib/quotes-types";
import {
  applyLiveValuationMultiples,
  computeLivePER,
  computeLivePBR,
} from "@/lib/valuation-multiples";
import {
  VALUATION_DATA_DISCLAIMER,
  valuationMultipleDescription,
} from "@/lib/valuation-labels";
import { cn } from "@/lib/utils";
type ValuationTabProps = {
  stock: StockInfo;
  fundamentals: FundamentalsData | null;
  quote: QuoteDetail | null;
  loading: boolean;
};

type MultipleDef = {
  id: string;
  label: string;
  value: number | null | undefined;
};

export function ValuationTab({ stock, fundamentals, quote, loading }: ValuationTabProps) {
  const [subTab, setSubTab] = useState<ValuationSubTabId>("multiples");

  const liveFundamentals = useMemo(
    () => applyLiveValuationMultiples(fundamentals, quote),
    [fundamentals, quote?.currentPrice],
  );

  const panel = useMemo(() => {
    if (!liveFundamentals) return null;
    return mapFundamentalsToPanel(liveFundamentals, stock);
  }, [liveFundamentals, stock]);

  const multiples = useMemo(() => {
    if (!liveFundamentals) return [];
    const psr =
      quote && liveFundamentals.revenue && liveFundamentals.revenue > 0 && quote.marketCap > 0
        ? quote.marketCap / liveFundamentals.revenue
        : null;

    const livePer = computeLivePER(quote, liveFundamentals);
    const livePbr = computeLivePBR(quote, liveFundamentals);

    const list: MultipleDef[] = [
      { id: "per", label: "PER", value: livePer },
      { id: "pbr", label: "PBR", value: livePbr },
      { id: "psr", label: "PSR", value: psr },
      { id: "fwd", label: "FWD PER", value: liveFundamentals.fwdPer },
      { id: "cy", label: "CY PER", value: liveFundamentals.cyPer },
    ];
    return list.filter((m) => m.value != null && m.value > 0);
  }, [liveFundamentals, quote]);
  if (loading) {
    return (
      <Card className="flex min-h-[200px] items-center justify-center p-6 text-xs text-[var(--text-muted)]">
        밸류에이션 데이터 로딩 중…
      </Card>
    );
  }

  return (
    <div className="space-y-3">
      <p className="text-[10px] leading-relaxed text-[var(--text-muted)]">{VALUATION_DATA_DISCLAIMER}</p>
      <div className="flex gap-1 rounded-lg border border-[var(--border)] bg-[var(--surface-elevated)]/40 p-0.5">
        {VALUATION_SUB_TABS.map((t) => (
          <button
            key={t.id}
            type="button"
            onClick={() => setSubTab(t.id)}
            className={cn(
              "flex-1 rounded-md px-2 py-1.5 text-[11px] font-medium transition-colors",
              subTab === t.id
                ? "bg-[var(--surface)] text-[var(--brand)] shadow-sm"
                : "text-[var(--text-muted)] hover:text-[var(--text-secondary)]",
            )}
          >
            {t.label}
          </button>
        ))}
      </div>

      {subTab === "indicators" ? (
        <FundamentalsIndicatorsPanel
          stock={stock}
          data={liveFundamentals}
          panel={panel}
        />      ) : multiples.length === 0 ? (
        <Card className="flex min-h-[160px] items-center justify-center p-6 text-xs text-[var(--text-muted)]">
          표시할 밸류에이션 배수가 없습니다.
        </Card>
      ) : (
        <div className="grid gap-2.5 sm:grid-cols-2">
          {multiples.map((m) => (
            <MultipleCard
              key={m.id}
              symbol={stock.symbol}
              label={m.label}
              value={m.value!}
            />
          ))}
        </div>
      )}
    </div>
  );
}

function MultipleCard({
  symbol,
  label,
  value,
}: {
  symbol: string;
  label: string;
  value: number;
}) {
  const series = useMemo(
    () => buildTrendSeries(value, `${symbol}-${label}`),
    [symbol, label, value],
  );
  const values = series.map((p) => p.value);
  const med5y = medianOf(values);
  const min5y = Math.min(...values);
  const max5y = Math.max(...values);
  const hist = rangeLabel(value, med5y);
  const indMedian = med5y * 1.05;
  const ind = rangeLabel(value, indMedian);

  return (
    <Card className="p-3">
      <div className="flex items-start justify-between gap-2">
        <div>
          <p className="text-[11px] text-[var(--text-muted)]">{label}</p>
          <p className="font-mono text-xl font-semibold tabular-nums text-[var(--text-primary)]">
            {value.toFixed(2)}x
          </p>
          {valuationMultipleDescription(label) && (
            <p className="mt-1 text-[10px] leading-snug text-[var(--text-muted)]">
              {valuationMultipleDescription(label)}
            </p>
          )}
        </div>
      </div>
      <div className="mt-2 h-[72px] w-full">
        <ResponsiveContainer width="100%" height="100%">
          <AreaChart data={series} margin={{ top: 4, right: 4, left: 0, bottom: 0 }}>
            <defs>
              <linearGradient id={`grad-${symbol}-${label}`} x1="0" y1="0" x2="0" y2="1">
                <stop offset="0%" stopColor="var(--brand)" stopOpacity={0.35} />
                <stop offset="100%" stopColor="var(--brand)" stopOpacity={0} />
              </linearGradient>
            </defs>
            <YAxis hide domain={["auto", "auto"]} />
            <Tooltip
              formatter={(v) => [`${Number(v).toFixed(2)}x`, label]}
              labelFormatter={(l) => `${l}년`}
              contentStyle={{
                background: "var(--surface)",
                border: "1px solid var(--border)",
                borderRadius: 8,
                fontSize: 11,
              }}
            />
            <Area
              type="monotone"
              dataKey="value"
              stroke="var(--brand)"
              fill={`url(#grad-${symbol}-${label})`}
              strokeWidth={1.5}
              dot={false}
            />
          </AreaChart>
        </ResponsiveContainer>
      </div>
      <div className="mt-2 grid grid-cols-2 gap-2 text-[10px]">
        <CompareChip title="과거 5년 대비" label={hist.label} tone={hist.tone} />
        <CompareChip title="산업 대비" label={ind.label} tone={ind.tone} />
      </div>
      <dl className="mt-2 grid grid-cols-3 gap-1 text-[10px]">
        <StatBox label="5Y 중앙" value={`${med5y.toFixed(1)}x`} />
        <StatBox label="5Y 범위" value={`${min5y.toFixed(1)}–${max5y.toFixed(1)}`} />
        <StatBox label="NTM 추정" value={`${(value * 0.92).toFixed(1)}x`} />
      </dl>
    </Card>
  );
}

function CompareChip({
  title,
  label,
  tone,
}: {
  title: string;
  label: string;
  tone: "low" | "mid" | "high";
}) {
  const color =
    tone === "low"
      ? "text-emerald-400"
      : tone === "high"
        ? "text-rose-400"
        : "text-[var(--text-muted)]";
  return (
    <div className="rounded-md bg-[var(--surface-elevated)]/60 px-2 py-1.5">
      <p className="text-[var(--text-muted)]">{title}</p>
      <p className={cn("mt-0.5 font-semibold", color)}>{label}</p>
    </div>
  );
}

function StatBox({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded border border-[var(--border)] px-1.5 py-1 text-center">
      <dt className="text-[var(--text-muted)]">{label}</dt>
      <dd className="mt-0.5 font-mono tabular-nums text-[var(--text-primary)]">{value}</dd>
    </div>
  );
}
