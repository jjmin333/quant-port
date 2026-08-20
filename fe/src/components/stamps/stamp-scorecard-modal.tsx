"use client";

import { useMemo, useState } from "react";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { formatDateKST, formatPercent } from "@/lib/format";
import {
  bestAndWorst,
  byConviction,
  byHorizon,
  byStance,
  cumulativeAlpha,
  horizonAverages,
  isJudged,
  type Bucket,
} from "@/lib/stamp-stats";
import { HORIZON_LABELS, type Stamp, type StampSummary } from "@/lib/stamp-types";
import { cn } from "@/lib/utils";

/**
 * 성적표 — 목록 위 적중률 타일을 누르면 열린다.
 *
 * 타일에는 숫자 하나만 두고 쪼갠 축은 전부 여기로 보냈다. 레일 카드로 만들면 폭이
 * 좁아 축 하나당 카드 하나가 되고, 다섯 개가 세로로 늘어서면 정작 목록을 밀어낸다.
 * 모달은 폭이 넉넉해서 탭 하나로 축을 갈아끼울 수 있다.
 *
 * 계산은 전부 이미 받아둔 스탬프 배열에서 한다 (`lib/stamp-stats.ts`).
 */

type TabId = "stance" | "horizon" | "conviction" | "accuracy";

const TABS: { id: TabId; label: string }[] = [
  { id: "stance", label: "관점별" },
  { id: "horizon", label: "검증기간별" },
  { id: "conviction", label: "확신도별" },
  { id: "accuracy", label: "목표가 정확도" },
];

function toneClass(value: number | undefined): string {
  if (value === undefined) return "text-[var(--text-muted)]";
  if (value > 0) return "text-[var(--profit)]";
  if (value < 0) return "text-[var(--loss)]";
  return "text-[var(--text-secondary)]";
}

