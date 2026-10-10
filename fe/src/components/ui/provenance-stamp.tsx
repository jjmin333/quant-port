"use client";

import { RotateCw } from "lucide-react";

import { hasProvenance, type ProvenanceParts } from "@/lib/data-reliability";
import { cn } from "@/lib/utils";

/**
 * 기준시각 · 출처 도장 (V2 요구 8).
 *
 * 카드 머리나 꼬리에 **한 줄**. 값마다 달지 않습니다 — 카드 안의 값은 거의 언제나
 * 같은 출처·같은 기준시각에서 오므로, 값마다 달면 같은 문구가 표 전체에 반복됩니다.
 *
 * 조각이 하나도 없으면 아무것도 그리지 않습니다. 빈 도장은 테두리만 남습니다.
 *
 * 이미 이 모양을 쓰는 자리가 있습니다 — 「09-18 종가 기준」(시나리오 카드)·
 * 「2026-09-17 기준」(랭킹). 새 문구를 만드는 것이 아니라 **같은 모양으로 맞추는**
 * 일입니다.
 */
export function ProvenanceStamp({
  parts,
  className,
}: {
  parts: ProvenanceParts;
  className?: string;
}) {
  if (!hasProvenance(parts)) return null;
  return (
    <span
      className={cn(
        "inline-flex items-center gap-1 rounded-full border border-[var(--border)]",
        "px-2 py-0.5 text-micro text-[var(--text-muted)]",
        className,
      )}
    >
      {parts.basis ? (
        <span>
          기준 <b className="font-semibold text-[var(--text-primary)]">{parts.basis}</b>
        </span>
      ) : null}
      {parts.basis && parts.source ? <span aria-hidden>·</span> : null}
      {parts.source ? (
        <span>
          출처 <b className="font-semibold text-[var(--text-primary)]">{parts.source}</b>
        </span>
      ) : null}
      {parts.caveat ? (
        <>
          <span aria-hidden>·</span>
          <b className="font-semibold text-[var(--score-mid)]">{parts.caveat}</b>
        </>
      ) : null}
    </span>
  );
}

/**
 * 불러오지 못한 값 — **유일하게 사용자가 할 일이 있는 상태**입니다.
 *
 * 그래서 유일하게 누를 것을 답니다. 「데이터 없음」(그냥 「—」)과 **다른 모양**이어야
 * 사용자가 다시 눌러 볼 생각을 합니다. 지금은 화면 코드 98곳이 오류를 조용히 빈 값으로
 * 바꾸고 있어, 우리가 고장 난 것을 사용자가 「원래 없는 것」으로 읽습니다.
 */
export function RetryMark({
  onRetry,
  label = "다시",
  className,
}: {
  onRetry: () => void;
  label?: string;
  className?: string;
}) {
  return (
    <span className={cn("inline-flex items-baseline gap-1", className)}>
      <span className="text-[var(--text-muted)]" aria-hidden>
        —
      </span>
      <button
        type="button"
        onClick={onRetry}
        className={cn(
          "inline-flex items-center gap-0.5 rounded border border-[var(--score-low)]",
          "px-1 text-micro font-semibold text-[var(--score-low)]",
          "hover:bg-[var(--score-low)]/10",
        )}
      >
        <RotateCw className="h-2.5 w-2.5" aria-hidden />
        {label}
      </button>
      <span className="sr-only">불러오지 못했습니다. 다시 시도할 수 있습니다.</span>
    </span>
  );
}
