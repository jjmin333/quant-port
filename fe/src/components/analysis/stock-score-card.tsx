"use client";

import { Card } from "@/components/ui/card";
import {
  SCORE_AXIS_LABELS,
  SCORE_AXIS_ORDER,
  SCORE_TONE_BG as BAR_CLASS,
  SCORE_TONE_TEXT as TEXT_CLASS,
  scoreTone,
  type ScoreAxisId,
  type StockScoreData,
} from "@/lib/stock-score-api";
import { cn } from "@/lib/utils";

function AxisBar({ axis, value }: { axis: ScoreAxisId; value: number | undefined }) {
  const has = typeof value === "number";
  const tone = has ? scoreTone(value) : "mid";
  return (
    <div className="flex items-center gap-2">
      <span className="w-16 shrink-0 text-[11px] text-[var(--text-muted)]">
        {SCORE_AXIS_LABELS[axis]}
      </span>
      <div className="h-1.5 min-w-0 flex-1 overflow-hidden rounded-full bg-[var(--surface-elevated)]">
        {has && (
          <div
            className={cn("h-full rounded-full", BAR_CLASS[tone])}
            style={{ width: `${Math.max(2, Math.min(100, value))}%` }}
          />
        )}
      </div>
      <span
        className={cn(
          "w-8 shrink-0 text-right text-[11px] tabular-nums",
          has ? TEXT_CLASS[tone] : "text-[var(--text-muted)]",
        )}
      >
        {has ? Math.round(value) : "-"}
      </span>
    </div>
  );
}

/**
 * Full 종합 점수 panel for the AI 분석 tab.
 *
 * Never shows the number alone: the axis breakdown, the peer rank and the data
 * coverage are what make a single figure defensible, and a score computed from
 * thin data is explicitly marked rather than quietly presented as equivalent.
 */
export function StockScoreCard({
  score,
  loading,
  className,
}: {
  score: StockScoreData | null;
  loading?: boolean;
  className?: string;
}) {
  if (loading) {
    return (
      <Card className={cn("p-3 text-xs text-[var(--text-muted)]", className)}>점수 불러오는 중…</Card>
    );
  }
  if (!score || score.score == null) return null;

  const tone = scoreTone(score.score);
  const coveragePct = Math.round((score.coverage ?? 0) * 100);
  const thin = coveragePct < 60;

  return (
    <Card className={cn("p-0", className)}>
      <div className="flex flex-wrap items-baseline justify-between gap-2 border-b border-[var(--border)] px-3 py-2.5">
        <h3 className="text-sm font-semibold text-[var(--text-primary)]">종합 점수</h3>
        <span className="text-[10px] text-[var(--text-muted)]">
          서버 산출 · {score.tradeDate}
        </span>
      </div>

      <div className="space-y-3 p-3">
        <div className="flex items-end gap-3">
          <div className="flex items-baseline gap-1.5">
            <span className={cn("text-3xl font-bold tabular-nums", TEXT_CLASS[tone])}>
              {score.score}
            </span>
            <span className="text-sm text-[var(--text-muted)]">/ 100</span>
          </div>
          {score.grade && (
            <span
              className={cn(
                "rounded-md border px-2 py-0.5 text-xs font-semibold",
                TEXT_CLASS[tone],
                "border-current",
              )}
            >
              {score.grade}
            </span>
          )}
          <div className="ml-auto text-right text-[11px] text-[var(--text-muted)]">
            {typeof score.peerPct === "number" && <div>동종 상위 {score.peerPct}%</div>}
            <div>데이터 커버리지 {coveragePct}%</div>
          </div>
        </div>

        <div className="space-y-1.5">
          {SCORE_AXIS_ORDER.map((axis) => (
            <AxisBar key={axis} axis={axis} value={score.breakdown?.[axis]} />
          ))}
        </div>

        {thin && (
          <p className="rounded-md border border-dashed border-[var(--border)] px-2 py-1.5 text-[10px] leading-relaxed text-[var(--text-muted)]">
            사용 가능한 데이터가 {coveragePct}%로 제한적이라 점수를 중립(50) 쪽으로 보정했습니다.
          </p>
        )}

        <p className="text-[10px] leading-relaxed text-[var(--text-muted)]">
          각 지표를 동종 그룹 내 순위 백분위로 환산해 가중 합산합니다. 데이터가 없는 축은 0점이 아니라
          가중치에서 제외됩니다. 투자 자문이 아닙니다.
        </p>
      </div>
    </Card>
  );
}

/**
 * One-axis score for a specific tab (차트=추세, 밸류에이션=가치 …), so each tab
 * carries the part of the score it is actually about.
 */
export function AxisScoreBadge({
  score,
  axis,
  className,
}: {
  score: StockScoreData | null;
  axis: ScoreAxisId;
  className?: string;
}) {
  const value = score?.breakdown?.[axis];
  if (typeof value !== "number") return null;
  const tone = scoreTone(value);

  return (
    <div
      className={cn(
        "flex items-center gap-2 rounded-lg border border-[var(--border)] bg-[var(--surface-elevated)]/40 px-2.5 py-1.5",
        className,
      )}
    >
      <span className="text-[11px] text-[var(--text-muted)]">
        {SCORE_AXIS_LABELS[axis]} 점수
      </span>
      <div className="h-1.5 w-16 overflow-hidden rounded-full bg-[var(--surface)]">
        <div
          className={cn("h-full rounded-full", BAR_CLASS[tone])}
          style={{ width: `${Math.max(2, Math.min(100, value))}%` }}
        />
      </div>
      <span className={cn("text-xs font-semibold tabular-nums", TEXT_CLASS[tone])}>
        {Math.round(value)}
      </span>
      {typeof score?.score === "number" && (
        <span className="text-[10px] text-[var(--text-muted)]">(종합 {score.score})</span>
      )}
    </div>
  );
}
