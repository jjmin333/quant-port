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
import {
  PERIOD_AXIS_HEIGHT,
  periodTickInterval,
  periodTickRenderer,
} from "@/components/analysis/period-axis-tick";
import { fetchFinancialHistoryClient } from "@/lib/financial-history-api";
import { buildPeriodTicks, periodTickIndex } from "@/lib/financial-period-label";
import type { FinancialHistoryPeriod } from "@/lib/financial-history-types";
import { formatFinancialAmount, formatFinancialTick } from "@/lib/format";
import type { FundamentalsData } from "@/lib/fundamentals-types";
import type { QuoteCurrency, StockInfo } from "@/lib/quotes-types";

type FinancialsChartsTabProps = {
  stock: StockInfo;
  fundamentals: FundamentalsData | null;
  loading: boolean;
};

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
  const charts = useMemo(() => buildFinancialCharts(periods), [periods]);

  if (loading || historyLoading) {
    return (
      <Card className="flex min-h-[200px] items-center justify-center p-6 text-cell text-[var(--text-muted)]">
        재무제표 차트 로딩 중…
      </Card>
    );
  }

  /*
   * 분기 이력이 없으면 **차트를 그리지 않는다.**
   *
   * 예전에는 현재값에서 5년치를 지어내 「연간 추이 (추정 포함)」이라 적었다.
   * 실측 2,516종목(국내 1,308 · 미국 1,208)이 그 지어낸 그림을 보고 있었다.
   * 빈 화면은 「모른다」지만 지어낸 곡선은 「이랬다」고 말한다 — 후자가 더 나쁘다.
   */
  if (periods.length === 0) {
    const notCovered = !fundamentals || fundamentals.coverageTier === "unavailable";
    return (
      <Card className="flex min-h-[160px] flex-col items-center justify-center gap-1 p-6 text-center">
        <p className="text-cell text-[var(--text-muted)]">
          {notCovered
            ? "재무제표 차트를 표시할 공시 데이터가 없습니다."
            : "분기별 재무 데이터가 아직 수집되지 않았습니다."}
        </p>
        {!notCovered && (
          <p className="text-micro leading-relaxed text-[var(--text-muted)]">
            배치가 순차로 수집합니다. 관심 종목에 추가하면 다음 실행에서 먼저 가져옵니다.
          </p>
        )}
      </Card>
    );
  }
  // Statement lines are scaled to the listing's own currency: 억/조원 for a
  // domestic filing, M/B for a US one. Without this the raw figure (e.g.
  // 240000000) reached the tooltip unformatted.
  const currency: QuoteCurrency = stock.currency === "USD" ? "USD" : "KRW";
  const amountTooltip = (v: unknown, name: unknown) =>
    [formatFinancialAmount(Number(v), currency), String(name ?? "")] as [string, string];
  const periodTick = periodTickRenderer(charts.tickIndex);

  return (
    <div className="grid gap-2.5 sm:grid-cols-2">
      <ChartCard
        title="매출 · 순이익"
        subtitle="분기별 실적 (클릭 시 상세)"
        onClick={() => setModalPreset("profitability")}
      >
        <ResponsiveContainer width="100%" height={140}>
          <BarChart data={charts.revenueProfit}>
            <CartesianGrid strokeDasharray="3 3" stroke="var(--border)" vertical={false} />
            <XAxis dataKey="year" tick={periodTick} interval={periodTickInterval(periods.length)} height={PERIOD_AXIS_HEIGHT} />
            <YAxis hide />
            <Tooltip contentStyle={tooltipStyle} formatter={amountTooltip} />
            <Legend wrapperStyle={{ fontSize: 11 }} />
            <Bar dataKey="revenue" name="매출" fill="#3B82F6" radius={[2, 2, 0, 0]} isAnimationActive={false} />
            <Bar dataKey="netIncome" name="순이익" fill="#10B981" radius={[2, 2, 0, 0]} isAnimationActive={false} />
          </BarChart>
        </ResponsiveContainer>
      </ChartCard>

      <ChartCard title="이익률" subtitle="영업 · 순이익률 (분기)">
        <ResponsiveContainer width="100%" height={140}>
          <LineChart data={charts.margins}>
            <CartesianGrid strokeDasharray="3 3" stroke="var(--border)" vertical={false} />
            <XAxis dataKey="year" tick={periodTick} interval={periodTickInterval(periods.length)} height={PERIOD_AXIS_HEIGHT} />
            <YAxis tick={{ fontSize: 11, fill: "var(--text-muted)" }} unit="%" />
            <Tooltip contentStyle={tooltipStyle} formatter={(v) => [`${Number(v).toFixed(1)}%`, ""]} />
            <Legend wrapperStyle={{ fontSize: 11 }} />
            {/* 값이 빈 분기는 선을 잇지 않는다. 점을 켜 두는 이유는 그 옆이 모두
                비어 홀로 남은 분기 — 선분이 생기지 않아 점이 없으면 안 보인다. */}
            <Line type="monotone" dataKey="opMargin" name="영업마진" stroke="#3B82F6" strokeWidth={2} dot={{ r: 2 }} isAnimationActive={false} />
            <Line type="monotone" dataKey="netMargin" name="순이익률" stroke="#10B981" strokeWidth={2} dot={{ r: 2 }} isAnimationActive={false} />
          </LineChart>
        </ResponsiveContainer>
      </ChartCard>

      <ChartCard
        title="성장률"
        subtitle="매출 · 영업이익 (직전 분기 대비)"
        onClick={() => setModalPreset("growth")}
      >
        <ResponsiveContainer width="100%" height={140}>
          <LineChart data={charts.growth}>
            <CartesianGrid strokeDasharray="3 3" stroke="var(--border)" vertical={false} />
            <XAxis dataKey="year" tick={periodTick} interval={periodTickInterval(periods.length)} height={PERIOD_AXIS_HEIGHT} />
            <YAxis tick={{ fontSize: 11, fill: "var(--text-muted)" }} unit="%" />
            <Tooltip contentStyle={tooltipStyle} formatter={(v) => [`${Number(v).toFixed(1)}%`, ""]} />
            <Legend wrapperStyle={{ fontSize: 11 }} />
            <Line type="monotone" dataKey="revenue" name="매출" stroke="#8B5CF6" strokeWidth={2} dot isAnimationActive={false} />
            <Line type="monotone" dataKey="secondary" name="영업이익" stroke="#14B8A6" strokeWidth={2} dot isAnimationActive={false} />
          </LineChart>
        </ResponsiveContainer>
      </ChartCard>

      {/* 분기말 종가 ÷ TTM EPS(=PER), 분기말 종가 ÷ 주당순자산(=PBR). 다른 다섯
          카드와 같은 분기 축을 쓴다 — 예전에는 이 카드만 연도 축이었는데, 값을
          지어냈기 때문이었다. */}
      <ChartCard title="PER · PBR" subtitle="분기말 종가 기준 밸류에이션">
        {hasAnyValue(charts.valuation, ["per", "pbr"]) ? (
          <ResponsiveContainer width="100%" height={140}>
            <LineChart data={charts.valuation}>
              <CartesianGrid strokeDasharray="3 3" stroke="var(--border)" vertical={false} />
              <XAxis dataKey="year" tick={periodTick} interval={periodTickInterval(periods.length)} height={PERIOD_AXIS_HEIGHT} />
              <YAxis tick={{ fontSize: 11, fill: "var(--text-muted)" }} />
              <Tooltip
                contentStyle={tooltipStyle}
                formatter={(v, name) => [`${Number(v).toFixed(1)}배`, name]}
              />
              <Legend wrapperStyle={{ fontSize: 11 }} />
              <Line type="monotone" dataKey="per" name="PER" stroke="#EF4444" strokeWidth={2} dot={{ r: 2 }} isAnimationActive={false} />
              <Line type="monotone" dataKey="pbr" name="PBR" stroke="#3B82F6" strokeWidth={2} dot={{ r: 2 }} isAnimationActive={false} />
            </LineChart>
          </ResponsiveContainer>
        ) : (
          <EmptySeries reason="EPS 또는 자본총계가 없어 계산할 수 없습니다" />
        )}
      </ChartCard>

      <ChartCard
        title="재무 건전성"
        subtitle="총자본 · 총부채 · 부채비율"
        onClick={() => setModalPreset("stability")}
      >
        <ResponsiveContainer width="100%" height={140}>
          <ComposedChart data={charts.health}>
            <CartesianGrid strokeDasharray="3 3" stroke="var(--border)" vertical={false} />
            <XAxis dataKey="label" tick={periodTick} interval={periodTickInterval(periods.length)} height={PERIOD_AXIS_HEIGHT} />
            <YAxis yAxisId="left" hide />
            <YAxis yAxisId="right" orientation="right" tick={{ fontSize: 11, fill: "var(--text-muted)" }} unit="%" />
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
            <Bar yAxisId="left" dataKey="equity" name="총자본" fill="#14B8A6" radius={[2, 2, 0, 0]} isAnimationActive={false} />
            <Bar yAxisId="left" dataKey="liabilities" name="총부채" fill="#0F766E" radius={[2, 2, 0, 0]} isAnimationActive={false} />
            <Line yAxisId="right" type="monotone" dataKey="debtRatio" name="부채비율" stroke="#F59E0B" strokeWidth={2} dot={false} isAnimationActive={false} />
          </ComposedChart>
        </ResponsiveContainer>
      </ChartCard>

      {/* EPS가 한 분기도 없으면 0 직선 대신 이유를 적는다. 예전에는 `?? 0`이라
          SK하이닉스처럼 EPS를 다른 계정명으로 신고하는 회사가 「EPS 0원」으로
          그려졌다(실제 62,044원). */}
      <ChartCard title="EPS 추이" subtitle="주당순이익">
        {hasAnyValue(charts.epsSeries, ["value"]) ? (
          <ResponsiveContainer width="100%" height={140}>
            <AreaMiniChart data={charts.epsSeries} color="#10B981" currency={currency} tick={periodTick} />
          </ResponsiveContainer>
        ) : (
          <EmptySeries reason="공시에서 주당순이익 항목을 찾지 못했습니다" />
        )}
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

/*
 * 이 여섯 카드의 시리즈는 모두 `isAnimationActive={false}`다.
 *
 * recharts의 등장 애니메이션은 `requestAnimationFrame`으로 `stroke-dasharray`를
 * 0에서 전체 길이까지 늘린다. **rAF가 안 돌면 그 자리에서 멈추고, 다시는
 * 끝나지 않는다** — 브라우저는 탭이 가려지면 rAF를 아예 호출하지 않는데
 * (`visibilityState`가 'visible'이어도 그렇다), 그러면 선이 1%만 그려진 채로
 * 영원히 남는다. 실측: 가려진 창에서 rAF 1초에 0회, 부채비율 선의
 * `stroke-dasharray`가 `5.36px 450.29px`로 6초 내내 고정.
 *
 * 사용자에게는 이것이 「데이터가 중간에 비어 있다」로 보인다 — 실제 결측과
 * 구분이 안 된다. 작은 카드에서 애니메이션이 주는 값보다 이 위험이 크다.
 */
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
        <h3 className="text-cell font-semibold text-[var(--text-primary)]">{title}</h3>
        <p className="text-micro text-[var(--text-muted)]">{subtitle}</p>
      </div>
      {children}
    </Card>
  );
}

