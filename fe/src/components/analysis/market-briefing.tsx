"use client";

import { TrendingDown, TrendingUp } from "lucide-react";
import { Card } from "@/components/ui/card";
import { formatPrice } from "@/lib/format";
import type {
  BriefingScenario,
  BriefingTone,
  MarketBriefing as MarketBriefingData,
  PriceLevel,
} from "@/lib/stock-summary-types";
import { cn } from "@/lib/utils";

const TONE_CLASS: Record<BriefingTone, string> = {
  positive: "text-[var(--success)]",
  negative: "text-[var(--danger)]",
  neutral: "text-[var(--text-secondary)]",
};

const TONE_DOT: Record<BriefingTone, string> = {
  positive: "bg-[var(--success)]",
  negative: "bg-[var(--danger)]",
  neutral: "bg-[var(--text-muted)]",
};

function LevelRow({
  level,
  currency,
  direction,
}: {
  level: PriceLevel;
  currency: "KRW" | "USD";
  direction: "up" | "down";
}) {
  return (
    <li className="flex items-baseline justify-between gap-2 py-1 text-[11px]">
      <span className="shrink-0 text-[var(--text-muted)]">{level.label}</span>
      <span className="min-w-0 flex-1 border-b border-dashed border-[var(--border)]" aria-hidden />
      <span className="shrink-0 tabular-nums text-[var(--text-primary)]">
        {formatPrice(level.price, currency)}
      </span>
      <span
        className={cn(
          "w-14 shrink-0 text-right tabular-nums",
          direction === "up" ? "text-[var(--success)]" : "text-[var(--danger)]",
        )}
      >
        {level.distancePct > 0 ? "+" : ""}
        {level.distancePct.toFixed(1)}%
      </span>
    </li>
  );
}

function ScenarioCard({
  scenario,
  currency,
  direction,
}: {
  scenario: BriefingScenario;
  currency: "KRW" | "USD";
  direction: "up" | "down";
}) {
  const up = direction === "up";
  return (
    <div className="rounded-lg border border-[var(--border)] bg-[var(--surface-elevated)]/40 p-2.5">
      <div
        className={cn(
          "mb-1.5 flex items-center gap-1.5 text-xs font-semibold",
          up ? "text-[var(--success)]" : "text-[var(--danger)]",
        )}
      >
        {up ? <TrendingUp className="h-3.5 w-3.5" /> : <TrendingDown className="h-3.5 w-3.5" />}
        {up ? "상승 시나리오" : "하락 시나리오"}
      </div>
      <p className="mb-1.5 text-[11px] leading-relaxed text-[var(--text-secondary)]">
        {scenario.summary}
      </p>
      {scenario.levels && scenario.levels.length > 0 && (
        <ul className="divide-y divide-[var(--border)]">
          {scenario.levels.map((level) => (
            <LevelRow
              key={`${level.label}-${level.price}`}
              level={level}
              currency={currency}
              direction={direction}
            />
          ))}
        </ul>
      )}
    </div>
  );
}

/**
 * The rule-based 시장 분석 that precedes the AI prose.
 *
 * Everything here is server-derived from real figures — every price level is an
 * actual moving average, swing extreme or 52-week bound, never a target someone
 * made up. That is what lets it render for every symbol, including the ones the
 * LLM never sees.
 */
export function MarketBriefing({
  briefing,
  currency,
  className,
}: {
  briefing: MarketBriefingData | undefined;
  currency: "KRW" | "USD";
  className?: string;
}) {
  if (!briefing) return null;
  const hasScenarios = briefing.upside || briefing.downside;
  if (!briefing.keyPoints?.length && !hasScenarios) return null;

  return (
    <Card className={cn("p-0", className)}>
      <div className="flex flex-wrap items-baseline justify-between gap-2 border-b border-[var(--border)] px-3 py-2.5">
        <h3 className="text-sm font-semibold text-[var(--text-primary)]">시장 분석</h3>
        <span className="text-[10px] text-[var(--text-muted)]">
          보유 데이터 기반 자동 산출{briefing.asOf ? ` · ${briefing.asOf}` : ""}
        </span>
      </div>

      <div className="space-y-3 p-3">
        {briefing.keyPoints && briefing.keyPoints.length > 0 && (
          <div>
            <h4 className="mb-1.5 text-[11px] font-medium text-[var(--text-muted)]">투자 포인트</h4>
            <ul className="space-y-1.5">
              {briefing.keyPoints.map((point) => (
                <li key={point.label} className="flex items-start gap-2 text-xs leading-relaxed">
                  <span
                    className={cn("mt-1.5 h-1.5 w-1.5 shrink-0 rounded-full", TONE_DOT[point.tone])}
                    aria-hidden
                  />
                  <span className="min-w-0">
                    <span className="font-medium text-[var(--text-primary)]">{point.label}</span>
                    <span className="mx-1 text-[var(--text-muted)]">·</span>
                    <span className={TONE_CLASS[point.tone]}>{point.text}</span>
                  </span>
                </li>
              ))}
            </ul>
          </div>
        )}

        {hasScenarios && (
          <div className="grid gap-2 sm:grid-cols-2">
            {briefing.upside && (
              <ScenarioCard scenario={briefing.upside} currency={currency} direction="up" />
            )}
            {briefing.downside && (
              <ScenarioCard scenario={briefing.downside} currency={currency} direction="down" />
            )}
          </div>
        )}

        <p className="text-[10px] leading-relaxed text-[var(--text-muted)]">
          가격대는 이동평균·최근 스윙 고저·52주 범위 등 실제 계산된 지표입니다. 목표가가 아니며 투자
          자문이 아닙니다.
        </p>
      </div>
    </Card>
  );
}
