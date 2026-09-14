"use client";

import { formatPrice } from "@/lib/format";
import { TARGET_STATUS_LABEL, type TargetStatus, type TargetView } from "@/lib/target-stats";
import { cn } from "@/lib/utils";
import { PriceDisplay } from "@/components/ui/price-display";

/** 상태 배지. 목록·카드·레일·모달이 같은 색과 같은 말을 쓴다. */
export function TargetStatusBadge({ status }: { status: TargetStatus }) {
  if (status === "open") return null;
  return (
    <span
      className={cn(
        "shrink-0 rounded-md px-1.5 py-0.5 text-micro font-semibold",
        status === "reached" && "bg-emerald-500/15 text-emerald-500",
        status === "near" && "bg-amber-500/15 text-amber-500",
        status === "stop_near" && "bg-[var(--loss)]/15 text-[var(--loss)]",
      )}
    >
      {TARGET_STATUS_LABEL[status]}
    </span>
  );
}

/** 목표까지 남은 거리 한 칸. 시세가 없으면 "—" — 0%로 그리면 도달했다는 뜻이 된다. */
export function TargetGapCell({ view, className }: { view: TargetView; className?: string }) {
  const { toTargetPct, status } = view;
  return (
    <div className={cn("text-right", className)}>
      <p className="text-micro text-[var(--text-muted)]">목표까지</p>
      <p
        className={cn(
          "font-num text-sm font-semibold tabular-nums",
          status === "reached"
            ? "text-emerald-500"
            : toTargetPct === null
              ? "text-[var(--text-muted)]"
              : "text-[var(--text-secondary)]",
        )}
      >
        {status === "reached"
          ? "도달"
          : toTargetPct === null
            ? "—"
            : `${toTargetPct > 0 ? "+" : ""}${toTargetPct.toFixed(1)}%`}
      </p>
    </div>
  );
}

/**
 * 목표 사다리 — 손절 · 등록가 · 현재가 · 1차 · 2차를 가격 순으로 세운 표.
 *
 * 목표가 칸이 하나뿐일 때는 "여기서 접고, 여기서 절반 팔고, 여기까지 간다"가 전부
 * 메모 필드에 한국어 문장으로 들어가 있었다. 사다리는 그 문장을 좌표로 되돌린다.
 */
export function TargetLadder({ view }: { view: TargetView }) {
  const { target, current, riskReward, toTargetPct, toStopPct } = view;

  const rows: { label: string; price: number; tone: "up" | "down" | "now" | "base" }[] = [];
  if (target.targetPrice2) rows.push({ label: "2차 목표", price: target.targetPrice2, tone: "up" });
  rows.push({ label: "1차 목표", price: target.targetPrice, tone: "up" });
  if (current !== null) rows.push({ label: "현재가", price: current, tone: "now" });
  if (target.basePrice) rows.push({ label: "등록가", price: target.basePrice, tone: "base" });
  if (target.stopPrice) rows.push({ label: "손절가", price: target.stopPrice, tone: "down" });

  if (rows.length < 2) return null;

  const max = Math.max(...rows.map((r) => r.price));

  return (
    <div className="space-y-1.5">
      <div className="space-y-1">
        {rows
          .slice()
          .sort((a, b) => b.price - a.price)
          .map((row) => (
            <div
              key={row.label}
              className="grid grid-cols-[54px_minmax(0,1fr)_auto] items-center gap-2 text-cell"
            >
              <span
                className={cn(
                  row.tone === "now"
                    ? "font-semibold text-[var(--text-primary)]"
                    : "text-[var(--text-muted)]",
                )}
              >
                {row.label}
              </span>
              <span className="h-1.5 rounded-full bg-[var(--surface-elevated)]">
                <span
                  className={cn(
                    "block h-full rounded-full",
                    row.tone === "up" && "bg-[var(--profit)]/50",
                    row.tone === "down" && "bg-[var(--loss)]/60",
                    row.tone === "now" && "bg-[var(--text-primary)]/50",
                    row.tone === "base" && "bg-[var(--text-muted)]/40",
                  )}
                  style={{ width: `${Math.max((row.price / max) * 100, 4)}%` }}
                />
              </span>
              <span
                className={cn(
                  "text-right font-num tabular-nums",
                  row.tone === "up" && "text-[var(--profit)]",
                  row.tone === "down" && "text-[var(--loss)]",
                  row.tone === "now" && "font-semibold text-[var(--text-primary)]",
                  row.tone === "base" && "text-[var(--text-muted)]",
                )}
              >
                <PriceDisplay value={row.price} currency={target.currency} size="inherit" />
              </span>
            </div>
          ))}
      </div>

      {(riskReward !== null || toStopPct !== null) && (
        <div className="space-y-0.5 border-t border-[var(--border)] pt-1.5">
          {riskReward !== null && (
            <div className="flex items-center justify-between text-cell">
              <span className="text-[var(--text-muted)]">손익비 (R:R)</span>
              <span className="font-num font-semibold tabular-nums">
                {riskReward.toFixed(2)} : 1
              </span>
            </div>
          )}
          {toStopPct !== null && (
            <div className="flex items-center justify-between text-cell">
              <span className="text-[var(--text-muted)]">목표까지 / 손절까지</span>
              <span className="font-num tabular-nums">
                <span className="text-[var(--profit)]">
                  {toTargetPct === null
                    ? "—"
                    : `${toTargetPct > 0 ? "+" : ""}${toTargetPct.toFixed(1)}%`}
                </span>
                {" / "}
                <span className="text-[var(--loss)]">{toStopPct.toFixed(1)}%</span>
              </span>
            </div>
          )}
        </div>
      )}
    </div>
  );
}

/** 등록 이후 요약 — 등록가·등락·최고/최저 도달. 등록가가 없으면 아예 그리지 않는다. */
export function TargetSinceRegistered({ view }: { view: TargetView }) {
  const { target, sinceBasePct } = view;
  if (!target.basePrice) return null;

  const peakOfTarget =
    target.peakPrice && target.targetPrice > 0
      ? (target.peakPrice / target.targetPrice) * 100
      : null;

  return (
    <div className="space-y-0.5">
      <Row
        label="등록 시점"
        value={`${formatPrice(target.basePrice, target.currency)}${
          target.baseDate ? ` (${target.baseDate})` : ""
        }`}
      />
      {sinceBasePct !== null && (
        <Row
          label="등록 후 등락"
          value={`${sinceBasePct > 0 ? "+" : ""}${sinceBasePct.toFixed(1)}%`}
          tone={sinceBasePct > 0 ? "up" : sinceBasePct < 0 ? "down" : undefined}
        />
      )}
      {target.peakPrice && (
        <Row
          label="최고 도달"
          value={`${formatPrice(target.peakPrice, target.currency)}${
            peakOfTarget ? ` (목표의 ${Math.round(peakOfTarget)}%)` : ""
          }`}
        />
      )}
      {target.reachedDate && <Row label="목표 도달일" value={target.reachedDate} tone="up" />}
    </div>
  );
}

function Row({
  label,
  value,
  tone,
}: {
  label: string;
  value: string;
  tone?: "up" | "down";
}) {
  return (
    <div className="flex items-center justify-between gap-2 text-cell">
      <span className="shrink-0 text-[var(--text-muted)]">{label}</span>
      <span
        className={cn(
          "min-w-0 truncate text-right font-num tabular-nums",
          tone === "up" && "text-[var(--profit)]",
          tone === "down" && "text-[var(--loss)]",
        )}
      >
        {value}
      </span>
    </div>
  );
}
