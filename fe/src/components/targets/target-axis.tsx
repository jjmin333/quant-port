"use client";

import { formatPrice } from "@/lib/format";
import type { TargetView } from "@/lib/target-stats";
import { cn } from "@/lib/utils";

/**
 * 「등록가 — 현재가 — 목표가」를 한 축에 올린 게이지. 손절이 있으면 그것도 같은 축에.
 *
 * 예전 행은 `+11.4%`라는 숫자 하나였는데, 그게 가까운 건지 먼 건지가 안 읽혔다.
 * 좌표로 세우면 눈이 거리를 먼저 읽고 숫자는 확인용이 된다.
 *
 * 축의 양 끝은 **실제 가격 범위**로 잡는다(손절·등록가·현재가·목표가 중 최소~최대에
 * 여백 8%). 등록가를 0%, 목표가를 100%로 고정하면 목표를 넘어선 행이 축 밖으로
 * 나가고, 손절이 어디인지도 표시할 자리가 없다.
 */
export function TargetAxis({
  view,
  className,
  showLabels = true,
}: {
  view: TargetView;
  className?: string;
  showLabels?: boolean;
}) {
  const { target, current, status, progress } = view;
  const marks = [target.targetPrice, target.targetPrice2, target.stopPrice, target.basePrice, current]
    .filter((v): v is number => typeof v === "number" && v > 0);

  if (marks.length < 2) return null;

  const min = Math.min(...marks);
  const max = Math.max(...marks);
  const pad = (max - min) * 0.08 || Math.abs(max) * 0.02 || 1;
  const lo = min - pad;
  const hi = max + pad;
  const at = (v: number) => ((v - lo) / (hi - lo)) * 100;

  const basePos = target.basePrice ? at(target.basePrice) : null;
  const nowPos = current !== null ? at(current) : null;
  const targetPos = at(target.targetPrice);
  const target2Pos = target.targetPrice2 ? at(target.targetPrice2) : null;
  const stopPos = target.stopPrice ? at(target.stopPrice) : null;

  // 채움은 등록가에서 현재가까지 — "여기서 시작해 여기까지 왔다".
  const fillFrom = basePos !== null && nowPos !== null ? Math.min(basePos, nowPos) : null;
  const fillTo = basePos !== null && nowPos !== null ? Math.max(basePos, nowPos) : null;
  const gaining = basePos !== null && nowPos !== null && nowPos >= basePos;

  return (
    <div className={cn("min-w-0", className)}>
      <div className="relative mt-3.5 h-1.5 rounded-full bg-[var(--surface-elevated)]">
        {fillFrom !== null && fillTo !== null && (
          <span
            className={cn(
              "absolute inset-y-0 rounded-full",
              status === "reached"
                ? "bg-emerald-500/70"
                : gaining
                  ? "bg-[var(--brand)]/70"
                  : "bg-[var(--loss)]/60",
            )}
            style={{ left: `${fillFrom}%`, width: `${Math.max(fillTo - fillFrom, 0.5)}%` }}
          />
        )}

        {stopPos !== null && (
          <span
            aria-hidden
            title={`손절 ${formatPrice(target.stopPrice as number, target.currency)}`}
            className="absolute -top-1 h-3.5 w-0 border-r-2 border-dashed border-[var(--loss)]"
            style={{ left: `${stopPos}%` }}
          />
        )}
        {targetPos !== null && (
          <span
            aria-hidden
            title={`목표 ${formatPrice(target.targetPrice, target.currency)}`}
            className="absolute -top-1 h-3.5 w-0 border-r-2 border-dashed border-[var(--profit)]"
            style={{ left: `${targetPos}%` }}
          />
        )}
        {target2Pos !== null && (
          <span
            aria-hidden
            title={`2차 목표 ${formatPrice(target.targetPrice2 as number, target.currency)}`}
            className="absolute -top-1 h-3.5 w-0 border-r-2 border-dotted border-[var(--profit)] opacity-60"
            style={{ left: `${target2Pos}%` }}
          />
        )}
        {basePos !== null && (
          <span
            aria-hidden
            title={`등록 ${formatPrice(target.basePrice as number, target.currency)}`}
            className="absolute -top-0.5 h-2.5 w-0.5 rounded-full bg-[var(--text-muted)]"
            style={{ left: `${basePos}%` }}
          />
        )}
        {nowPos !== null && (
          <span
            aria-hidden
            title={`현재 ${formatPrice(current as number, target.currency)}`}
            className="absolute -top-1.5 h-4.5 w-[3px] rounded-full bg-[var(--text-primary)]"
            style={{ left: `${nowPos}%` }}
          />
        )}
      </div>

      {showLabels && (
        <div className="mt-1 flex items-center justify-between gap-2 text-[10px] text-[var(--text-muted)]">
          <span className="truncate">
            {target.basePrice
              ? `등록 ${formatPrice(target.basePrice, target.currency)}`
              : "등록가 없음"}
            {progress !== null && ` · 진행 ${Math.round(progress * 100)}%`}
          </span>
          <span className="shrink-0 font-mono tabular-nums text-[var(--profit)]">
            목표 {formatPrice(target.targetPrice, target.currency)}
          </span>
        </div>
      )}
    </div>
  );
}
