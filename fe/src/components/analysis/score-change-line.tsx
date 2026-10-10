"use client";

import { ChevronRight } from "lucide-react";

import {
  changeUnavailableLabel,
  formatDelta,
  shortDriverLabel,
  type ScoreChangeSummary,
} from "@/lib/score-explain-api";
import { cn } from "@/lib/utils";

/**
 * 종합 점수 카드의 **변화 한 줄** (2026-09-17 사용자 결정 A).
 *
 * # 왜 한 줄인가
 *
 * 이 카드는 모바일 개요 탭의 최상단이라 높이가 곧 비용이다 — 차트가 시작되기 전의
 * 마지막 관문이다(`stock-score-card.tsx` 머리글). 그래서 원인을 **가장 큰 것 하나만**
 * 적고, 나머지는 AI 분석 탭의 설명 패널이 받는다.
 *
 * # 왜 이 줄이 필요한가
 *
 * 점수의 하루 변동은 시장이 아니라 **평가 축의 유무**가 만드는 일이 잦다(실측
 * 2026-09-16: 열 종목 중 다섯이 그랬고, AAPL은 +11점 중 +10.75가 가치 축 결측이었다).
 * 그 날 이 카드는 「점수가 올랐다」만 말하고 있었다.
 *
 * # 비어 있는 경우를 구분한다
 *
 *   · 비교할 회차가 없다 → 그렇게 적는다 (0으로 때우지 않는다)
 *   · 회차는 있는데 점수가 그대로다 → 「변화 없음」
 *   · 점수 자체가 없는 종목 → 아무것도 그리지 않는다 (카드도 안 서 있다)
 */
export function ScoreChangeLine({
  change,
  onOpen,
  className,
}: {
  change: ScoreChangeSummary | null | undefined;
  /** 있으면 줄 전체가 버튼이 되어 설명 패널로 보낸다. */
  onOpen?: () => void;
  className?: string;
}) {
  if (!change) return null;

  const body = <ScoreChangeLineBody change={change} clickable={Boolean(onOpen)} />;
  const base =
    "flex w-full items-center gap-1.5 border-t border-[var(--border)] px-3 py-1.5 text-left text-micro";

  if (!onOpen) {
    return <div className={cn(base, className)}>{body}</div>;
  }
  return (
    <button
      type="button"
      onClick={onOpen}
      className={cn(base, "transition-colors hover:bg-[var(--surface-elevated)]", className)}
    >
      {body}
    </button>
  );
}

function ScoreChangeLineBody({
  change,
  clickable,
}: {
  change: ScoreChangeSummary;
  clickable: boolean;
}) {
  if (!change.available) {
    return (
      <span className="text-[var(--text-muted)]">{changeUnavailableLabel(change.reason)}</span>
    );
  }

  const delta = change.delta ?? 0;
  const cause = shortDriverLabel(change.topDriver);
  const tone = delta > 0 ? "text-[var(--profit)]" : delta < 0 ? "text-[var(--loss)]" : "";

  return (
    <>
      <span className="shrink-0 text-[var(--text-muted)]">전일</span>
      <span className={cn("shrink-0 font-semibold tabular-nums", tone)}>{formatDelta(delta)}</span>
      {cause ? (
        <>
          <span className="shrink-0 text-[var(--text-muted)]">·</span>
          {/*
            사유는 **줄이 좁아지면 먼저 잘리는 쪽**이다. 왼쪽의 변화량은 종목을
            넘겨 보는 사람이 찾는 값이라 잘리면 안 된다.
          */}
          <span className="min-w-0 truncate text-[var(--text-secondary)]">{cause}</span>
        </>
      ) : (
        <span className="text-[var(--text-muted)]">변화 없음</span>
      )}
      {clickable && (
        <span className="ml-auto flex shrink-0 items-center text-[var(--text-muted)]">
          근거
          <ChevronRight className="h-3 w-3" aria-hidden />
        </span>
      )}
    </>
  );
}

/**
 * 목록 행에 들어가는 더 짧은 꼴 (2026-09-17 사용자 결정 C).
 *
 * 카드 줄과 달리 **자기 테두리를 갖지 않는다** — 행 안의 한 칸이라 여백과 구분선은
 * 행이 정한다. 「전일」이라는 말도 뺀다: 목록 머리에 기준일이 한 번 적혀 있고,
 * 행마다 같은 단어를 스무 번 되풀이하면 정작 다른 값인 숫자가 묻힌다.
 */
export function ScoreChangeInline({
  change,
  className,
}: {
  change: ScoreChangeSummary | null | undefined;
  className?: string;
}) {
  if (!change || !change.available) return null;

  const delta = change.delta ?? 0;
  const cause = shortDriverLabel(change.topDriver);
  const tone = delta > 0 ? "text-[var(--profit)]" : delta < 0 ? "text-[var(--loss)]" : "";

  return (
    <span className={cn("flex min-w-0 items-center gap-1 text-micro", className)}>
      <span className={cn("shrink-0 font-semibold tabular-nums", tone)}>{formatDelta(delta)}</span>
      {cause && (
        <>
          <span className="shrink-0 text-[var(--text-muted)]">·</span>
          <span className="min-w-0 truncate text-[var(--text-muted)]">{cause}</span>
        </>
      )}
    </span>
  );
}
