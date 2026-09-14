"use client";

import { StockAvatar } from "@/components/stocks/stock-avatar";
import { PriceDisplay } from "@/components/ui/price-display";
import { formatDateKST, formatPercent } from "@/lib/format";
import type { Prediction } from "@/lib/prediction-types";
import { cn } from "@/lib/utils";
import {
  PredictionDday,
  PredictionGauge,
  PredictionSeal,
  PredictionStatusBadge,
  horizonLabel,
  progressLabel,
  toneForValue,
} from "@/components/predictions/prediction-bits";

/**
 * 예측 한 건의 카드.
 *
 * 목록 행은 375px에서 성립하지 않는다 — 축도 게이지도 숫자 세 칸도 들어갈 폭이
 * 없다. 좁은 화면과 「개봉 대기」 덱이 이 카드를 쓴다.
 *
 * `sealed`면 **결과를 가린다.** 만기가 지난 예측은 결과를 먼저 보여주는 순간
 * 개봉이 사건이 아니라 숫자 확인이 된다 — 그때의 내 말을 먼저 읽게 두고, 누르면
 * 채점을 편다. 목록 쪽에서는 가리지 않는다(이미 아는 것을 또 누르게 하면 방해다).
 */
export function PredictionCard({
  prediction,
  sealed = false,
  onOpen,
  onReveal,
  className,
}: {
  prediction: Prediction;
  sealed?: boolean;
  onOpen: () => void;
  onReveal?: () => void;
  className?: string;
}) {
  const p = prediction;
  const prog = progressLabel(p);

  return (
    <div
      className={cn(
        "flex min-w-0 flex-col gap-2 rounded-xl border border-[var(--border)] bg-[var(--surface)] p-3",
        sealed && "border-[var(--brand)]/45",
        className,
      )}
    >
      <div className="flex items-center gap-2">
        <StockAvatar symbol={p.symbol} name={p.symbolName} size="xs" />
        <div className="min-w-0 flex-1">
          <p className="truncate text-note font-semibold text-[var(--text-primary)]">
            {p.symbolName}
          </p>
          <p className="text-micro text-[var(--text-muted)]">
            {formatDateKST(p.createdAt)} 등록 · {horizonLabel(p)}
            {p.conviction !== undefined && <> · 확신 {p.conviction}</>}
          </p>
        </div>
        {sealed ? <PredictionDday prediction={p} /> : <PredictionStatusBadge status={p.status} />}
      </div>

      <p className="text-note tabular-nums">
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
          <span className="ml-1 text-micro text-[var(--text-muted)]">
            ({formatPercent(p.expectedReturn)}
            {!p.upside && " · 하방"})
          </span>
        )}
      </p>

      {sealed ? (
        <p className="rounded-lg border border-dashed border-[var(--border)] px-2.5 py-1.5 text-micro text-[var(--text-muted)]">
          결과는 열어야 보입니다 — 그때 뭐라고 했는지 먼저 읽어보세요.
        </p>
      ) : (
        <>
          <PredictionGauge prediction={p} />
          <div className="flex flex-wrap items-baseline gap-x-3 gap-y-0.5 text-micro">
            <span className="text-[var(--text-muted)]">
              현재{" "}
              <b className="tabular-nums text-[var(--text-primary)]">
                {p.currentPrice === undefined ? (
                  "—"
                ) : (
                  <PriceDisplay value={p.currentPrice} currency={p.currency} size="inherit" />
                )}
              </b>
            </span>
            <span className={cn("font-semibold tabular-nums", toneForValue(p.sinceBasePct))}>
              {p.sinceBasePct === undefined ? "—" : formatPercent(p.sinceBasePct)}
            </span>
            <span className={cn("font-semibold tabular-nums", prog.tone)}>{prog.text}</span>
          </div>
        </>
      )}

      {p.note && (
        <p className="line-clamp-2 border-l-2 border-[var(--border)] pl-2 text-micro text-[var(--text-secondary)]">
          “{p.note}”
        </p>
      )}

      {sealed && p.held && (
        <p className="text-micro text-amber-600 dark:text-amber-400">
          보유 중인 종목입니다
          {p.holdingAvgPrice !== undefined && (
            <>
              {" · 평단 "}
              <PriceDisplay value={p.holdingAvgPrice} currency={p.currency} size="inherit" />
            </>
          )}
        </p>
      )}

      <div className="mt-auto flex gap-1.5 pt-0.5">
        {sealed && onReveal ? (
          <button
            type="button"
            onClick={onReveal}
            className="flex-1 rounded-md bg-[var(--brand)] px-2.5 py-1.5 text-cell font-medium text-white hover:opacity-90"
          >
            열어서 채점 보기
          </button>
        ) : (
          <button
            type="button"
            onClick={onOpen}
            className="flex-1 rounded-md border border-[var(--border)] px-2.5 py-1.5 text-cell font-medium text-[var(--text-secondary)] hover:bg-[var(--surface-elevated)]"
          >
            자세히
          </button>
        )}
      </div>
    </div>
  );
}

