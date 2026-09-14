"use client";

import { StockAvatar } from "@/components/stocks/stock-avatar";
import { PriceDisplay } from "@/components/ui/price-display";
import { formatDateKST, formatPercent } from "@/lib/format";
import type { Prediction } from "@/lib/prediction-types";
import { cn } from "@/lib/utils";
import {
  PredictionAxis,
  PredictionDday,
  PredictionGauge,
  PredictionSeal,
  PredictionStatusBadge,
  horizonLabel,
  progressLabel,
  toneForValue,
} from "@/components/predictions/prediction-bits";

/**
 * 목록 행 두 가지 밀도.
 *
 * - `axis` — 가격 축을 행 안에 넣는다(행 높이 ~78px). 「지금 어디쯤 와 있나」가
 *   좌표로 읽혀 스캔이 빠르다. 예측이 열 건 안팎일 때의 기본값이다.
 * - `compact` — 축 대신 게이지 하나(행 높이 ~36px). 수십 건이 쌓이면 축은 화면을
 *   다 먹는다.
 *
 * 폭 판단은 **컨테이너 쿼리**로 한다. 뷰포트로 재면 레일이 있는 데스크탑에서 본문이
 * 실제로는 좁은데도 넓은 줄 알고 축을 넣어 가로 오버플로가 난다 — 관심종목 행이
 * 같은 이유로 `@container`를 쓴다.
 */

export type RowDensity = "axis" | "compact";

/** 「74,200 → 82,000원 예상」 한 줄. 숫자 나열 대신 문장으로 읽히게 한다. */
function PlanLine({ p, className }: { p: Prediction; className?: string }) {
  return (
    <span className={cn("tabular-nums", className)}>
      {p.basePrice !== undefined && (
        <>
          <span className="text-[var(--text-muted)]">
            <PriceDisplay value={p.basePrice} currency={p.currency} size="inherit" />
          </span>
          <span className="mx-1 text-[var(--text-muted)]">→</span>
        </>
      )}
      <b className={p.upside ? "text-[var(--profit)]" : "text-[var(--loss)]"}>
        <PriceDisplay value={p.targetPrice} currency={p.currency} size="inherit" />
      </b>
      {p.expectedReturn !== undefined && (
        <span className="ml-1 text-[var(--text-muted)]">
          ({formatPercent(p.expectedReturn)})
        </span>
      )}
    </span>
  );
}

export function PredictionRow({
  prediction,
  density,
  onOpen,
}: {
  prediction: Prediction;
  density: RowDensity;
  onOpen: () => void;
}) {
  const p = prediction;
  const prog = progressLabel(p);

  if (density === "compact") {
    return (
      <button
        type="button"
        onClick={onOpen}
        className="flex w-full items-center gap-2 px-3 py-2 text-left hover:bg-[var(--surface-elevated)]/60"
      >
        <PredictionStatusBadge status={p.status} className="w-[52px] text-center" />
        <StockAvatar symbol={p.symbol} name={p.symbolName} size="xs" />
        <span className="min-w-0 flex-1 @md:flex-none @md:w-[104px]">
          <span className="block truncate text-cell font-semibold text-[var(--text-primary)]">
            {p.symbolName}
          </span>
        </span>
        <PlanLine p={p} className="hidden text-micro text-[var(--text-muted)] @2xl:inline" />
        <span className="hidden flex-1 @lg:block">
          <PredictionGauge prediction={p} />
        </span>
        <span
          className={cn(
            "hidden w-[62px] shrink-0 text-right text-micro font-semibold tabular-nums @lg:block",
            prog.tone,
          )}
        >
          {prog.text}
        </span>
        <span
          className={cn(
            "w-[62px] shrink-0 text-right text-cell font-semibold tabular-nums",
            toneForValue(p.sinceBasePct),
          )}
        >
          {p.sinceBasePct === undefined ? "—" : formatPercent(p.sinceBasePct)}
        </span>
        <span className="hidden w-[54px] shrink-0 text-right @xl:block">
          <PredictionDday prediction={p} />
        </span>
      </button>
    );
  }

  return (
    <button
      type="button"
      onClick={onOpen}
      className="flex w-full items-center gap-2.5 px-3 py-2.5 text-left hover:bg-[var(--surface-elevated)]/60"
    >
      <PredictionSeal prediction={p} size="sm" className="@md:size-[46px]" />

      <span className="min-w-0 flex-1 @2xl:max-w-[260px]">
        <span className="flex flex-wrap items-center gap-1.5">
          <StockAvatar symbol={p.symbol} name={p.symbolName} size="xs" />
          <span className="truncate text-note font-semibold text-[var(--text-primary)]">
            {p.symbolName}
          </span>
          {p.conviction !== undefined && (
            <span className="shrink-0 rounded-md border border-[var(--border)] px-1.5 py-0 text-micro font-semibold text-[var(--text-muted)]">
              확신 {p.conviction}
            </span>
          )}
          {p.scoreShift !== undefined && Math.abs(p.scoreShift) >= 5 && (
            <span className="shrink-0 rounded-md bg-amber-500/15 px-1.5 py-0 text-micro font-semibold text-amber-600 dark:text-amber-400">
              점수 {p.scoreThen}→{p.scoreNow}
            </span>
          )}
        </span>

        <span className="mt-0.5 flex flex-wrap items-center gap-x-2 text-micro text-[var(--text-muted)]">
          <span>
            {formatDateKST(p.createdAt)} 등록 · {horizonLabel(p)}
            {p.dueAt && <> · {formatDateKST(p.dueAt)} 만기</>}
          </span>
        </span>

        {/* 축이 접히는 폭에서는 계획 문장이 대신 선다 — 목표가가 행에서 사라지면 안 된다. */}
        <PlanLine p={p} className="mt-0.5 block text-micro text-[var(--text-secondary)] @2xl:hidden" />

        {p.note && (
          <span className="mt-0.5 block truncate text-micro text-[var(--text-secondary)]">
            “{p.note}”
          </span>
        )}
      </span>

      <span className="hidden w-[300px] shrink-0 @2xl:block @4xl:w-[330px]">
        <PredictionAxis prediction={p} />
      </span>

      <span className="hidden w-[78px] shrink-0 text-right @lg:block">
        <span className="block text-micro text-[var(--text-muted)]">등록 후</span>
        <span
          className={cn(
            "block text-note font-bold tabular-nums",
            toneForValue(p.sinceBasePct),
          )}
        >
          {p.sinceBasePct === undefined ? "—" : formatPercent(p.sinceBasePct)}
        </span>
        <span className="block text-micro tabular-nums text-[var(--text-muted)]">
          {p.alpha === undefined ? "알파 —" : `알파 ${formatPercent(p.alpha)}`}
        </span>
      </span>

      <span className="w-[68px] shrink-0 text-right">
        <span className="block text-micro text-[var(--text-muted)]">목표까지</span>
        <span className={cn("block text-note font-bold tabular-nums", prog.tone)}>
          {p.status === "reached"
            ? "도달"
            : p.toTargetPct === undefined
              ? "—"
              : formatPercent(p.toTargetPct)}
        </span>
        <span className="block text-micro tabular-nums text-[var(--text-muted)]">
          {p.targetAccuracy === undefined
            ? prog.text
            : `오차 ${p.targetAccuracy.toFixed(1)}%`}
        </span>
      </span>
    </button>
  );
}
