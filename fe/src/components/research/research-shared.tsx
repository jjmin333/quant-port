"use client";

import { Badge } from "@/components/ui/badge";
import { formatPercent, formatPrice } from "@/lib/format";
import {
  DIRECTION_LABELS,
  STATUS_LABELS,
  statusTone,
  type ResearchDirection,
  type ResearchEvidence,
  type ResearchStatus,
  type ResearchTargetOdds,
} from "@/lib/research-types";
import { cn } from "@/lib/utils";

/**
 * 만기까지 남은 일수. 판정이 끝난 콜은 null.
 *
 * 컴포넌트 밖 함수인 이유: 렌더 도중 시계를 직접 읽으면 같은 입력이 매번 다른 결과를
 * 내는 순수하지 않은 렌더가 된다(스탬프의 `daysUntilDue`와 같은 처리).
 */
export function daysUntilDue(dueAt: string, status: ResearchStatus): number | null {
  if (status !== "open") return null;
  const due = new Date(dueAt).getTime();
  if (Number.isNaN(due)) return null;
  return Math.max(0, Math.ceil((due - Date.now()) / 86_400_000));
}

export function toneClass(value: number | undefined): string {
  if (value === undefined) return "text-[var(--text-muted)]";
  if (value > 0) return "text-[var(--profit)]";
  if (value < 0) return "text-[var(--loss)]";
  return "text-[var(--text-secondary)]";
}

export function DirectionBadge({ direction }: { direction: ResearchDirection }) {
  const variant = direction === "buy" ? "profit" : direction === "sell" ? "loss" : "default";
  return (
    <Badge variant={variant} className="px-1.5 py-0 text-[10px]">
      {DIRECTION_LABELS[direction]}
    </Badge>
  );
}

export function StatusBadge({ status }: { status: ResearchStatus }) {
  const tone = statusTone(status);
  const variant = tone === "positive" ? "profit" : tone === "negative" ? "loss" : "default";
  return (
    <Badge variant={variant} className="px-1.5 py-0 text-[10px]">
      {STATUS_LABELS[status]}
    </Badge>
  );
}

/**
 * Marks a call that runs against our own score. Aggregating these separately is
 * the only way to tell "beat the market" from "agreed with the model".
 */
export function ContrarianBadge() {
  return (
    <Badge variant="warning" className="px-1.5 py-0 text-[10px]">
      역발상
    </Badge>
  );
}

/**
 * The frozen evidence card — what the *server* measured at publish time, shown
 * next to what the author claimed. Nothing here was typed by the author.
 */
export function EvidenceCard({
  evidence,
  score,
  grade,
  currency,
  className,
}: {
  evidence: ResearchEvidence;
  score?: number;
  grade?: string;
  currency: "KRW" | "USD";
  className?: string;
}) {
  const axes: { label: string; value?: number }[] = [
    { label: "가치", value: evidence.breakdown?.valuation },
    { label: "수익성", value: evidence.breakdown?.quality },
    { label: "성장", value: evidence.breakdown?.growth },
    { label: "추세", value: evidence.breakdown?.technical },
    { label: "심리", value: evidence.breakdown?.sentiment },
  ];
  const present = axes.filter((a) => a.value !== undefined && a.value !== null);

  return (
    <div
      className={cn(
        "rounded-xl border border-[var(--border)] bg-[var(--surface)] p-3",
        className,
      )}
    >
      <div className="flex items-baseline justify-between gap-2">
        <p className="text-[11px] font-medium text-[var(--text-muted)]">
          발행 시점 근거 · 서버 측정
        </p>
        {score != null && (
          <p className="text-sm font-semibold text-[var(--text-primary)]">
            {score}점{grade ? ` (${grade})` : ""}
          </p>
        )}
      </div>

      {score == null && (
        <p className="mt-1.5 text-[11px] text-[var(--text-muted)]">
          이 종목은 종합 점수를 산출할 데이터가 부족합니다.
        </p>
      )}

      {present.length > 0 && (
        <div className="mt-2 space-y-1">
          {present.map((axis) => (
            <div key={axis.label} className="flex items-center gap-2">
              <span className="w-10 shrink-0 text-[10px] text-[var(--text-muted)]">
                {axis.label}
              </span>
              <span className="h-1.5 flex-1 overflow-hidden rounded-full bg-[var(--surface-elevated)]">
                <span
                  className="block h-full rounded-full bg-[var(--brand)]"
                  style={{ width: `${Math.max(0, Math.min(100, axis.value ?? 0))}%` }}
                />
              </span>
              <span className="w-6 shrink-0 text-right text-[10px] tabular-nums text-[var(--text-secondary)]">
                {Math.round(axis.value ?? 0)}
              </span>
            </div>
          ))}
        </div>
      )}

      <dl className="mt-2.5 grid grid-cols-2 gap-x-3 gap-y-1 text-[11px]">
        {evidence.peerPct != null && (
          <EvidenceRow label="동종 상위" value={`${evidence.peerPct}%`} />
        )}
        {evidence.coverage != null && evidence.coverage > 0 && (
          <EvidenceRow label="커버리지" value={`${Math.round(evidence.coverage * 100)}%`} />
        )}
        {evidence.per != null && <EvidenceRow label="PER" value={evidence.per.toFixed(1)} />}
        {evidence.pbr != null && <EvidenceRow label="PBR" value={evidence.pbr.toFixed(2)} />}
        {!!evidence.week52High && (
          <EvidenceRow label="52주 최고" value={formatPrice(evidence.week52High, currency)} />
        )}
        {!!evidence.week52Low && (
          <EvidenceRow label="52주 최저" value={formatPrice(evidence.week52Low, currency)} />
        )}
      </dl>
    </div>
  );
}

