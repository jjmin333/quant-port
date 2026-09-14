"use client";

import { useEffect, useState } from "react";
import {
  Bar,
  CartesianGrid,
  ComposedChart,
  Legend,
  Line,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";
import { Loader2, X } from "lucide-react";
import { fetchFinancialHistoryClient } from "@/lib/financial-history-api";
import type { FinancialHistoryPeriod } from "@/lib/financial-history-types";
import { formatFinancialAmount, formatPercent } from "@/lib/format";
import {
  PERIOD_AXIS_HEIGHT,
  periodTickInterval,
  periodTickRenderer,
} from "@/components/analysis/period-axis-tick";
import { buildPeriodTicks, periodTickIndex } from "@/lib/financial-period-label";
import type { QuoteCurrency } from "@/lib/quotes-types";
import { cn } from "@/lib/utils";
import { toUserMessage } from "@/lib/error-message";
import { buttonVariants } from "@/components/ui/button";

export type FinancialDetailPreset = "profitability" | "growth" | "stability";

type FinancialDetailModalProps = {
  open: boolean;
  onClose: () => void;
  symbol: string;
  currency: QuoteCurrency;
  preset: FinancialDetailPreset;
};

type PresetConfig = {
  title: string;
  subtitle: string;
  primaryLabel: string;
  primaryKey: "revenue" | "operatingIncome" | "totalEquity";
  primaryColor: string;
  secondaryLabel?: string;
  secondaryKey?: "netIncome" | "totalLiabilities";
  secondaryColor?: string;
  lineLabel: string;
  lineKey: "netProfitMargin" | "operatingMargin" | "debtRatio";
  lineColor: string;
  headlineLabel: string;
  headlineKey: "netIncome" | "operatingIncome" | "debtRatio";
  headlineIsRatio: boolean;
};

const PRESETS: Record<FinancialDetailPreset, PresetConfig> = {
  profitability: {
    title: "수익성",
    subtitle: "매출·순이익 성장률",
    primaryLabel: "매출",
    primaryKey: "revenue",
    primaryColor: "#3B82F6",
    secondaryLabel: "순이익",
    secondaryKey: "netIncome",
    secondaryColor: "#1E3A8A",
    lineLabel: "순이익률",
    lineKey: "netProfitMargin",
    lineColor: "#F59E0B",
    headlineLabel: "순이익",
    headlineKey: "netIncome",
    headlineIsRatio: false,
  },
  growth: {
    title: "성장성",
    subtitle: "영업이익 성장률",
    primaryLabel: "영업이익",
    primaryKey: "operatingIncome",
    primaryColor: "#A855F7",
    lineLabel: "영업이익률",
    lineKey: "operatingMargin",
    lineColor: "#10B981",
    headlineLabel: "영업이익",
    headlineKey: "operatingIncome",
    headlineIsRatio: false,
  },
  stability: {
    title: "안정성",
    subtitle: "총자본·총부채·부채비율",
    primaryLabel: "총자본",
    primaryKey: "totalEquity",
    primaryColor: "#14B8A6",
    secondaryLabel: "총부채",
    secondaryKey: "totalLiabilities",
    secondaryColor: "#0F766E",
    lineLabel: "부채비율",
    lineKey: "debtRatio",
    lineColor: "#F59E0B",
    headlineLabel: "부채비율",
    headlineKey: "debtRatio",
    headlineIsRatio: true,
  },
};

const YEAR_OPTIONS = [1, 3, 5] as const;

function formatPeriodLabel(p: FinancialHistoryPeriod): string {
  if (!p.periodEnd) return `${p.fiscalYear} Q${p.periodIndex}`;
  const d = new Date(p.periodEnd);
  if (Number.isNaN(d.getTime())) return `${p.fiscalYear} Q${p.periodIndex}`;
  return `${String(d.getFullYear()).slice(2)}년 ${d.getMonth() + 1}월`;
}

function buildHeadline(
  periods: FinancialHistoryPeriod[],
  config: PresetConfig,
  currency: QuoteCurrency,
): { text: string; positive: boolean } | null {
  if (periods.length === 0) return null;
  const cur = periods[periods.length - 1];
  const curVal = cur[config.headlineKey];
  if (curVal == null) return null;

  const periodLabel = `${cur.fiscalYear}년 ${cur.periodIndex}분기`;
  const valueLabel = config.headlineIsRatio
    ? `${curVal.toFixed(2)}%`
    : formatFinancialAmount(Math.abs(curVal), currency);

  const prev = periods.length > 1 ? periods[periods.length - 2] : null;
  const prevVal = prev ? prev[config.headlineKey] : null;
  if (prevVal == null || prevVal === 0) {
    return { text: `${periodLabel} ${config.headlineLabel}은 ${valueLabel}이에요.`, positive: curVal >= 0 };
  }

  const diff = config.headlineIsRatio
    ? curVal - prevVal
    : ((curVal - prevVal) / Math.abs(prevVal)) * 100;
  const diffLabel = config.headlineIsRatio ? `${Math.abs(diff).toFixed(2)}%p` : `${Math.abs(diff).toFixed(2)}%`;
  const verb = diff >= 0 ? "높아요" : "낮아요";
  return {
    text: `${periodLabel} ${config.headlineLabel}은 ${valueLabel}으로 직전 분기 대비 ${diffLabel} ${verb}.`,
    positive: diff >= 0,
  };
}

export function FinancialDetailModal({ open, onClose, symbol, currency, preset }: FinancialDetailModalProps) {
  const [years, setYears] = useState<number>(3);
  const [data, setData] = useState<FinancialHistoryPeriod[] | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!open) return;
    let cancelled = false;
    setLoading(true);
    setError(null);
    fetchFinancialHistoryClient(symbol, years)
      .then((res) => {
        if (!cancelled) setData(res.periods);
      })
      .catch((e) => {
        if (!cancelled) setError(toUserMessage(e, "재무 히스토리 조회 실패"));
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [open, symbol, years]);

  useEffect(() => {
    if (!open) return;
    const prevOverflow = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    const onKeyDown = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    window.addEventListener("keydown", onKeyDown);
    return () => {
      document.body.style.overflow = prevOverflow;
      window.removeEventListener("keydown", onKeyDown);
    };
  }, [open, onClose]);

  if (!open) return null;

  const config = PRESETS[preset];
  const periods = data ?? [];
  const headline = buildHeadline(periods, config, currency);
  const ticks = buildPeriodTicks(periods);
  const periodTick = periodTickRenderer(periodTickIndex(ticks));
  const chartData = periods.map((p, i) => ({
    label: ticks[i]?.full ?? formatPeriodLabel(p),
    primary: p[config.primaryKey] ?? null,
    secondary: config.secondaryKey ? p[config.secondaryKey] ?? null : null,
    line: p[config.lineKey] ?? null,
  }));

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-0 sm:p-4">
      <button type="button" aria-label="닫기" className="absolute inset-0 bg-black/50" onClick={onClose} />

      <div
        role="dialog"
        aria-modal="true"
        aria-label={`${config.title} 상세`}
        className="relative flex h-full w-full max-w-[720px] flex-col overflow-hidden bg-[var(--surface)] shadow-2xl sm:h-auto sm:max-h-[calc(100%-2rem)] sm:rounded-xl sm:border sm:border-[var(--border)]"
      >
        <header className="flex shrink-0 items-start justify-between gap-3 border-b border-[var(--border)] px-4 py-3">
          <div className="min-w-0">
            <h2 className="text-base font-semibold text-[var(--text-primary)]">{config.title}</h2>
            <p className="text-cell text-[var(--text-muted)]">{config.subtitle}</p>
          </div>
          <button
            type="button"
            aria-label="닫기"
            onClick={onClose}
            className="flex h-8 w-8 shrink-0 items-center justify-center rounded-md border border-[var(--border)] text-[var(--text-secondary)] hover:bg-[var(--surface-elevated)]"
          >
            <X className="h-4 w-4" />
          </button>
        </header>

        <div className="min-h-0 flex-1 overflow-y-auto px-4 py-3">
          {headline && (
            <p className="mb-3 text-sm leading-relaxed text-[var(--text-primary)]">
              {headline.text.split(/(\d[\d.]*%p?)/).map((chunk, i) =>
                /^\d/.test(chunk) ? (
                  <span
                    key={i}
                    className={headline.positive ? "font-semibold text-[var(--profit)]" : "font-semibold text-[var(--loss)]"}
                  >
                    {chunk}
                  </span>
                ) : (
                  <span key={i}>{chunk}</span>
                ),
              )}
            </p>
          )}

          <div className="mb-3 flex gap-1">
            {YEAR_OPTIONS.map((y) => (
              <button
                key={y}
                type="button"
                onClick={() => setYears(y)}
                className={cn(buttonVariants({ variant: "outline", size: "xs" }),
                  years === y
                    ? "bg-[var(--brand-muted)] text-[var(--brand)]"
                    : "border border-[var(--border)] text-[var(--text-secondary)] hover:bg-[var(--surface-elevated)]",
                )}
              >
                {y}년
              </button>
            ))}
          </div>

          {loading ? (
            <div className="flex h-48 items-center justify-center text-cell text-[var(--text-muted)]">
              <Loader2 className="mr-2 h-4 w-4 animate-spin" />
              불러오는 중…
            </div>
          ) : error ? (
            <p className="py-6 text-center text-cell text-[var(--loss)]">{error}</p>
          ) : periods.length === 0 ? (
            <p className="py-6 text-center text-cell text-[var(--text-muted)]">표시할 데이터가 없습니다.</p>
          ) : (
            <>
              <ResponsiveContainer width="100%" height={220}>
                <ComposedChart data={chartData} margin={{ top: 4, right: 8, left: 8, bottom: 0 }}>
                  <CartesianGrid strokeDasharray="3 3" stroke="var(--border)" vertical={false} />
                  <XAxis dataKey="label" tick={periodTick} interval={periodTickInterval(ticks.length)} height={PERIOD_AXIS_HEIGHT} />
                  <YAxis yAxisId="left" hide />
                  <YAxis yAxisId="right" orientation="right" tick={{ fontSize: 11, fill: "var(--text-muted)" }} unit="%" />
                  <Tooltip
                    contentStyle={{ background: "var(--surface)", border: "1px solid var(--border)", borderRadius: 8, fontSize: 11 }}
                    formatter={(value, name) =>
                      name === config.lineLabel
                        ? [`${Number(value).toFixed(2)}%`, name]
                        : [formatFinancialAmount(Number(value), currency), name]
                    }
                  />
                  <Legend wrapperStyle={{ fontSize: 11 }} />
                  <Bar yAxisId="left" dataKey="primary" name={config.primaryLabel} fill={config.primaryColor} radius={[2, 2, 0, 0]} isAnimationActive={false} />
                  {config.secondaryKey && (
                    <Bar yAxisId="left" dataKey="secondary" name={config.secondaryLabel} fill={config.secondaryColor} radius={[2, 2, 0, 0]} isAnimationActive={false} />
                  )}
                  <Line yAxisId="right" type="monotone" dataKey="line" name={config.lineLabel} stroke={config.lineColor} strokeWidth={2} dot={{ r: 2 }} isAnimationActive={false} />
                </ComposedChart>
              </ResponsiveContainer>

              <div className="mt-3 overflow-x-auto rounded-lg border border-[var(--border)]">
                <table className="w-full min-w-max text-left text-cell">
                  <thead>
                    <tr className="border-b border-[var(--border)] bg-[var(--surface-elevated)] text-[var(--text-muted)]">
                      <th className="sticky left-0 z-10 bg-[var(--surface-elevated)] px-2 py-1.5 font-medium">항목</th>
                      {periods.map((p, i) => (
                        <th key={i} className="whitespace-nowrap px-2 py-1.5 text-right font-medium">
                          {formatPeriodLabel(p)}
                        </th>
                      ))}
                    </tr>
                  </thead>
                  <tbody>
                    <MetricRow label={config.primaryLabel} dotColor={config.primaryColor} periods={periods} pick={(p) => p[config.primaryKey]} currency={currency} />
                    {config.secondaryKey && (
                      <MetricRow label={config.secondaryLabel!} dotColor={config.secondaryColor!} periods={periods} pick={(p) => p[config.secondaryKey!]} currency={currency} />
                    )}
                    <MetricRow label={config.lineLabel} dotColor={config.lineColor} hollow periods={periods} pick={(p) => p[config.lineKey]} percent />
                    <GrowthRow label={`${config.headlineLabel} 성장률`} periods={periods} preset={preset} />
                  </tbody>
                </table>
              </div>
              <p className="mt-2 text-micro text-[var(--text-muted)]">
                성장률은 직전 분기 대비 증감입니다. 국내 종목은 DART, 미국 종목은 SEC EDGAR 공시 기준입니다.
              </p>
            </>
          )}
        </div>
      </div>
    </div>
  );
}

