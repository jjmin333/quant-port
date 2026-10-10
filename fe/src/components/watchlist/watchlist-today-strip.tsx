"use client";

import { Card } from "@/components/ui/card";
import type { WatchlistTodaySummary } from "@/lib/watchlist-today";
import { cn } from "@/lib/utils";

/**
 * 「오늘 내 관심」 띠 — 목록 맨 위 한 줄 (2026-09-27 사용자 결정 A).
 *
 * 행을 하나씩 훑기 전에 **오늘 볼 곳**을 먼저 말한다: 오른 것·내린 것, 점수가 크게 움직인
 * 종목, 뉴스가 몰린 종목, 가장 가까운 실적. 칩은 값이 있을 때만 선다 — 「점수 변화 없음」을
 * 적는 칩은 안내가 아니라 잡음이다. 좁은 화면에서는 줄바꿈으로 흐른다.
 */
export function WatchlistTodayStrip({ summary }: { summary: WatchlistTodaySummary }) {
  if (summary.total === 0) return null;
  return (
    <Card className="flex flex-wrap items-center gap-1.5 px-3 py-2 text-cell">
      <span className="mr-1 font-semibold text-[var(--text-primary)]">오늘 내 관심 {summary.total}</span>
      {(summary.up > 0 || summary.down > 0) && (
        <Chip>
          <span className="text-[var(--profit)]">상승 {summary.up}</span>
          <span className="text-[var(--text-muted)]"> · </span>
          <span className="text-[var(--loss)]">하락 {summary.down}</span>
        </Chip>
      )}
      {summary.scoreMover && (
        <Chip tone="score">
          점수 {summary.scoreMover.delta > 0 ? "급등" : "급락"} · {summary.scoreMover.name}{" "}
          {summary.scoreMover.delta > 0 ? `▲${summary.scoreMover.delta}` : `▼${-summary.scoreMover.delta}`}
        </Chip>
      )}
      {summary.nextEvent && (
        <Chip tone="event">
          실적 임박 · {summary.nextEvent.name}{" "}
          {summary.nextEvent.days === 0 ? "오늘" : `D-${summary.nextEvent.days}`}
        </Chip>
      )}
      {summary.newsLeader && (
        <Chip tone="news">
          뉴스 최다 · {summary.newsLeader.name} {summary.newsLeader.count}
        </Chip>
      )}
    </Card>
  );
}

function Chip({
  tone,
  children,
}: {
  tone?: "score" | "event" | "news";
  children: React.ReactNode;
}) {
  return (
    <span
      className={cn(
        "inline-flex max-w-full items-center truncate rounded-full border px-2 py-0.5 text-micro",
        tone === "score" && "border-transparent bg-emerald-500/12 font-semibold text-emerald-700 dark:text-emerald-400",
        tone === "event" && "border-transparent bg-amber-500/15 font-semibold text-amber-700 dark:text-amber-400",
        tone === "news" && "border-transparent bg-[var(--brand-muted)] font-semibold text-[var(--brand)]",
        !tone && "border-[var(--border)] text-[var(--text-secondary)]",
      )}
    >
      {children}
    </span>
  );
}
