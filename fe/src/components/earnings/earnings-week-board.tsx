"use client";

import { useMemo } from "react";
import { ChevronLeft, ChevronRight } from "lucide-react";
import type { EarningsDaySummary, EarningsEvent } from "@/lib/earnings-types";
import { formatShortDate, shiftDay, todayKST, weekdayOf, weekdaysOf } from "@/lib/earnings-range";
import { cn } from "@/lib/utils";

/**
 * 주간 보드 (L2) — 「이번 주 어느 날이 격전지인가」.
 *
 * 월 히트맵은 그 답을 **색 농도로만** 말한다. 실적은 주 단위로 몰리므로(실측:
 * 8월 국내 190건이 3~14일에 전부) 한 주를 나란히 놓으면 날짜별 무게와 대표 종목이
 * 같이 읽힌다.
 *
 * **월~금만 그린다.** 발표는 거래일에만 있어서 토·일 칸은 영원히 비고, 그러면
 * 격자의 2/7이 낭비된다.
 */
export function EarningsWeekBoard({
  weekStart,
  days,
  events,
  selected,
  onSelect,
  onWeekChange,
}: {
  weekStart: string;
  days: EarningsDaySummary[] | null;
  /** 이 주의 상세 행 — 날짜별 대표 종목을 뽑는 데 쓴다. */
  events: EarningsEvent[] | null;
  selected: string | null;
  onSelect: (date: string | null) => void;
  onWeekChange: (weekStart: string) => void;
}) {
  const dates = useMemo(() => weekdaysOf(weekStart), [weekStart]);
  const byDate = useMemo(() => new Map((days ?? []).map((d) => [d.date, d])), [days]);

  /**
   * 날짜별 대표 종목 — **서프라이즈 절댓값이 가장 큰 것**.
   *
   * 「가장 큰 발표」를 시가총액으로 고르고 싶지만 이 화면에 시총이 없다. 대신
   * 「가장 놀라운 발표」를 고른다 — 그게 그 날짜를 열어 볼 이유이기도 하다.
   */
  const highlights = useMemo(() => {
    const map = new Map<string, EarningsEvent[]>();
    for (const ev of events ?? []) {
      if (ev.surprise?.pct == null) continue;
      const list = map.get(ev.date) ?? [];
      list.push(ev);
      map.set(ev.date, list);
    }
    for (const [, list] of map) {
      list.sort((a, b) => Math.abs(b.surprise!.pct!) - Math.abs(a.surprise!.pct!));
    }
    return map;
  }, [events]);

  const today = todayKST();

  return (
    <div className="rounded-xl border bg-card p-3">
      <div className="mb-2 flex items-center justify-between">
        <button
          type="button"
          aria-label="이전 주"
          onClick={() => onWeekChange(shiftDay(weekStart, -7))}
          className="rounded-md p-1 text-muted-foreground hover:bg-muted hover:text-foreground"
        >
          <ChevronLeft className="size-4" />
        </button>
        <div className="flex items-center gap-2">
          <span className="text-sm font-semibold tabular-nums">
            {formatShortDate(dates[0])} – {formatShortDate(dates[4])}
          </span>
          {selected && (
            <button
              type="button"
              onClick={() => onSelect(null)}
              className="rounded-full bg-muted px-2 py-0.5 text-micro text-muted-foreground hover:text-foreground"
            >
              이 주 전체 보기
            </button>
          )}
        </div>
        <button
          type="button"
          aria-label="다음 주"
          onClick={() => onWeekChange(shiftDay(weekStart, 7))}
          className="rounded-md p-1 text-muted-foreground hover:bg-muted hover:text-foreground"
        >
          <ChevronRight className="size-4" />
        </button>
      </div>

      <div className="grid grid-cols-2 gap-2 sm:grid-cols-3 lg:grid-cols-5">
        {dates.map((date) => {
          const cell = byDate.get(date);
          const total = cell ? cell.confirmed + cell.scheduled + cell.estimated : 0;
          const picks = (highlights.get(date) ?? []).slice(0, 3);
          const isSelected = selected === date;
          return (
            <button
              key={date}
              type="button"
              onClick={() => onSelect(isSelected ? null : date)}
              className={cn(
                "min-w-0 rounded-lg border p-2 text-left transition-colors hover:bg-muted/50",
                isSelected ? "border-foreground ring-1 ring-foreground" : "border-border",
                date === today && !isSelected && "border-muted-foreground/60",
              )}
            >
              <div className="flex items-baseline gap-1">
                <span className="text-cell font-bold">
                  {weekdayOf(date)} {Number(date.slice(8))}
                </span>
                <span className="text-micro font-normal text-muted-foreground">
                  {total > 0 ? `${total}건` : "없음"}
                </span>
              </div>
              <p className="mt-0.5 truncate text-micro text-muted-foreground">
                {cell && cell.confirmed > 0
                  ? `발표 ${cell.confirmed}`
                  : cell && cell.scheduled > 0
                    ? `예정 ${cell.scheduled}`
                    : cell && cell.estimated > 0
                      ? `추정 ${cell.estimated}`
                      : " "}
              </p>
              <div className="mt-1 space-y-0.5">
                {picks.length === 0 ? (
                  <p className="truncate text-micro text-muted-foreground">&nbsp;</p>
                ) : (
                  picks.map((ev) => (
                    <p
                      key={`${ev.symbol}-${ev.fiscalPeriod}`}
                      className="flex min-w-0 items-baseline gap-1 text-micro"
                    >
                      <span className="min-w-0 flex-1 truncate">{ev.name || ev.symbol}</span>
                      <span
                        className={cn(
                          "shrink-0 font-num tabular-nums",
                          ev.surprise!.pct! >= 0
                            ? "text-[var(--profit)]"
                            : "text-[var(--loss)]",
                        )}
                      >
                        {ev.surprise!.pct! >= 0 ? "+" : ""}
                        {ev.surprise!.pct!.toFixed(0)}%
                      </span>
                    </p>
                  ))
                )}
              </div>
            </button>
          );
        })}
      </div>
    </div>
  );
}
