"use client";

import { CalendarDays } from "lucide-react";
import { SectionCard, SectionEmpty, SectionSkeleton } from "@/components/ui/section-card";
import { daysUntil, type UpcomingEvent } from "@/hooks/use-upcoming-events";
import { cn } from "@/lib/utils";

/**
 * 관심종목의 다가오는 일정.
 *
 * 목록에는 "지금 얼마인가"만 있다. 실적 발표 이틀 전에 담은 종목이 왜 그렇게 움직이는지는
 * 여기가 답한다.
 *
 * **직접 조회하지 않는다** — 목록 화면이 `useUpcomingEvents`로 한 번 부른 결과를 받아
 * 쓴다. 행의 임박 표식과 이 카드가 각자 부르면 같은 화면에서 요청이 두 배가 된다.
 */
export function WatchlistScheduleCard({
  events,
  onPick,
}: {
  events: UpcomingEvent[] | null;
  /** 행을 누르면 그 종목 퀵룩이 열린다 — 「무슨 일정인지」 다음 질문은 「그 종목은 지금 어떤가」다. */
  onPick: (symbol: string) => void;
}) {
  const rows = events?.slice(0, 6) ?? null;

  return (
    <SectionCard
      title={
        <span className="flex items-center gap-1.5">
          <CalendarDays className="size-3.5" />
          다가오는 일정
        </span>
      }
    >
      {rows === null ? (
        <SectionSkeleton rows={3} />
      ) : rows.length === 0 ? (
        <SectionEmpty text="예정된 실적·공시가 없습니다" />
      ) : (
        <ul className="min-w-0">
          {rows.map((ev, i) => {
            const dday = daysUntil(ev.date);
            return (
              <li key={`${ev.symbol}-${ev.date}-${i}`} className="min-w-0">
                <button
                  type="button"
                  onClick={() => onPick(ev.symbol)}
                  className="flex w-full min-w-0 items-start gap-2 rounded-md px-1 py-1 text-left text-[11px] hover:bg-accent"
                >
                  <span className="w-[38px] shrink-0 font-mono tabular-nums text-muted-foreground">
                    {ev.date.slice(5)}
                  </span>
                  <span className="min-w-0 flex-1">
                    <span className="block truncate font-medium">{ev.name}</span>
                    <span className="block truncate text-[10px] text-muted-foreground">
                      {ev.label.trim()}
                    </span>
                  </span>
                  {dday != null && dday <= 14 && (
                    <span className="shrink-0 rounded bg-[var(--brand-muted)] px-1 text-[10px] font-semibold text-[var(--brand)]">
                      {dday === 0 ? "오늘" : `D-${dday}`}
                    </span>
                  )}
                  <span
                    className={cn(
                      "shrink-0 rounded px-1 text-[10px]",
                      ev.source === "dart"
                        ? "bg-[var(--brand-muted)] text-[var(--brand)]"
                        : "bg-muted text-muted-foreground",
                    )}
                  >
                    {ev.source === "dart" ? "DART" : "실적"}
                  </span>
                </button>
              </li>
            );
          })}
        </ul>
      )}
    </SectionCard>
  );
}