export function StampScorecardModal({
  open,
  onOpenChange,
  stamps,
  summary,
  onOpenReveal,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  stamps: Stamp[];
  summary: StampSummary | null;
  onOpenReveal: () => void;
}) {
  const [tab, setTab] = useState<TabId>("stance");

  const stats = useMemo(() => {
    const judged = stamps.filter(isJudged);
    return {
      judged,
      stance: byStance(stamps),
      horizon: byHorizon(stamps).map((b) => ({
        ...b,
        label: HORIZON_LABELS[b.key as keyof typeof HORIZON_LABELS] ?? b.key,
      })),
      conviction: byConviction(stamps).filter((b) => b.judged > 0),
      alpha: cumulativeAlpha(stamps),
      horizons: horizonAverages(stamps),
      ...bestAndWorst(stamps),
    };
  }, [stamps]);

  const hitRate = summary?.hitRate;
  const dueCount = summary?.dueCount ?? 0;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[88vh] overflow-y-auto sm:max-w-[640px]">
        <DialogHeader>
          <DialogTitle>내 스탬프 성적표</DialogTitle>
          <DialogDescription>
            판정 완료 {summary?.judged ?? stats.judged.length}건 · 검증 중{" "}
            {(summary?.total ?? stamps.length) - (summary?.judged ?? stats.judged.length)}건 · 개봉
            대기 {dueCount}건
          </DialogDescription>
        </DialogHeader>

        <div className="grid gap-3 sm:grid-cols-[132px_1fr] sm:items-center">
          <div className="text-center">
            <HitRateRing value={hitRate} />
            <p className="mt-1.5 text-[10.5px] text-[var(--text-muted)]">
              적중 {summary?.hits ?? 0} / 판정 {summary?.judged ?? 0}
            </p>
          </div>
          <div className="grid grid-cols-2 gap-2">
            <Tile
              label="평균 알파"
              value={summary?.avgAlpha === undefined ? "—" : formatPercent(summary.avgAlpha)}
              valueClass={toneClass(summary?.avgAlpha)}
              foot={
                summary?.avgReturn === undefined
                  ? "벤치마크 대비 초과수익"
                  : `내 평균 ${formatPercent(summary.avgReturn)}`
              }
            />
            <Tile
              label="목표가 평균 오차"
              value={
                summary?.avgTargetError === undefined
                  ? "—"
                  : `${summary.avgTargetError.toFixed(1)}%`
              }
              foot={`±3% 이내 ${summary?.withinBand ?? 0}건${
                summary?.earlyHits ? ` · 장중 도달 ${summary.earlyHits}건` : ""
              }`}
            />
            <Tile
              label="최고 성적"
              value={stats.best ? stats.best.symbolName || stats.best.symbol : "—"}
              valueClass="text-sm"
              foot={
                stats.best
                  ? `${formatDateKST(stats.best.stampedAt)} · α ${formatPercent(stats.best.horizon?.alpha ?? 0)}`
                  : "판정된 스탬프가 없습니다"
              }
            />
            <Tile
              label="최악 성적"
              value={stats.worst ? stats.worst.symbolName || stats.worst.symbol : "—"}
              valueClass="text-sm"
              foot={
                stats.worst
                  ? `${formatDateKST(stats.worst.stampedAt)} · α ${formatPercent(stats.worst.horizon?.alpha ?? 0)}`
                  : "판정된 스탬프가 없습니다"
              }
            />
          </div>
        </div>

        <div className="flex gap-0.5 rounded-lg bg-[var(--surface-elevated)] p-0.5">
          {TABS.map((t) => (
            <button
              key={t.id}
              type="button"
              onClick={() => setTab(t.id)}
              className={cn(
                "flex-1 rounded-md px-2 py-1 text-[11px] font-medium transition-colors",
                tab === t.id
                  ? "bg-[var(--surface)] text-[var(--text-primary)] shadow-sm"
                  : "text-[var(--text-muted)] hover:text-[var(--text-secondary)]",
              )}
            >
              {t.label}
            </button>
          ))}
        </div>

        <Panel>
          {tab === "accuracy" ? (
            <AccuracyBreakdown stamps={stamps} />
          ) : (
            <BucketBars
              buckets={
                tab === "stance"
                  ? stats.stance
                  : tab === "horizon"
                    ? stats.horizon
                    : stats.conviction
              }
            />
          )}
        </Panel>

        {stats.alpha.length > 1 && (
          <Panel title={`누적 알파 · 스탬프 순서 ${stats.alpha.length}건`}>
            <CumulativeAlphaChart values={stats.alpha} />
          </Panel>
        )}

        <Panel title="구간별 평균 수익률 — 내 스탬프 vs 벤치마크">
          <HorizonComparison rows={stats.horizons} />
        </Panel>

        <div className="flex items-center gap-2 border-t border-[var(--border)] pt-3">
          <p className="text-[10.5px] text-[var(--text-muted)]">
            매매 신호가 아닙니다 · 적중/빗나감은 방향, 오차는 목표가 기준
          </p>
          {dueCount > 0 && (
            <button
              type="button"
              onClick={() => {
                onOpenChange(false);
                onOpenReveal();
              }}
              className="ml-auto shrink-0 rounded-lg bg-[var(--brand)] px-3 py-1.5 text-[11px] font-medium text-white"
            >
              개봉 대기 {dueCount}건 열기 →
            </button>
          )}
        </div>
      </DialogContent>
    </Dialog>
  );
}

function HitRateRing({ value }: { value?: number }) {
  const pct = value ?? 0;
  return (
    <div
      className="relative mx-auto size-24 rounded-full"
      style={{
        background: `conic-gradient(var(--profit) 0 ${pct}%, var(--surface-elevated) ${pct}% 100%)`,
      }}
      role="img"
      aria-label={value === undefined ? "적중률 없음" : `적중률 ${Math.round(pct)}%`}
    >
      <span className="absolute inset-[9px] rounded-full bg-[var(--surface)]" />
      <span className="absolute inset-0 grid place-items-center text-lg font-bold text-[var(--text-primary)]">
        {value === undefined ? "—" : `${Math.round(pct)}%`}
      </span>
    </div>
  );
}

function Tile({
  label,
  value,
  valueClass,
  foot,
}: {
  label: string;
  value: string;
  valueClass?: string;
  foot: string;
}) {
  return (
    <div className="rounded-lg border border-[var(--border)] p-2.5">
      <p className="text-[10.5px] text-[var(--text-muted)]">{label}</p>
      <p className={cn("mt-0.5 text-lg font-bold tabular-nums", valueClass)}>{value}</p>
      <p className="mt-0.5 truncate text-[10px] text-[var(--text-muted)]">{foot}</p>
    </div>
  );
}

