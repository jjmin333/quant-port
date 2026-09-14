"use client";

import { useMemo } from "react";

import { StockAvatar } from "@/components/stocks/stock-avatar";
import { formatPercent } from "@/lib/format";
import type { Prediction, PredictionSummary } from "@/lib/prediction-types";
import { cn } from "@/lib/utils";

/**
 * 화면 맨 위 두 위젯 — 「전체가 지금 어디에 있나」와 「전체적으로 어땠나」.
 *
 * 목록은 한 번에 대여섯 건만 보인다. 예측이 스무 건이 되면 「대체로 잘 가고 있나」에
 * 아무도 대답하지 못한다. 조망 스트립은 그 질문 하나만 맡는다.
 */

/* ─────────────────────── C3 · 전체 조망 ─────────────────────── */

// 축은 진척률(%)이다. 손절 쪽으로 60%까지, 목표 너머로 30%까지 본다 — 그 밖은
// 끝에 붙여 둔다. 실제 최소·최대로 잡으면 예측 하나가 -800%일 때 나머지가 전부
// 한 점에 뭉친다.
const LO = -60;
const HI = 130;
const SPAN = HI - LO;

function xOf(progress: number): number {
  const clamped = Math.max(LO, Math.min(HI, progress));
  return 3 + ((clamped - LO) / SPAN) * 94;
}

const ZERO_X = xOf(0);
const TARGET_X = xOf(100);

export function PredictionSpread({
  predictions,
  onSelect,
  className,
}: {
  predictions: Prediction[];
  onSelect: (p: Prediction) => void;
  className?: string;
}) {
  // 진척률을 못 세는 예측(등록가나 현재가가 없는 것)은 축에 올리지 않는다.
  // 0으로 두면 「등록가에 그대로 있다」는 거짓이 된다.
  const { points, skipped } = useMemo(() => {
    const usable = predictions.filter((p) => p.progress !== undefined);
    const sorted = [...usable].sort((a, b) => (a.progress as number) - (b.progress as number));

    // 가까이 붙은 점은 위아래로 엇갈리게 둔다. 375px에서 여섯 건이면 반드시 겹친다.
    // 누적값을 클로저 변수로 들면 렌더가 끝난 뒤에도 남으므로 reduce로 흘린다.
    const laid = sorted.reduce<{ p: Prediction; x: number; lane: number }[]>((acc, p) => {
      const x = xOf(p.progress as number);
      const prev = acc[acc.length - 1];
      const lane = prev && x - prev.x < 9 ? (prev.lane + 1) % 2 : 0;
      acc.push({ p, x, lane });
      return acc;
    }, []);
    return { points: laid, skipped: predictions.length - usable.length };
  }, [predictions]);

  if (points.length === 0) return null;

  const reachedSide = points.filter((pt) => (pt.p.progress as number) >= 70).length;
  const belowZero = points.filter((pt) => (pt.p.progress as number) < 0).length;

  return (
    <div className={cn("min-w-0", className)}>
      <div className="relative h-[86px] w-full">
        {/* 구간 음영 — 손절 쪽과 목표 쪽 */}
        <span
          aria-hidden
          className="absolute top-[34px] h-4 rounded bg-[var(--loss)]/10"
          style={{ left: 0, width: `${ZERO_X}%` }}
        />
        <span
          aria-hidden
          className="absolute top-[34px] h-4 rounded bg-emerald-500/10"
          style={{ left: `${TARGET_X}%`, right: 0 }}
        />
        <span aria-hidden className="absolute inset-x-0 top-[41px] h-0.5 rounded bg-[var(--surface-elevated)]" />
        <span
          aria-hidden
          className="absolute top-[26px] h-8 w-px bg-[var(--text-muted)] opacity-60"
          style={{ left: `${ZERO_X}%` }}
        />
        <span
          aria-hidden
          className="absolute top-[26px] h-8 w-px bg-emerald-500 opacity-70"
          style={{ left: `${TARGET_X}%` }}
        />

        <span
          className="absolute top-[62px] -translate-x-1/2 whitespace-nowrap text-micro text-[var(--text-muted)]"
          style={{ left: `${ZERO_X}%` }}
        >
          등록가
        </span>
        <span
          className="absolute top-[62px] -translate-x-1/2 whitespace-nowrap text-micro text-emerald-600 dark:text-emerald-400"
          style={{ left: `${TARGET_X}%` }}
        >
          목표
        </span>

        {points.map(({ p, x, lane }) => (
          <button
            key={p.key}
            type="button"
            onClick={() => onSelect(p)}
            title={`${p.symbolName} · ${
              p.progress === undefined ? "" : `목표의 ${Math.round(p.progress)}%`
            }`}
            aria-label={`${p.symbolName} 예측 열기`}
            className="absolute -translate-x-1/2 rounded-full ring-offset-1 ring-offset-[var(--surface)] transition-transform hover:z-10 hover:scale-110 focus-visible:ring-2 focus-visible:ring-[var(--brand)]"
            style={{ left: `${x}%`, top: lane === 0 ? 30 : 6 }}
          >
            <StockAvatar symbol={p.symbol} name={p.symbolName} size="xs" />
          </button>
        ))}
      </div>

      <p className="mt-1 text-micro leading-relaxed text-[var(--text-muted)]">
        {points.length}건 중{" "}
        <b className="text-emerald-600 dark:text-emerald-400">{reachedSide}건</b>이 목표 근처에
        있고,{" "}
        <b className={belowZero > 0 ? "text-[var(--loss)]" : "text-[var(--text-secondary)]"}>
          {belowZero}건
        </b>
        이 등록가보다 반대로 가 있습니다.
        {skipped > 0 && <> 시세를 못 받은 {skipped}건은 축에 올리지 않았습니다.</>}
      </p>
    </div>
  );
}