/** 개봉을 끝낸 직후 그 자리에 펴지는 채점 결과. 카드를 뒤집은 뒷면이다. */
export function PredictionVerdictCard({
  prediction,
  onOpen,
  className,
}: {
  prediction: Prediction;
  onOpen: () => void;
  className?: string;
}) {
  const p = prediction;
  /*
   * 연 뒤에도 실링이 「개봉」이라고 적혀 있으면 방금 한 행동이 반영되지 않은 것처럼
   * 보인다. 실제 판정이 있으면 그것을 도장으로 찍는다 — 예시 모드는 서버에
   * `opened`를 기록하지 않아 원본 상태가 `due`로 남기 때문이기도 하다.
   */
  const sealed: Prediction =
    p.verdict === "hit" || p.verdict === "miss" ? { ...p, status: p.verdict } : p;

  return (
    <div
      className={cn(
        "flex min-w-0 flex-col gap-2 rounded-xl border border-[var(--brand)]/45 bg-[var(--brand-muted)] p-3",
        className,
      )}
    >
      <div className="flex items-center gap-2.5">
        <PredictionSeal prediction={sealed} />
        <div className="min-w-0 flex-1">
          <p
            className={cn(
              "text-lg font-bold tabular-nums leading-tight",
              toneForValue(p.horizonReturn ?? p.sinceBasePct),
            )}
          >
            {p.horizonReturn === undefined
              ? p.sinceBasePct === undefined
                ? "—"
                : formatPercent(p.sinceBasePct)
              : formatPercent(p.horizonReturn)}
          </p>
          <p className="truncate text-micro text-[var(--text-muted)]">
            {p.symbolName} · 목표{" "}
            <PriceDisplay value={p.targetPrice} currency={p.currency} size="inherit" />
          </p>
        </div>
      </div>

      <dl className="grid grid-cols-2 gap-x-3 gap-y-1">
        <Cell label="벤치마크 대비" value={p.alpha === undefined ? "—" : formatPercent(p.alpha)} tone={toneForValue(p.alpha)} />
        <Cell
          label="목표가 오차"
          value={p.targetAccuracy === undefined ? "—" : `${p.targetAccuracy.toFixed(1)}%`}
        />
        <Cell
          label="최고 / 최저"
          value={
            p.peakReturn === undefined && p.troughReturn === undefined
              ? "—"
              : `${p.peakReturn === undefined ? "—" : formatPercent(p.peakReturn)} / ${
                  p.troughReturn === undefined ? "—" : formatPercent(p.troughReturn)
                }`
          }
        />
        <Cell
          label="종합점수"
          value={
            p.scoreThen === undefined
              ? "—"
              : `${p.scoreThen} → ${p.scoreNow ?? "—"}`
          }
          tone={toneForValue(p.scoreShift)}
        />
      </dl>

      <button
        type="button"
        onClick={onOpen}
        className="mt-auto rounded-md border border-[var(--border)] bg-[var(--surface)] px-2.5 py-1.5 text-cell font-medium text-[var(--text-secondary)] hover:bg-[var(--surface-elevated)]"
      >
        그때와 지금 대조
      </button>
    </div>
  );
}

function Cell({ label, value, tone }: { label: string; value: string; tone?: string }) {
  return (
    <div className="min-w-0">
      <dt className="text-micro text-[var(--text-muted)]">{label}</dt>
      <dd className={cn("truncate text-cell font-bold tabular-nums", tone)}>{value}</dd>
    </div>
  );
}