function AreaMiniChart({
  data,
  color,
  currency,
  tick,
}: {
  data: { year: string; value: number | null }[];
  color: string;
  currency: QuoteCurrency;
  tick: ReturnType<typeof periodTickRenderer>;
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
        <XAxis dataKey="year" tick={tick} interval={periodTickInterval(data.length)} height={PERIOD_AXIS_HEIGHT} />
        <YAxis
          tick={{ fontSize: 11, fill: "var(--text-muted)" }}
          tickFormatter={(v) => formatFinancialTick(Number(v), currency)}
          width={44}
        />
        <Tooltip contentStyle={tooltipStyle} formatter={(v) => [perShare(v), "EPS"]} />
        <Line type="monotone" dataKey="value" stroke={color} strokeWidth={2} dot isAnimationActive={false} />
      </LineChart>
    </ResponsiveContainer>
  );
}

/**
 * 여섯 카드의 데이터. **`periods`에 실제로 있는 값만 쓴다.**
 *
 * 예전에는 두 곳에서 숫자를 지어냈다. ⓐ PER·PBR은 `buildTrendSeries`로 현재값
 * 하나에서 5년치를 의사난수로 역산했고(심볼 문자열이 시드라 매번 같은 그림이
 * 나와 「추이」처럼 보였다), ⓑ 분기 이력이 없으면 현재 매출 × 0.72~1.00으로
 * 5년치를 만들어 「연간 추이 (추정 포함)」이라 적었다. 후자는 실측 2,516종목이
 * 보고 있었다.
 *
 * 이제 PER·PBR은 서버가 분기말 종가 × 그 분기 재무로 계산해 주고, 이력이 없으면
 * 차트를 그리지 않는다(호출부가 안내 문구로 갈린다).
 *
 * 없는 값은 `null`로 둔다 — `?? 0`은 「0이다」라고 말한다. SK하이닉스 EPS 차트가
 * 0에 붙은 직선이었던 이유가 그것이다(실제 EPS 62,044원).
 */
