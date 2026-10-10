"use client";

import { useMemo } from "react";
import { Loader2, Target } from "lucide-react";

import {
  formatScenarioPct,
  formatScenarioPrice,
  scenarioAxisPositions,
  SCENARIO_TONE,
  type PriceScenario,
  type ScenarioPoint,
} from "@/lib/price-scenario-api";
import { cn } from "@/lib/utils";

/**
 * 90일 시나리오 — **개요 「종합 점수」 카드 안**의 한 구획 (2026-10-07 사용자 요청 9).
 *
 * 2026-09-19부터 밸류에이션 탭 맨 위의 독립 카드였다. 이제 점수 카드가 「이 종목이 어떤가」
 * 다음에 「그래서 90일 뒤 어디쯤인가」를 함께 말한다. 가독성을 위해 바꾼 것:
 *
 *   · 맨 위에 **한 문장**: 「90일 뒤 A ~ B 사이일 가능성이 높습니다 (이 방법 적중률 N%)」
 *   · 세 줄 목록 → **세 칸 타일**(약세 · 중립 · 강세) — 가격·등락률·내재 PER이 한눈에
 *   · 간단/상세 토글을 걷었다 — 검증 수치는 접힌 「검증 수치」 안으로
 *
 * # 하나의 목표가를 찍지 않습니다
 *
 * 실측에서 90일 뒤 실제 가격은 현재가에서 **평균 24%** 떨어져 있었습니다. 그 오차를
 * 아는 채로 한 숫자를 내놓으면 그 정밀도는 거짓입니다.
 *
 * # 막대는 **가격 축**입니다
 *
 * 세 칸을 같은 간격으로 그리면 「강세가 약세보다 멀다」는 사실이 사라집니다. 로그
 * 밴드라 위쪽 거리가 언제나 더 먼데, 그 비대칭이 이 구획이 말하려는 것의 절반입니다.
 *
 * # 적중률은 이 종목의 성적이 아닙니다
 *
 * 종목별 표본은 6건뿐이라 적중률이라 부를 수 없습니다. 문장을 「이 방법은」으로
 * 시작해 그 구분을 잃지 않게 합니다.
 *
 * 시나리오가 없거나(이력 부족) 못 받았으면 **아무것도 그리지 않는다** — 점수 카드 안에서
 * 「없다」를 한 줄 더 적는 것은 높이만 쓴다.
 */
export function PriceScenarioSection({
  scenario,
  loading,
  className,
}: {
  scenario: PriceScenario | null;
  loading: boolean;
  className?: string;
}) {
  const points = useMemo(() => scenario?.points ?? [], [scenario]);
  const axis = useMemo(() => scenarioAxisPositions(points), [points]);

  if (loading) {
    return (
      <p className={cn("flex items-center gap-1.5 px-3 py-2 text-cell text-[var(--text-muted)]", className)}>
        <Loader2 className="h-3 w-3 animate-spin" />
        90일 시나리오를 계산하는 중입니다…
      </p>
    );
  }
  if (!scenario || scenario.unavailable || points.length === 0) return null;

  const bear = points.find((p) => p.kind === "bear");
  const bull = points.find((p) => p.kind === "bull");
  const acc = scenario.accuracy;
  const base = points.find((p) => p.kind === "base");
  /*
    범위 폭을 함께 적는다 (2026-10-10, BUSINESS_REVIEW §5 D4). 「적중률 77%」만 있으면 −38%~+61%처럼
    넓은 범위가 정확한 예측처럼 읽힌다 — 범위가 넓을수록 그 안에 들어갈 확률은 저절로 오른다.
  */
  const widthPct = bear && bull && base && base.price > 0 ? ((bull.price - bear.price) / base.price) * 100 : null;

  return (
    <div className={cn("space-y-2 px-3 py-2.5", className)}>
      <div className="flex flex-wrap items-center gap-x-2 gap-y-0.5">
        <Target className="h-3.5 w-3.5 shrink-0 text-[var(--brand)]" aria-hidden />
        <span className="text-cell font-semibold text-[var(--text-primary)]">
          {scenario.horizonDays}일 시나리오
        </span>
        {/* 종가 기준임을 적습니다 — 실시간 시세가 아닙니다. */}
        <span className="text-micro text-[var(--text-muted)]">{scenario.asOf} 종가 기준</span>
      </div>

      {bear && bull && (
        <p className="text-cell leading-relaxed text-[var(--text-secondary)]">
          {scenario.horizonDays}일 뒤{" "}
          <b className={SCENARIO_TONE.bear}>{formatScenarioPrice(bear.price, scenario.currency)}</b> ~{" "}
          <b className={SCENARIO_TONE.bull}>{formatScenarioPrice(bull.price, scenario.currency)}</b> 사이일
          가능성이 높습니다
          {widthPct != null && (
            <span className="text-[var(--text-muted)]"> (범위 폭 {widthPct.toFixed(0)}%)</span>
          )}
          .
          {acc ? (
            <span className="text-[var(--text-muted)]">
              {" "}
              같은 방법으로 과거에 그린 범위가 실제 가격을 담은 비율은{" "}
              <b className="text-[var(--text-primary)]">{acc.hitRate.toFixed(0)}%</b>입니다(범위 포함률).
            </span>
          ) : null}
        </p>
      )}

      <div className="grid grid-cols-3 gap-1.5">
        {(["bear", "base", "bull"] as const).map((kind) => {
          const p = points.find((x) => x.kind === kind);
          return p ? <ScenarioTile key={kind} point={p} currency={scenario.currency} /> : <span key={kind} />;
        })}
      </div>

      <PriceAxis points={points} axis={axis} />

      <p className="text-micro text-[var(--text-muted)]">
        {scenario.horizonDays}일 변동성{" "}
        <b className="text-[var(--text-primary)]">{scenario.volatilityPct.toFixed(1)}%</b>
      </p>

      {acc && (
        <details className="group text-micro text-[var(--text-muted)]">
          <summary className="cursor-pointer select-none hover:text-[var(--text-primary)]">검증 수치</summary>
          <div className="mt-1.5 flex flex-wrap gap-1.5">
            <Kpi label="범위 포함률" value={`${acc.hitRate.toFixed(1)}%`} />
            <Kpi label="중립 평균 오차" value={`${acc.baseErrorPct.toFixed(1)}%`} />
            <Kpi label="표본" value={acc.samples.toLocaleString("ko-KR")} />
          </div>
          <p className="mt-1.5">
            {acc.symbols}종목 × {acc.dates}개 시점 · {scenario.horizonDays}일 지평
            {acc.measuredAt ? ` · ${acc.measuredAt} 기준` : ""}
          </p>
          <p className="mt-1">범위가 넓을수록 포함률은 저절로 오릅니다 — 예측이 아니라 변동성으로 그린 범위입니다.</p>
        </details>
      )}
    </div>
  );
}

