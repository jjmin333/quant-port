"use client";

import { useMemo } from "react";

import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { formatPercent } from "@/lib/format";
import {
  habitStats,
  type HabitBucket,
  type Prediction,
  type PredictionSummary,
} from "@/lib/prediction-types";
import { cn } from "@/lib/utils";

/**
 * 성적표 — 「어떤 종류의 판단에서 틀리는가」.
 *
 * 「내 적중률 50%」는 아무 행동도 바꾸지 못한다. 사용자가 알고 싶은 것은 확신이
 * 실제로 맞는지, 어떤 기한에서 약한지, 목표를 얼마나 높게 잡는 버릇이 있는지다.
 * 재료는 전부 이미 저장돼 있다.
 *
 * **표본이 적으면 적다고 말한다.** 판정 3건 미만인 칸은 점선으로 두고 결론 문장을
 * 쓰지 않는다. 1/1을 100%로 그리면 그건 데이터가 아니라 점(占)이다.
 */

const MIN_SAMPLES = 3;

export function PredictionScorecardModal({
  open,
  onOpenChange,
  predictions,
  summary,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  predictions: Prediction[];
  summary: PredictionSummary;
}) {
  const habits = useMemo(() => habitStats(predictions), [predictions]);

  const settled = useMemo(
    () =>
      predictions
        .filter(
          (p) =>
            (p.verdict === "hit" || p.verdict === "miss") &&
            typeof p.expectedReturn === "number" &&
            typeof p.horizonReturn === "number",
        )
        .sort((a, b) => (a.targetAccuracy ?? 0) - (b.targetAccuracy ?? 0)),
    [predictions],
  );

  const biasReliable = habits.biasSamples >= MIN_SAMPLES;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[88vh] overflow-y-auto sm:max-w-[680px]">
        <DialogHeader>
          <DialogTitle>내 예측 성적표</DialogTitle>
          <DialogDescription>
            기한이 있는 예측만 채점됩니다. 무기한 예측은 도달 여부만 추적합니다.
          </DialogDescription>
        </DialogHeader>

        {summary.judged === 0 ? (
          <p className="rounded-lg border border-dashed border-[var(--border)] px-3 py-6 text-center text-cell text-[var(--text-muted)]">
            아직 채점된 예측이 없습니다. 기한이 지나면 여기에 쌓입니다.
          </p>
        ) : (
          <div className="space-y-4">
            {/* 한 줄 진단 */}
            <div className="rounded-xl border border-[var(--border)] bg-[var(--surface-elevated)] px-3 py-2.5">
              <p className="text-note leading-relaxed">
                판정된 <b>{summary.judged}건</b> 중 <b>{summary.hits}건</b>을 맞췄습니다
                {summary.hitRate !== undefined && <> ({Math.round(summary.hitRate)}%)</>}.
                {biasReliable && habits.targetBiasPp !== undefined && (
                  <>
                    {" "}
                    다만 목표가는{" "}
                    <b className="text-amber-600 dark:text-amber-400">
                      평균 {Math.abs(habits.targetBiasPp).toFixed(1)}%p{" "}
                      {habits.targetBiasPp > 0 ? "높게" : "낮게"}
                    </b>{" "}
                    잡는 편입니다.
                  </>
                )}
              </p>
              {!biasReliable && (
                <p className="mt-1 text-micro text-[var(--text-muted)]">
                  목표가 버릇은 상방 예측 {MIN_SAMPLES}건부터 문장으로 말합니다 (지금{" "}
                  {habits.biasSamples}건).
                </p>
              )}
            </div>

            {/* D2 · 목표가 버릇 */}
            {settled.length > 0 && (
              <section>
                <h3 className="mb-2 text-note font-semibold">
                  목표가를 얼마나 정확히 잡습니까
                  <span className="ml-1.5 text-micro font-normal text-[var(--text-muted)]">
                    예상 ◇ 대비 실제 ●
                  </span>
                </h3>
                <TargetBias items={settled} />
              </section>
            )}

            {/* D3 · 판단 습관 */}
            <section className="grid gap-3 sm:grid-cols-2">
              <HabitCard title="확신이 실제로 맞습니까" buckets={habits.conviction} />
              <div className="space-y-3">
                {habits.horizon.length > 0 && (
                  <HabitCard title="어떤 기한에서 약합니까" buckets={habits.horizon} />
                )}
                <HabitCard
                  title="방향별"
                  buckets={habits.direction}
                  footnote={
                    habits.direction[1].judged <= 1
                      ? "하방 예측이 거의 없습니다. 오르는 쪽만 찍고 있다면 적중률이 시장 방향에 딸려 갑니다."
                      : undefined
                  }
                />
              </div>
            </section>

            <p className="text-micro leading-relaxed text-[var(--text-muted)]">
              이 화면은 <b className="text-[var(--text-secondary)]">본인 판단에 대한 자기 채점</b>
              이지 매매 권유가 아닙니다. 적중·빗나감은 방향으로만 판정하고, 목표가 오차는 별도
              축으로 함께 적습니다 — 방향은 맞고 폭이 틀린 예측이 있기 때문입니다.
            </p>
          </div>
        )}
      </DialogContent>
    </Dialog>
  );
}

