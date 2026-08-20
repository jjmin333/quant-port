"use client";

import { formatDateKST } from "@/lib/format";
import type { Stamp } from "@/lib/stamp-types";
import { cn } from "@/lib/utils";

/**
 * 도장 실링 — 스탬프의 판정을 한 눈에 읽히는 시각 은유로 세운다.
 *
 * 목록 행에서 판정을 배지로만 적으면 「강세」·「적중」·「개봉 대기」가 같은 크기의
 * 회색 알약 세 개로 늘어서서, 정작 이 화면이 답해야 할 "맞았나 틀렸나"가 나머지와
 * 같은 무게로 묻힌다. 실링은 행에서 유일하게 큰 요소라 스크롤 중에도 결과가 먼저
 * 눈에 들어온다.
 *
 * 테두리로 상태를 한 겹 더 구분한다 — 파선은 판정이 끝난 것, 실선은 지금 개봉할 수
 * 있는 것, 점선은 아직 세는 중인 것.
 */

export type SealKind = "hit" | "miss" | "due" | "pending" | "unknown";

export function sealKindOf(stamp: Stamp): SealKind {
  if (stamp.due && !stamp.opened) return "due";
  switch (stamp.verdict) {
    case "hit":
      return "hit";
    case "miss":
      return "miss";
    case "unknown":
      return "unknown";
    default:
      return "pending";
  }
}

/** 개봉까지 남은 일수. 0이면 오늘, 음수면 이미 지났다. */
export function daysUntilDue(dueAt: string): number {
  const due = new Date(dueAt);
  if (Number.isNaN(due.getTime())) return 0;
  const startOfDay = (d: Date) => new Date(d.getFullYear(), d.getMonth(), d.getDate()).getTime();
  return Math.round((startOfDay(due) - startOfDay(new Date())) / 86_400_000);
}

const SEAL_STYLE: Record<SealKind, { tone: string; border: string }> = {
  hit: { tone: "text-[var(--profit)]", border: "border-dashed" },
  miss: { tone: "text-amber-600 dark:text-amber-400", border: "border-dashed" },
  due: { tone: "text-[var(--brand)]", border: "border-solid" },
  pending: { tone: "text-[var(--text-muted)]", border: "border-dotted" },
  unknown: { tone: "text-[var(--text-muted)]", border: "border-dotted" },
};

/**
 * 실링에 적을 두 줄을 고른다. 윗줄은 판정, 아랫줄은 "왜 그렇게 됐는지"의 가장 짧은
 * 근거 — 조기 적중은 닿은 날짜, 만기 판정은 목표가 오차, 검증 중은 남은 일수.
 */
function sealText(stamp: Stamp, kind: SealKind): { top: string; bottom: string } {
  if (stamp.targetHit) {
    return { top: kind === "due" ? "개봉" : "적중", bottom: `${formatDateKST(stamp.targetHit.date)} 도달` };
  }
  switch (kind) {
    case "due": {
      const d = daysUntilDue(stamp.dueAt);
      return { top: "개봉", bottom: d === 0 ? "오늘" : `${Math.abs(d)}일 지남` };
    }
    case "hit":
    case "miss":
      return {
        top: kind === "hit" ? "적중" : "빗나감",
        bottom:
          stamp.targetAccuracy === undefined ? "" : `오차 ${stamp.targetAccuracy.toFixed(1)}%`,
      };
    case "unknown":
      return { top: "판정", bottom: "불가" };
    default: {
      const d = daysUntilDue(stamp.dueAt);
      return { top: "검증", bottom: d <= 0 ? "곧 개봉" : `D-${d}` };
    }
  }
}

export function StampSeal({
  stamp,
  size = "md",
  className,
}: {
  stamp: Stamp;
  size?: "sm" | "md";
  className?: string;
}) {
  const kind = sealKindOf(stamp);
  const style = SEAL_STYLE[kind];
  const { top, bottom } = sealText(stamp, kind);

  return (
    <span
      aria-label={`${top} ${bottom}`}
      className={cn(
        "grid shrink-0 place-items-center rounded-full border-[1.5px] text-center leading-[1.05]",
        // 살짝 기울여야 붙인 도장처럼 읽힌다. 각도를 더 주면 글자가 읽기 어려워진다.
        "-rotate-[9deg] opacity-[0.85]",
        size === "sm" ? "size-9 text-[8.5px]" : "size-[46px] text-[9.5px]",
        style.tone,
        style.border,
        className,
      )}
    >
      <span className="font-extrabold tracking-tight">
        {top}
        {bottom && (
          <span className="block text-[8px] font-semibold tracking-normal opacity-80">
            {bottom}
          </span>
        )}
      </span>
    </span>
  );
}
