"use client";

import { useEffect, useMemo, useState, type ReactNode } from "react";
import {
  Bar,
  BarChart,
  CartesianGrid,
  ComposedChart,
  Legend,
  Line,
  LineChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";
import { Card } from "@/components/ui/card";
import { FinancialDetailModal, type FinancialDetailPreset } from "@/components/analysis/financial-detail-modal";
import { buildTrendSeries } from "@/lib/analysis-chart-utils";
import { fetchFinancialHistoryClient } from "@/lib/financial-history-api";
import type { FinancialHistoryPeriod } from "@/lib/financial-history-types";
import { formatFinancialAmount, formatFinancialTick } from "@/lib/format";
import type { FundamentalsData } from "@/lib/fundamentals-types";
import type { QuoteCurrency, StockInfo } from "@/lib/quotes-types";

type FinancialsChartsTabProps = {
  stock: StockInfo;
  fundamentals: FundamentalsData | null;
  loading: boolean;
};

function periodLabel(p: FinancialHistoryPeriod): string {
  if (!p.periodEnd) return `${p.fiscalYear} Q${p.periodIndex}`;
  const d = new Date(p.periodEnd);
  if (Number.isNaN(d.getTime())) return `${p.fiscalYear} Q${p.periodIndex}`;
  return `${String(d.getFullYear()).slice(2)}년 ${d.getMonth() + 1}월`;
}

export function FinancialsChartsTab({ stock, fundamentals, loading }: FinancialsChartsTabProps) {
  const [history, setHistory] = useState<FinancialHistoryPeriod[] | null>(null);
  const [historyLoading, setHistoryLoading] = useState(true);
  const [modalPreset, setModalPreset] = useState<FinancialDetailPreset | null>(null);

  useEffect(() => {
    let cancelled = false;
    setHistoryLoading(true);
    fetchFinancialHistoryClient(stock.symbol, 3)
      .then((res) => {
        if (!cancelled) setHistory(res.periods);
      })
      .catch(() => {
        if (!cancelled) setHistory(null);
      })
      .finally(() => {
        if (!cancelled) setHistoryLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [stock.symbol]);

  const periods = useMemo(() => history ?? [], [history]);
  const charts = useMemo(() => buildFinancialCharts(stock.symbol, fundamentals, periods), [stock.symbol, fundamentals, periods]);

  if (loading || historyLoading) {
    return (
      <Card className="flex min-h-[200px] items-center justify-center p-6 text-xs text-[var(--text-muted)]">
        재무제표 차트 로딩 중…
      </Card>
    );
  }

  if (!fundamentals || fundamentals.coverageTier === "unavailable") {
    return (
      <Card className="flex min-h-[160px] items-center justify-center p-6 text-xs text-[var(--text-muted)]">
        재무제표 차트를 표시할 공시 데이터가 없습니다.
      </Card>
    );
  }

  const hasHistory = periods.length > 0;
  // Statement lines are scaled to the listing's own currency: 억/조원 for a
  // domestic filing, M/B for a US one. Without this the raw figure (e.g.
  // 240000000) reached the tooltip unformatted.
  const currency: QuoteCurrency = stock.currency === "USD" ? "USD" : "KRW";
  const amountTooltip = (v: unknown, name: unknown) =>
    [formatFinancialAmount(Number(v), currency), String(name ?? "")] as [string, string];

  return (
    <div className="grid gap-2.5 sm:grid-cols-2">
      <ChartCard
        title="매출 · 순이익"
        subtitle={hasHistory ? "분기별 실적 (클릭 시 상세)" : "연간 추이 (추정 포함)"}
        onClick={hasHistory ? () => setModalPreset("profitability") : undefined}
      >
        <ResponsiveContainer width="100%" height={140}>
          <BarChart data={charts.revenueProfit}>
            <CartesianGrid strokeDasharray="3 3" stroke="var(--border)" vertical={false} />
            <XAxis dataKey="year" tick={{ fontSize: 10, fill: "var(--text-muted)" }} />
            <YAxis hide />
            <Tooltip contentStyle={tooltipStyle} formatter={amountTooltip} />
            <Legend wrapperStyle={{ fontSize: 10 }} />
            <Bar dataKey="revenue" name="매출" fill="#3B82F6" radius={[2, 2, 0, 0]} />
            <Bar dataKey="netIncome" name="순이익" fill="#10B981" radius={[2, 2, 0, 0]} />
          </BarChart>
        </ResponsiveContainer>
      </ChartCard>

      <ChartCard title="이익률" subtitle={hasHistory ? "영업 · 순이익률 (분기)" : "영업 · 순이익 · ROE"}>
        <ResponsiveContainer width="100%" height={140}>
          <LineChart data={charts.margins}>
            <CartesianGrid strokeDasharray="3 3" stroke="var(--border)" vertical={false} />
            <XAxis dataKey="year" tick={{ fontSize: 10, fill: "var(--text-muted)" }} />
            <YAxis tick={{ fontSize: 10, fill: "var(--text-muted)" }} unit="%" />
            <Tooltip contentStyle={tooltipStyle} formatter={(v) => [`${Number(v).toFixed(1)}%`, ""]} />
            <Legend wrapperStyle={{ fontSize: 10 }} />
            <Line type="monotone" dataKey="opMargin" name="영업마진" stroke="#3B82F6" strokeWidth={2} dot={false} />
            <Line type="monotone" dataKey="netMargin" name="순이익률" stroke="#10B981" strokeWidth={2} dot={false} />
          </LineChart>
        </ResponsiveContainer>
      </ChartCard>

      <ChartCard
        title="성장률"
        subtitle={hasHistory ? "매출 · 영업이익 (직전 분기 대비)" : "매출 · EPS (YoY)"}
        onClick={hasHistory ? () => setModalPreset("growth") : undefined}
      >
        <ResponsiveContainer width="100%" height={140}>
          <LineChart data={charts.growth}>
            <CartesianGrid strokeDasharray="3 3" stroke="var(--border)" vertical={false} />
            <XAxis dataKey="year" tick={{ fontSize: 10, fill: "var(--text-muted)" }} />
            <YAxis tick={{ fontSize: 10, fill: "var(--text-muted)" }} unit="%" />
            <Tooltip contentStyle={tooltipStyle} formatter={(v) => [`${Number(v).toFixed(1)}%`, ""]} />
            <Legend wrapperStyle={{ fontSize: 10 }} />
            <Line type="monotone" dataKey="revenue" name="매출" stroke="#8B5CF6" strokeWidth={2} dot />
            <Line type="monotone" dataKey="secondary" name={hasHistory ? "영업이익" : "EPS"} stroke="#14B8A6" strokeWidth={2} dot />
          </LineChart>
        </ResponsiveContainer>
      </ChartCard>

      <ChartCard title="PER · PBR" subtitle="밸류에이션 추이">
        <ResponsiveContainer width="100%" height={140}>
          <LineChart data={charts.valuation}>
            <CartesianGrid strokeDasharray="3 3" stroke="var(--border)" vertical={false} />
            <XAxis dataKey="year" tick={{ fontSize: 10, fill: "var(--text-muted)" }} />
            <YAxis tick={{ fontSize: 10, fill: "var(--text-muted)" }} />
            <Tooltip
              contentStyle={tooltipStyle}
              formatter={(v, name) => [`${Number(v).toFixed(1)}배`, name]}
            />
            <Legend wrapperStyle={{ fontSize: 10 }} />
            <Line type="monotone" dataKey="per" name="PER" stroke="#EF4444" strokeWidth={2} dot={false} />
            <Line type="monotone" dataKey="pbr" name="PBR" stroke="#3B82F6" strokeWidth={2} dot={false} />
          </LineChart>
        </ResponsiveContainer>
      </ChartCard>

      <ChartCard
        title="재무 건전성"
        subtitle={hasHistory ? "총자본 · 총부채 · 부채비율" : "부채 · 유동"}
        onClick={hasHistory ? () => setModalPreset("stability") : undefined}
      >
        <ResponsiveContainer width="100%" height={140}>
          <ComposedChart data={charts.health}>
            <CartesianGrid strokeDasharray="3 3" stroke="var(--border)" vertical={false} />
            <XAxis dataKey="label" tick={{ fontSize: 10, fill: "var(--text-muted)" }} />
            <YAxis yAxisId="left" hide />
            <YAxis yAxisId="right" orientation="right" tick={{ fontSize: 10, fill: "var(--text-muted)" }} unit="%" />
            {/* Mixed units on one chart: the bars are amounts, the line is a
                ratio, so the tooltip has to branch per series. */}
            <Tooltip
              contentStyle={tooltipStyle}
              formatter={(v, name) => [
                name === "부채비율"
                  ? `${Number(v).toFixed(1)}%`
                  : formatFinancialAmount(Number(v), currency),
                name,
              ]}
            />
            <Bar yAxisId="left" dataKey="equity" name="총자본" fill="#14B8A6" radius={[2, 2, 0, 0]} />
            <Bar yAxisId="left" dataKey="liabilities" name="총부채" fill="#0F766E" radius={[2, 2, 0, 0]} />
            <Line yAxisId="right" type="monotone" dataKey="debtRatio" name="부채비율" stroke="#F59E0B" strokeWidth={2} dot={false} />
          </ComposedChart>
        </ResponsiveContainer>
      </ChartCard>

      <ChartCard title="EPS 추이" subtitle="주당순이익">
        <ResponsiveContainer width="100%" height={140}>
          <AreaMiniChart data={charts.epsSeries} color="#10B981" currency={currency} />
        </ResponsiveContainer>
      </ChartCard>

      <FinancialDetailModal
        open={modalPreset != null}
        onClose={() => setModalPreset(null)}
        symbol={stock.symbol}
        currency={stock.currency}
        preset={modalPreset ?? "profitability"}
      />
    </div>
  );
}

const tooltipStyle = {
  background: "var(--surface)",
  border: "1px solid var(--border)",
  borderRadius: 8,
  fontSize: 11,
};

function ChartCard({
  title,
  subtitle,
  children,
  onClick,
}: {
  title: string;
  subtitle: string;
  children: ReactNode;
  onClick?: () => void;
}) {
  return (
    <Card
      className={onClick ? "cursor-pointer p-3 transition-colors hover:border-[var(--brand)]" : "p-3"}
      onClick={onClick}
      role={onClick ? "button" : undefined}
      tabIndex={onClick ? 0 : undefined}
      onKeyDown={
        onClick
          ? (e) => {
              if (e.key === "Enter" || e.key === " ") {
                e.preventDefault();
                onClick();
              }
            }
          : undefined
      }
    >
      <div className="mb-2">
        <h3 className="text-xs font-semibold text-[var(--text-primary)]">{title}</h3>
        <p className="text-[10px] text-[var(--text-muted)]">{subtitle}</p>
      </div>
      {children}
    </Card>
  );
}

function AreaMiniChart({
  data,
  color,
  currency,
}: {
  data: { year: string; value: number }[];
  color: string;
  currency: QuoteCurrency;
}) {
  // EPS is a per-share amount, so it stays in plain currency units rather than
  // the 억/조 scaling used for statement totals.
  const perShare = (v: unknown) =>
    currency === "USD"
      ? `$${Number(v).toFixed(2)}`
      : `${Number(v).toLocaleString("ko-KR", { maximumFractionDigits: 0 })}원`;

  return (
    <ResponsiveContainer width="100%" height="100%">
      <LineChart data={data}>
        <CartesianGrid strokeDasharray="3 3" stroke="var(--border)" vertical={false} />
        <XAxis dataKey="year" tick={{ fontSize: 10, fill: "var(--text-muted)" }} />
        <YAxis
          tick={{ fontSize: 10, fill: "var(--text-muted)" }}
          tickFormatter={(v) => formatFinancialTick(Number(v), currency)}
          width={44}
        />
        <Tooltip contentStyle={tooltipStyle} formatter={(v) => [perShare(v), "EPS"]} />
        <Line type="monotone" dataKey="value" stroke={color} strokeWidth={2} dot />
      </LineChart>
    </ResponsiveContainer>
  );
}

/**
 * Uses real multi-quarter history when available (`periods`, from
 * /financials/history). PER·PBR has no historical source (would need
 * historical price×EPS pairing) and keeps the single-point trend-series
 * estimate; everything else falls back to that same estimate only when
 * history failed to load (e.g. neither DART nor EDGAR configured).
 */
function buildFinancialCharts(symbol: string, data: FundamentalsData | null, periods: FinancialHistoryPeriod[]) {
  const fy = data?.fiscalYear ?? new Date().getFullYear();
  const years = [fy - 4, fy - 3, fy - 2, fy - 1, fy].map(String);

  const per = data?.per ?? 0;
  const pbr = data?.pbr ?? 0;
  const perSeries = buildTrendSeries(per, `${symbol}-per-fin`, 5);
  const pbrSeries = buildTrendSeries(pbr, `${symbol}-pbr-fin`, 5);
  const valuation = perSeries.map((p, i) => ({
    year: p.label,
    per: p.value,
    pbr: pbrSeries[i]?.value ?? 0,
  }));

  if (periods.length > 0) {
    const revenueProfit = periods.map((p) => ({
      year: periodLabel(p),
      revenue: p.revenue ?? 0,
      netIncome: p.netIncome ?? 0,
    }));
    const margins = periods.map((p) => ({
      year: periodLabel(p),
      opMargin: p.operatingMargin ?? 0,
      netMargin: p.netProfitMargin ?? 0,
    }));
    const growth = periods.map((p) => ({
      year: periodLabel(p),
      revenue: p.revenueGrowthQoq ?? 0,
      secondary: p.operatingIncomeGrowthQoq ?? 0,
    }));
    const health = periods.map((p) => ({
      label: periodLabel(p),
      equity: p.totalEquity ?? 0,
      liabilities: p.totalLiabilities ?? 0,
      debtRatio: p.debtRatio ?? 0,
    }));
    const epsSeries = periods.map((p) => ({ year: periodLabel(p), value: p.epsBasic ?? 0 }));
    return { revenueProfit, margins, growth, valuation, health, epsSeries };
  }

  // Fallback estimate — only reached when /financials/history returned nothing.
  const rev = data?.revenue ?? 0;
  const ni = data?.netIncome ?? 0;
  const revGrowth = data?.revenueGrowthYoy ?? 0;
  const epsGrowth = data?.epsGrowthYoy ?? 0;
  const opm = data?.operatingMargin ?? 0;
  const npm = data?.netProfitMargin ?? 0;

  const revenueProfit = years.map((year, i) => {
    const factor = 0.72 + i * 0.07;
    return {
      year,
      revenue: rev > 0 ? Math.round(rev * factor) : 0,
      netIncome: ni > 0 ? Math.round(ni * factor * 0.9) : 0,
    };
  });
  const margins = years.map((year, i) => ({
    year,
    opMargin: Math.max(0, opm * (0.85 + i * 0.04)),
    netMargin: Math.max(0, npm * (0.85 + i * 0.04)),
  }));
  const growth = years.map((year, i) => ({
    year,
    revenue: revGrowth * (0.6 + i * 0.1),
    secondary: epsGrowth * (0.6 + i * 0.1),
  }));
  const health = [{ label: "현재", equity: 0, liabilities: 0, debtRatio: data?.debtRatio ?? 0 }];
  const epsBase = per > 0 && data?.price ? data.price / per : 0;
  const epsSeries = buildTrendSeries(epsBase, `${symbol}-eps`, 5).map((p) => ({ year: p.label, value: p.value }));

  return { revenueProfit, margins, growth, valuation, health, epsSeries };
}