function buildFinancialCharts(periods: FinancialHistoryPeriod[]) {
  const ticks = buildPeriodTicks(periods);
  const at = (i: number) => ticks[i].full;

  return {
    revenueProfit: periods.map((p, i) => ({
      year: at(i),
      revenue: p.revenue ?? null,
      netIncome: p.netIncome ?? null,
    })),
    margins: periods.map((p, i) => ({
      year: at(i),
      opMargin: p.operatingMargin ?? null,
      netMargin: p.netProfitMargin ?? null,
    })),
    growth: periods.map((p, i) => ({
      year: at(i),
      revenue: p.revenueGrowthQoq ?? null,
      secondary: p.operatingIncomeGrowthQoq ?? null,
    })),
    valuation: periods.map((p, i) => ({
      year: at(i),
      per: p.per ?? null,
      pbr: p.pbr ?? null,
    })),
    health: periods.map((p, i) => ({
      label: at(i),
      equity: p.totalEquity ?? null,
      liabilities: p.totalLiabilities ?? null,
      debtRatio: p.debtRatio ?? null,
    })),
    epsSeries: periods.map((p, i) => ({ year: at(i), value: p.epsBasic ?? null })),
    tickIndex: periodTickIndex(ticks),
  };
}

/** 한 시리즈에 값이 하나라도 있는지 — 없으면 빈 축 대신 안내를 그린다. */
function hasAnyValue(rows: Array<Record<string, unknown>>, keys: string[]): boolean {
  return rows.some((row) => keys.some((k) => row[k] != null));
}

/**
 * 값이 하나도 없는 시리즈의 자리. **빈 축을 그리지 않는다** — 0에 붙은 직선이나
 * 눈금만 있는 격자는 「값이 0이다」 또는 「버그다」로 읽힌다. 왜 없는지를 적는 편이
 * 짧고 정확하다.
 */
function EmptySeries({ reason }: { reason: string }) {
  return (
    <div className="flex h-[140px] flex-col items-center justify-center gap-0.5 text-center">
      <p className="text-cell text-[var(--text-muted)]">데이터 없음</p>
      <p className="text-micro text-[var(--text-muted)]">{reason}</p>
    </div>
  );
}