function EvidenceRow({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex items-baseline justify-between gap-2">
      <dt className="text-[var(--text-muted)]">{label}</dt>
      <dd className="tabular-nums text-[var(--text-secondary)]">{value}</dd>
    </div>
  );
}

/**
 * Measured reachability of the target. It never blocks a target — it only makes
 * the claim's difficulty visible. When history is too short it says so instead
 * of showing a small number that reads like a real probability.
 */
export function TargetOddsNote({ odds }: { odds: ResearchTargetOdds }) {
  if (odds.insufficient) {
    return (
      <p className="text-[11px] text-[var(--text-muted)]">
        이 종목의 저장된 일봉이 {odds.horizonDays}일 구간을 평가하기에 부족합니다 — 도달 확률을
        계산하지 않았습니다.
      </p>
    );
  }
  const probability = odds.probability ?? 0;
  const hard = probability < 10;
  return (
    <div className="space-y-0.5">
      <p className={cn("text-[11px]", hard ? "text-[var(--loss)]" : "text-[var(--text-secondary)]")}>
        과거 {odds.windows.toLocaleString()}개 {odds.horizonDays}일 구간 중{" "}
        <strong>{odds.reached.toLocaleString()}회</strong>가 {formatPercent(odds.requiredPct)} 이상
        움직였습니다 ({probability.toFixed(1)}%)
      </p>
      {odds.medianPct !== undefined && (
        <p className="text-[10px] text-[var(--text-muted)]">
          같은 기간 중앙값 {formatPercent(odds.medianPct)}
          {odds.bestPct !== undefined && ` · 최대 ${formatPercent(odds.bestPct)}`}
          {odds.fromDate && odds.toDate && ` · ${odds.fromDate} ~ ${odds.toDate}`}
        </p>
      )}
    </div>
  );
}

/** Horizon progress bar — "D+142 / 365". */
export function HorizonProgress({
  progress,
  publishedAt,
  dueAt,
  status,
}: {
  progress: number;
  publishedAt: string;
  dueAt: string;
  status: ResearchStatus;
}) {
  const total = Math.max(
    1,
    Math.round((new Date(dueAt).getTime() - new Date(publishedAt).getTime()) / 86_400_000),
  );
  const elapsed = Math.round(total * Math.max(0, Math.min(1, progress)));
  const done = status !== "open";
  return (
    <div className="space-y-1">
      <div className="flex items-center justify-between text-[10px] text-[var(--text-muted)]">
        <span>{done ? "판정 완료" : "판정 진행"}</span>
        <span className="tabular-nums">
          D+{elapsed} / {total}
        </span>
      </div>
      <span className="block h-1.5 overflow-hidden rounded-full bg-[var(--surface-elevated)]">
        <span
          className={cn(
            "block h-full rounded-full",
            done ? "bg-[var(--text-muted)]" : "bg-[var(--brand)]",
          )}
          style={{ width: `${Math.max(2, Math.min(100, progress * 100))}%` }}
        />
      </span>
    </div>
  );
}
