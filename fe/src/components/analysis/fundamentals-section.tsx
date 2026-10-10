"use client";

import { Card } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { ProvenanceStamp } from "@/components/ui/provenance-stamp";
import { fundamentalsProvenance } from "@/lib/data-reliability";
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
      <Card className="flex min-h-[160px] items-center justify-center p-6 text-cell text-[var(--text-muted)]">
        지표 데이터 로딩 중…
      </Card>
    );
  }

  if (!data) {
    return (
      <Card className="flex min-h-[160px] items-center justify-center p-6 text-cell text-[var(--text-muted)]">
        지표 데이터를 불러오지 못했습니다.
      </Card>
    );
  }

  if (data.coverageTier === "not_applicable") {
    return (
      <Card className="border-dashed p-5 text-center">
        <Badge variant="info">{COVERAGE_TIER_LABEL.not_applicable}</Badge>
        <p className="mt-3 text-cell text-[var(--text-secondary)]">
          {data.message ?? "재무제표 미제공 상품입니다."}
        </p>
      </Card>
    );
  }

  if (data.coverageTier === "unavailable") {
    return (
      <Card className="p-5 text-center">
        <Badge variant="warning">{COVERAGE_TIER_LABEL.unavailable}</Badge>
        <p className="mt-3 text-cell text-[var(--text-secondary)]">공시 재무 데이터가 없습니다.</p>
      </Card>
    );
  }

  const panel = panelProp ?? mapFundamentalsToPanel(data, stock);
  if (!panel) return null;

  const tierVariant =
    panel.coverageTier === "full" ? "profit" : panel.coverageTier === "basic" ? "warning" : "info";

  return (
    <Card className="p-0">
      {/*
        # 점수는 **이 카드가 그리지 않는다** (2026-09-03 사용자 결정)

        여기 머리줄에 「가치 점수 62 / 100 · 종합 66」이 글자로 적혀 있었다. 그런데
        재무제표 탭은 같은 것을 탭 맨 위의 막대 달린 배지(`AxisScoreBadge`)로 보여
        준다 — 같은 앱에서 「수익성 점수」와 「가치 점수」를 읽는 자리와 모양이 서로
        달랐다는 뜻이다. 점수는 이제 밸류에이션 탭도 그 배지로 그린다
        (`TAB_SCORE_AXIS`), 그래서 이 자리는 카드가 무엇인지(종합 지표)와 그 숫자를
        얼마나 믿을 수 있는지(커버리지 등급)만 말한다.
      */}
      <div className="flex flex-wrap items-center gap-x-3 gap-y-1 border-b border-[var(--border)] px-3 py-2.5">
        <div className="flex min-w-0 items-center gap-1.5">
          <p className="text-micro text-[var(--text-muted)]">종합 지표</p>
          {/* 커버리지 등급은 「이 카드의 숫자를 얼마나 믿을 수 있나」라 남긴다 —
              함께 있던 「핵심 요약」 배지와 태그 줄은 뺐다(전부 중복이었다). */}
          <Badge variant={tierVariant} className="text-micro">
            {COVERAGE_TIER_LABEL[panel.coverageTier]}
          </Badge>
        </div>
        {/*
          기준(회계기간)·출처(DART/EDGAR)는 커버리지 등급이 **말하지 않는** 것이다
          (요구 8). 등급은 「얼마나 채워졌나」이고 이것은 「언제 것이고 어디서 왔나」다.

          2026-09-03에 뺀 각주("재무제표·발행주식수는 실시간이 아니며…")를 되살리는
          것이 아니다 — 그것은 모든 종목에 똑같이 붙는 산문이었고, 이것은 종목마다
          다른 사실 두 개다.
        */}
        <ProvenanceStamp className="ml-auto" parts={fundamentalsProvenance(data)} />
      </div>
      {data.supplement && data.supplement.fields.length > 0 && (
        <p className="border-b border-[var(--border)] px-3 py-1.5 text-micro text-[var(--text-muted)]">
          {supplementLabel(data.supplement.fields)}은 공시 재무에 없어 Finnhub 값(본국 상장 기준 ·{" "}
          {data.supplement.asOf.slice(5, 10).replace("-", "/")})으로 채웠습니다.
        </p>
      )}

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
        <span className="text-cell text-[var(--text-secondary)]">{metric.label}</span>
        <span className={cn("text-micro font-medium", metricToneClass(metric.tone))}>
          {metric.verdict}
        </span>
      </div>
      <p className="mt-1 font-num text-lg font-semibold tabular-nums text-[var(--text-primary)]">
        {metric.value}
      </p>
      {/* 값 바로 아래에 한 줄 풀이. 「PER」만 적힌 카드는 입문자에게 아무 말도
          하지 않는다 — 무엇을 무엇으로 나눈 값이고 높으면/낮으면 무슨 뜻인지가
          숫자 옆에 붙어 있어야 한다. */}
      {metric.hint && (
        <p className="mt-1 text-micro leading-snug text-[var(--text-muted)]">{metric.hint}</p>
      )}
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


const SUPPLEMENT_FIELD_LABEL: Record<string, string> = {
  per: "PER",
  pbr: "PBR",
  psr: "PSR",
  roe: "ROE",
  operatingMargin: "영업이익률",
  netProfitMargin: "순이익률",
  revenueGrowthYoy: "매출 성장",
  epsGrowthYoy: "EPS 성장",
  currentRatio: "유동비율",
};

/** 「PER·PBR·ROE 등 5개」 꼴 — 칸 이름을 다 늘어놓으면 한 줄을 넘는다. */
function supplementLabel(fields: string[]): string {
  const names = fields.map((f) => SUPPLEMENT_FIELD_LABEL[f] ?? f);
  return names.length <= 3 ? names.join("·") : `${names.slice(0, 3).join("·")} 등 ${names.length}개`;
}