function Panel({ title, children }: { title?: string; children: React.ReactNode }) {
  return (
    <section className="rounded-lg border border-[var(--border)] p-3">
      {title && (
        <h4 className="mb-2 text-[10.5px] font-semibold text-[var(--text-muted)]">{title}</h4>
      )}
      {children}
    </section>
  );
}

function BucketBars({ buckets }: { buckets: Bucket[] }) {
  const shown = buckets.filter((b) => b.judged > 0);
  if (shown.length === 0) {
    return (
      <p className="text-center text-[11px] text-[var(--text-muted)]">
        아직 판정이 끝난 스탬프가 없습니다.
      </p>
    );
  }
  return (
    <ul className="space-y-1.5">
      {shown.map((b) => {
        const rate = b.hitRate ?? 0;
        return (
          <li key={b.key} className="flex items-center gap-2 text-[11px]">
            <span className="w-14 shrink-0 truncate text-[var(--text-muted)]">{b.label}</span>
            <span className="flex h-1.5 min-w-0 flex-1 overflow-hidden rounded-full bg-[var(--surface-elevated)]">
              <span className="block h-full bg-[var(--profit)]" style={{ width: `${rate}%` }} />
              <span
                className="block h-full bg-amber-500/60"
                style={{ width: `${100 - rate}%` }}
              />
            </span>
            <span className="w-24 shrink-0 text-right tabular-nums text-[var(--text-secondary)]">
              {Math.round(rate)}% ({b.hits}/{b.judged})
            </span>
            <span
              className={cn("w-12 shrink-0 text-right tabular-nums", toneClass(b.avgAlpha))}
            >
              {b.avgAlpha === undefined ? "—" : formatPercent(b.avgAlpha)}
            </span>
          </li>
        );
      })}
    </ul>
  );
}

/**
 * 정확도 축은 적중/빗나감이 아니라 "목표가에서 얼마나 벗어났나"의 분포다. 방향은
 * 맞췄지만 폭이 한참 틀린 스탬프가 여기서 드러난다.
 */
function AccuracyBreakdown({ stamps }: { stamps: Stamp[] }) {
  const rows = stamps.filter((s) => s.targetAccuracy !== undefined);
  if (rows.length === 0) {
    return (
      <p className="text-center text-[11px] text-[var(--text-muted)]">
        아직 정산된 스탬프가 없습니다.
      </p>
    );
  }
  const bands = [
    { label: "±3% 이내", test: (v: number) => v <= 3 },
    { label: "3~10%", test: (v: number) => v > 3 && v <= 10 },
    { label: "10~20%", test: (v: number) => v > 10 && v <= 20 },
    { label: "20% 초과", test: (v: number) => v > 20 },
  ];
  return (
    <>
      <ul className="space-y-1.5">
        {bands.map((band) => {
          const n = rows.filter((s) => band.test(s.targetAccuracy as number)).length;
          const pct = (n / rows.length) * 100;
          return (
            <li key={band.label} className="flex items-center gap-2 text-[11px]">
              <span className="w-16 shrink-0 text-[var(--text-muted)]">{band.label}</span>
              <span className="h-1.5 min-w-0 flex-1 overflow-hidden rounded-full bg-[var(--surface-elevated)]">
                <span className="block h-full bg-[var(--brand)]" style={{ width: `${pct}%` }} />
              </span>
              <span className="w-16 shrink-0 text-right tabular-nums text-[var(--text-secondary)]">
                {n}건 {Math.round(pct)}%
              </span>
            </li>
          );
        })}
      </ul>
      <AccuracyFootnote rows={rows} />
    </>
  );
}

/**
 * 정확도 축이 무엇을 말해주는지 한 줄. "0건 있습니다"는 문장이 안 되므로, 해당하는
 * 스탬프가 있을 때만 그 이야기를 하고 없으면 축의 의미만 적는다.
 */