/** 예상(◇)과 실제(●)를 한 축에 올린 아령 그림. 둘이 붙어 있을수록 폭까지 맞춘 것. */
function TargetBias({ items }: { items: Prediction[] }) {
  const values = items.flatMap((p) => [p.expectedReturn as number, p.horizonReturn as number]);
  const lo = Math.min(0, ...values);
  const hi = Math.max(0, ...values);
  const span = hi - lo || 1;
  const at = (v: number) => 4 + ((v - lo) / span) * 92;
  const zero = at(0);

  return (
    <div className="space-y-1.5">
      {items.map((p) => {
        const exp = at(p.expectedReturn as number);
        const act = at(p.horizonReturn as number);
        const from = Math.min(exp, act);
        const width = Math.abs(exp - act);
        return (
          <div key={p.key} className="flex items-center gap-2">
            <span className="w-[74px] shrink-0 truncate text-cell font-medium">
              {p.symbolName}
            </span>
            <span className="relative h-6 min-w-0 flex-1">
              <span aria-hidden className="absolute inset-x-0 top-1/2 h-px bg-[var(--border)]" />
              <span
                aria-hidden
                className="absolute top-1/2 h-4 w-px -translate-y-1/2 bg-[var(--text-muted)] opacity-40"
                style={{ left: `${zero}%` }}
              />
              {width > 0.4 && (
                <span
                  aria-hidden
                  className="absolute top-1/2 h-0.5 -translate-y-1/2 bg-amber-500/60"
                  style={{ left: `${from}%`, width: `${width}%` }}
                />
              )}
              <span
                aria-hidden
                className="absolute top-1/2 size-2.5 -translate-x-1/2 -translate-y-1/2 rotate-45 border-2 border-[var(--text-muted)] bg-[var(--surface)]"
                style={{ left: `${exp}%` }}
              />
              <span
                aria-hidden
                className={cn(
                  "absolute top-1/2 size-2.5 -translate-x-1/2 -translate-y-1/2 rounded-full border-2 border-[var(--surface)]",
                  (p.horizonReturn as number) >= 0 ? "bg-[var(--profit)]" : "bg-[var(--loss)]",
                )}
                style={{ left: `${act}%` }}
              />
            </span>
            <span className="w-[132px] shrink-0 text-right text-micro tabular-nums text-[var(--text-muted)] sm:w-[168px]">
              {formatPercent(p.expectedReturn as number)} →{" "}
              <b
                className={
                  (p.horizonReturn as number) >= 0
                    ? "text-[var(--profit)]"
                    : "text-[var(--loss)]"
                }
              >
                {formatPercent(p.horizonReturn as number)}
              </b>
              {p.targetAccuracy !== undefined && (
                <span className="hidden sm:inline"> · 오차 {p.targetAccuracy.toFixed(1)}%</span>
              )}
            </span>
          </div>
        );
      })}
    </div>
  );
}

function HabitCard({
  title,
  buckets,
  footnote,
}: {
  title: string;
  buckets: HabitBucket[];
  footnote?: string;
}) {
  const shown = buckets.filter((b) => b.judged > 0);
  const thin = shown.every((b) => b.judged < MIN_SAMPLES);

  return (
    <div className="rounded-xl border border-[var(--border)] p-3">
      <h4 className="mb-2 text-cell font-semibold">{title}</h4>
      {shown.length === 0 ? (
        <p className="text-micro text-[var(--text-muted)]">아직 판정된 예측이 없습니다.</p>
      ) : (
        <div className="space-y-1">
          {shown.map((b) => {
            const rate = b.judged === 0 ? 0 : (b.hits / b.judged) * 100;
            const enough = b.judged >= MIN_SAMPLES;
            return (
              <div key={b.label} className="flex items-center gap-2">
                <span className="w-[52px] shrink-0 text-micro text-[var(--text-secondary)]">
                  {b.label}
                </span>
                <span className="relative h-3 min-w-0 flex-1 overflow-hidden rounded bg-[var(--surface-elevated)]">
                  {enough ? (
                    <span
                      className="absolute inset-y-0 left-0 rounded bg-[var(--profit)]"
                      style={{ width: `${rate}%` }}
                    />
                  ) : (
                    <>
                      <span
                        className="absolute inset-y-0 left-0 rounded bg-[var(--text-muted)]/30"
                        style={{ width: `${rate}%` }}
                      />
                      <span className="absolute inset-0 rounded border border-dashed border-[var(--border)]" />
                    </>
                  )}
                </span>
                <span className="w-[62px] shrink-0 text-right text-micro tabular-nums">
                  <b>
                    {b.hits}/{b.judged}
                  </b>
                  {enough && (
                    <span className="ml-1 text-[var(--text-muted)]">{Math.round(rate)}%</span>
                  )}
                </span>
              </div>
            );
          })}
        </div>
      )}
      {thin && shown.length > 0 && (
        <p className="mt-2 rounded-md bg-amber-500/10 px-2 py-1.5 text-micro leading-relaxed text-amber-700 dark:text-amber-400">
          각 칸의 표본이 {MIN_SAMPLES}건 미만입니다. 방향이 보이더라도 아직{" "}
          <b>결론을 쓰지 않습니다</b>.
        </p>
      )}
      {footnote && !thin && (
        <p className="mt-2 text-micro leading-relaxed text-[var(--text-muted)]">{footnote}</p>
      )}
    </div>
  );
}