/* ─────────────────────── D1 · 성적 요약 ─────────────────────── */

/**
 * **적중률과 도달률을 섞지 않는다.** 무기한 예측은 기다리면 언젠가 닿을 수 있어
 * 같은 분모에 넣으면 숫자가 부풀어 오른다. 왼쪽이 채점, 오른쪽이 추적이다.
 */
export function PredictionScoreboard({
  summary,
  onOpenScorecard,
  className,
}: {
  summary: PredictionSummary;
  onOpenScorecard: () => void;
  className?: string;
}) {
  const pct = summary.hitRate ?? 0;

  return (
    <div className={cn("flex flex-wrap items-center gap-x-4 gap-y-3", className)}>
      <button
        type="button"
        onClick={onOpenScorecard}
        className="flex min-w-0 items-center gap-3 rounded-lg text-left focus-visible:outline focus-visible:outline-2 focus-visible:outline-[var(--brand)]"
      >
        <span
          className="relative grid size-[58px] shrink-0 place-items-center rounded-full"
          style={{
            background:
              summary.hitRate === undefined
                ? "var(--surface-elevated)"
                : `conic-gradient(var(--profit) 0 ${pct}%, var(--surface-elevated) ${pct}% 100%)`,
          }}
          role="img"
          aria-label={
            summary.hitRate === undefined
              ? "적중률 없음"
              : `적중률 ${Math.round(pct)}퍼센트`
          }
        >
          <span className="absolute inset-[7px] rounded-full bg-[var(--surface)]" />
          <span className="relative text-note font-bold text-[var(--text-primary)]">
            {summary.hitRate === undefined ? "—" : `${Math.round(pct)}%`}
          </span>
        </span>
        <span className="min-w-0">
          <span className="block text-cell font-semibold text-[var(--text-primary)]">
            기한 예측 적중률
          </span>
          <span className="block text-micro text-[var(--text-muted)]">
            판정 {summary.judged}건 중 {summary.hits}건 적중
          </span>
          <span className="mt-0.5 block text-micro text-[var(--brand)]">성적표 열기 →</span>
        </span>
      </button>

      <dl className="flex min-w-0 flex-wrap gap-x-4 gap-y-2">
        <Stat
          label="평균 알파"
          value={summary.avgAlpha === undefined ? "—" : formatPercent(summary.avgAlpha)}
          tone={
            summary.avgAlpha === undefined
              ? undefined
              : summary.avgAlpha > 0
                ? "text-[var(--profit)]"
                : summary.avgAlpha < 0
                  ? "text-[var(--loss)]"
                  : undefined
          }
        />
        <Stat
          label="목표가 오차"
          value={
            summary.avgTargetError === undefined
              ? "—"
              : `${summary.avgTargetError.toFixed(1)}%`
          }
        />
        <Stat
          label="무기한 도달"
          value={
            summary.undated === 0
              ? "—"
              : `${summary.undatedReached} / ${summary.undated}`
          }
          hint="채점 없이 추적만"
        />
        <Stat
          label="예측"
          value={`${summary.total}건`}
          hint={`${summary.symbols}개 종목 · 기한 ${summary.dated} · 무기한 ${summary.undated}`}
        />
      </dl>
    </div>
  );
}

function Stat({
  label,
  value,
  tone,
  hint,
}: {
  label: string;
  value: string;
  tone?: string;
  hint?: string;
}) {
  return (
    <div className="min-w-0">
      <dt className="text-micro text-[var(--text-muted)]">{label}</dt>
      <dd className={cn("text-lg font-bold leading-tight tabular-nums", tone)}>{value}</dd>
      {hint && <p className="truncate text-micro text-[var(--text-muted)]">{hint}</p>}
    </div>
  );
}