function AccuracyFootnote({ rows }: { rows: Stamp[] }) {
  const wideHits = rows.filter(
    (s) => s.verdict === "hit" && (s.targetAccuracy as number) > 20,
  ).length;
  return (
    <p className="mt-2 border-t border-[var(--border)] pt-2 text-[10.5px] text-[var(--text-muted)]">
      {wideHits > 0
        ? `방향은 맞췄지만 폭이 20% 넘게 빗나간 스탬프가 ${wideHits}건 있습니다 — 적중률과 정확도는 다른 이야기입니다.`
        : "적중/빗나감은 방향으로만 가립니다. 여기 오차는 폭까지 맞췄는지를 따로 보는 축입니다."}
    </p>
  );
}

/** 0선 위아래로 누적 알파를 그린다. 축 눈금은 양 끝 값으로 대신한다. */
function CumulativeAlphaChart({ values }: { values: number[] }) {
  const W = 560;
  const H = 70;
  const min = Math.min(0, ...values);
  const max = Math.max(0, ...values);
  const span = max - min || 1;
  const y = (v: number) => 6 + (H - 12) * (1 - (v - min) / span);
  const x = (i: number) => (values.length === 1 ? W : (i / (values.length - 1)) * W);
  const points = values.map((v, i) => `${x(i)},${y(v)}`).join(" ");
  const last = values[values.length - 1];

  return (
    <>
      <svg viewBox={`0 0 ${W} ${H}`} className="h-[70px] w-full" preserveAspectRatio="none" role="img">
        <line
          x1="0"
          y1={y(0)}
          x2={W}
          y2={y(0)}
          stroke="var(--border)"
          strokeWidth="1"
          vectorEffect="non-scaling-stroke"
        />
        <polyline
          points={points}
          fill="none"
          stroke="var(--brand)"
          strokeWidth="1.8"
          vectorEffect="non-scaling-stroke"
        />
        <circle cx={x(values.length - 1)} cy={y(last)} r="3" fill="var(--brand)" />
      </svg>
      <div className="flex justify-between text-[10px] text-[var(--text-muted)]">
        <span>0%p</span>
        <span className={toneClass(last)}>
          누적 {last > 0 ? "+" : ""}
          {last.toFixed(1)}%p
        </span>
      </div>
    </>
  );
}

function HorizonComparison({
  rows,
}: {
  rows: { horizon: string; mine?: number; benchmark?: number }[];
}) {
  const values = rows.flatMap((r) => [r.mine, r.benchmark]).filter((v): v is number => v !== undefined);
  if (values.length === 0) {
    return (
      <p className="text-center text-[11px] text-[var(--text-muted)]">
        아직 구간 수익률이 없습니다.
      </p>
    );
  }
  const scale = Math.max(...values.map(Math.abs), 1);
  const height = (v?: number) => (v === undefined ? 0 : (Math.abs(v) / scale) * 100);

  return (
    <>
      <div className="grid h-24 grid-cols-4 items-end gap-3">
        {rows.map((r) => (
          <div key={r.horizon} className="flex h-full items-end justify-center gap-1.5">
            <span
              className="w-4 rounded-t-sm bg-[var(--brand)]"
              style={{ height: `${height(r.mine)}%` }}
              title={r.mine === undefined ? "—" : formatPercent(r.mine)}
            />
            <span
              className="w-4 rounded-t-sm bg-[var(--text-muted)]/45"
              style={{ height: `${height(r.benchmark)}%` }}
              title={r.benchmark === undefined ? "—" : formatPercent(r.benchmark)}
            />
          </div>
        ))}
      </div>
      <div className="mt-1 grid grid-cols-4 gap-3 text-center text-[10px] text-[var(--text-muted)]">
        {rows.map((r) => (
          <span key={r.horizon}>
            {r.horizon}
            <span className={cn("ml-1", toneClass(r.mine))}>
              {r.mine === undefined ? "—" : formatPercent(r.mine)}
            </span>
          </span>
        ))}
      </div>
      <div className="mt-2 flex gap-3 text-[10px] text-[var(--text-muted)]">
        <span className="flex items-center gap-1">
          <span className="size-2 rounded-sm bg-[var(--brand)]" />내 스탬프
        </span>
        <span className="flex items-center gap-1">
          <span className="size-2 rounded-sm bg-[var(--text-muted)]/45" />
          벤치마크
        </span>
      </div>
    </>
  );
}