function MetricRow({
  label,
  dotColor,
  hollow,
  periods,
  pick,
  currency,
  percent,
}: {
  label: string;
  dotColor: string;
  hollow?: boolean;
  periods: FinancialHistoryPeriod[];
  pick: (p: FinancialHistoryPeriod) => number | undefined;
  currency?: QuoteCurrency;
  percent?: boolean;
}) {
  return (
    <tr className="border-b border-[var(--border)]/60 last:border-0">
      <td className="sticky left-0 z-10 whitespace-nowrap bg-[var(--surface)] px-2 py-1.5 text-[var(--text-secondary)]">
        <span className="inline-flex items-center gap-1.5">
          <span
            className="inline-block h-2 w-2 rounded-full"
            style={hollow ? { border: `1.5px solid ${dotColor}` } : { background: dotColor }}
          />
          {label}
        </span>
      </td>
      {periods.map((p, i) => {
        const v = pick(p);
        return (
          <td key={i} className="whitespace-nowrap px-2 py-1.5 text-right tabular-nums text-[var(--text-primary)]">
            {v == null ? "—" : percent ? `${v.toFixed(2)}%` : formatFinancialAmount(v, currency ?? "KRW")}
          </td>
        );
      })}
    </tr>
  );
}

function GrowthRow({
  label,
  periods,
  preset,
}: {
  label: string;
  periods: FinancialHistoryPeriod[];
  preset: FinancialDetailPreset;
}) {
  const growthKey: "revenueGrowthQoq" | "operatingIncomeGrowthQoq" | "netIncomeGrowthQoq" | null =
    preset === "growth" ? "operatingIncomeGrowthQoq" : preset === "profitability" ? "netIncomeGrowthQoq" : null;
  if (!growthKey) return null;

  return (
    <tr className="border-b border-[var(--border)]/60 last:border-0">
      <td className="sticky left-0 z-10 whitespace-nowrap bg-[var(--surface)] px-2 py-1.5 text-[var(--text-secondary)]">{label}</td>
      {periods.map((p, i) => {
        const v = p[growthKey];
        return (
          <td
            key={i}
            className={cn(
              "whitespace-nowrap px-2 py-1.5 text-right tabular-nums font-medium",
              v == null ? "text-[var(--text-muted)]" : v >= 0 ? "text-[var(--profit)]" : "text-[var(--loss)]",
            )}
          >
            {v == null ? "—" : formatPercent(v)}
          </td>
        );
      })}
    </tr>
  );
}