/* ------------------------------------------------------------------------- */

function ScenarioTile({ point, currency }: { point: ScenarioPoint; currency?: string }) {
  return (
    <div
      className={cn(
        "rounded-md border px-2 py-1.5 text-center",
        point.kind === "base" ? "border-[var(--border)] bg-[var(--surface-elevated)]/60" : "border-[var(--border)]",
      )}
    >
      <span className={cn("block text-micro font-semibold", SCENARIO_TONE[point.kind])}>
        {point.kind === "base" ? "중립 · 현재가" : point.label}
      </span>
      <span className="block text-note font-bold tabular-nums text-[var(--text-primary)]">
        {formatScenarioPrice(point.price, currency)}
      </span>
      <span className={cn("block text-micro tabular-nums", SCENARIO_TONE[point.kind])}>
        {point.kind === "base" ? "±0%" : formatScenarioPct(point.changePct)}
      </span>
      {/* PER이 있는 종목은 60%뿐이라 없으면 줄을 비운다 — 「-」로 채우지 않는다. */}
      {point.impliedPer != null && (
        <span className="block text-micro tabular-nums text-[var(--text-muted)]">
          PER {point.impliedPer.toFixed(1)}배
        </span>
      )}
    </div>
  );
}

function PriceAxis({ points, axis }: { points: ScenarioPoint[]; axis: Map<string, number> }) {
  return (
    <div
      className="relative h-2 rounded-full"
      style={{
        background:
          "linear-gradient(90deg, var(--loss-bg, rgba(37,99,235,.18)) 0%, var(--muted) 50%, var(--profit-bg, rgba(220,38,38,.18)) 100%)",
      }}
      aria-hidden
    >
      {points.map((p) => (
        <span
          key={p.kind}
          className={cn(
            "absolute top-1/2 h-3.5 w-0.5 -translate-y-1/2 rounded-sm",
            p.kind === "base" ? "bg-[var(--text-primary)]" : p.kind === "bear" ? "bg-[var(--loss)]" : "bg-[var(--profit)]",
          )}
          style={{ left: `${axis.get(p.kind) ?? 50}%` }}
        />
      ))}
    </div>
  );
}

function Kpi({ label, value }: { label: string; value: string }) {
  return (
    <div className="min-w-[5.5rem] flex-1 rounded-md border border-[var(--border)] px-2 py-1">
      <span className="block text-micro text-[var(--text-muted)]">{label}</span>
      <span className="block text-note font-bold tabular-nums text-[var(--text-primary)]">{value}</span>
    </div>
  );
}
