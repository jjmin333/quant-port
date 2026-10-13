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
 * 90일 뒤 가격 범위 — 개요, 다음 실적 옆 (2026-10-13 요청 3, 시안 `docs/mockups/stock-overview-v2.html` ④).
 *
 * 한 문장(「10번 중 N번꼴로 A ~ B 사이」) + 가격 축 막대 + 검증 세 칸(범위 안 · 아래 · 위)만 남긴다.
 * 10-10의 「폭 ≥60%면 접고 『방향을 말하기 어렵습니다』」는 대부분의 종목이 그 문장만 보게 만들어 걷었고,
 * 아래에 붙던 변동성·PER·검증 수치 설명도 같은 날 사용자 지시로 뺐다.
 *
 * # 하나의 목표가를 찍지 않습니다
 *
 * 실측에서 90일 뒤 실제 가격은 현재가에서 **평균 24%** 떨어져 있었습니다. 그 오차를
 * 아는 채로 한 숫자를 내놓으면 그 정밀도는 거짓입니다.
 *
 * # 막대는 **가격 축**입니다
 *
 * 로그 밴드라 위쪽 거리가 언제나 더 멉니다 — 같은 간격으로 그리면 그 비대칭이 사라집니다.
 *
 * # 적중률은 이 종목의 성적이 아닙니다
 *
 * 종목별 표본은 6건뿐이라, 세 칸은 「같은 방법으로 과거에 그린 범위」의 성적입니다(title로 밝힌다).
 *
 * 시나리오가 없거나(이력 부족) 못 받았으면 **아무것도 그리지 않는다**.
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
        90일 범위를 계산하는 중입니다…
      </p>
    );
  }
  if (!scenario || scenario.unavailable || points.length === 0) return null;

  const bear = points.find((p) => p.kind === "bear");
  const bull = points.find((p) => p.kind === "bull");
  const acc = scenario.accuracy;
  const price = (v: number) => withUnit(v, scenario.currency);
  const sample = acc
    ? `같은 방법으로 과거에 그린 범위를 ${scenario.horizonDays}일 뒤 실제 가격과 맞춰 본 결과 · ${acc.symbols}종목 × ${acc.dates}시점 = ${acc.samples.toLocaleString("ko-KR")}건`
    : undefined;

  return (
    <div className={cn("space-y-2 px-3 py-2.5", className)}>
      <div className="flex flex-wrap items-center gap-x-2 gap-y-0.5">
        <Target className="h-3.5 w-3.5 shrink-0 text-[var(--brand)]" aria-hidden />
        <span className="text-cell font-semibold text-[var(--text-primary)]">{scenario.horizonDays}일 뒤 가격 범위</span>
        {/* 종가 기준임을 적습니다 — 실시간 시세가 아닙니다. */}
        <span className="text-micro tabular-nums text-[var(--text-muted)]">
          {scenario.asOf.slice(5).replace("-", "/")} 종가 {price(scenario.close)} 기준
        </span>
      </div>

      {bear && bull && (
        <p className="text-cell leading-relaxed text-[var(--text-secondary)]">
          {scenario.horizonDays}일 뒤 주가는{" "}
          {acc && <b className="text-[var(--text-primary)]">10번 중 {Math.round(acc.hitRate / 10)}번꼴로 </b>}
          <b className={cn("tabular-nums", SCENARIO_TONE.bear)}>{price(bear.price)}</b> ~{" "}
          <b className={cn("tabular-nums", SCENARIO_TONE.bull)}>{price(bull.price)}</b> 사이에 있습니다.
        </p>
      )}

      <PriceAxis points={points} axis={axis} currency={scenario.currency} />

      {acc && (
        <div className="grid grid-cols-3 gap-1.5" title={sample}>
          <Kpi label="범위 안에 든 비율" value={`${acc.hitRate.toFixed(0)}%`} />
          <Kpi label="범위보다 아래" value={`${acc.missBelowPct.toFixed(1)}%`} tone={SCENARIO_TONE.bear} />
          <Kpi label="범위보다 위" value={`${acc.missAbovePct.toFixed(1)}%`} tone={SCENARIO_TONE.bull} />
        </div>
      )}
    </div>
  );
}

/* ------------------------------------------------------------------------- */

function withUnit(v: number, currency?: string): string {
  return (currency ?? "").toUpperCase() === "USD" ? `$${formatScenarioPrice(v, currency)}` : `${formatScenarioPrice(v, currency)}원`;
}

/** 가격 축 — 위에 등락률, 아래에 가격. 양끝 이름표는 끝에 붙이고 현재가만 제 자리에 세운다. */
function PriceAxis({ points, axis, currency }: { points: ScenarioPoint[]; axis: Map<string, number>; currency?: string }) {
  const label = (p: ScenarioPoint) =>
    p.kind === "bear" ? { left: 0 } : p.kind === "bull" ? { right: 0 } : { left: `${axis.get(p.kind) ?? 50}%`, transform: "translateX(-50%)" };
  return (
    <div className="space-y-0.5">
      <div className="relative h-4 text-micro font-semibold tabular-nums">
        {points.map((p) => (
          <span key={p.kind} className={cn("absolute whitespace-nowrap", SCENARIO_TONE[p.kind])} style={label(p)}>
            {p.kind === "base" ? "현재" : formatScenarioPct(p.changePct)}
          </span>
        ))}
      </div>
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
      <div className="relative h-4 text-micro tabular-nums text-[var(--text-muted)]">
        {points.map((p) => (
          <span key={p.kind} className="absolute whitespace-nowrap" style={label(p)}>
            {withUnit(p.price, currency)}
          </span>
        ))}
      </div>
    </div>
  );
}

function Kpi({ label, value, tone }: { label: string; value: string; tone?: string }) {
  return (
    <div className="min-w-0 rounded-md border border-[var(--border)] px-2 py-1">
      <span className="block truncate text-micro text-[var(--text-muted)]">{label}</span>
      <span className={cn("block text-note font-bold tabular-nums", tone ?? "text-[var(--text-primary)]")}>{value}</span>
    </div>
  );
}
