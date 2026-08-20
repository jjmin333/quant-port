"use client";

import { useEffect, useState } from "react";
import { CalendarDays } from "lucide-react";
import {
  SectionCard,
  SectionEmpty,
  SectionSkeleton,
} from "@/components/ui/section-card";
import { fetchEarningsClient } from "@/lib/earnings-api";
import { formatDateKST } from "@/lib/format";
import type { EarningsCalendarEvent } from "@/lib/earnings-types";
import { cn } from "@/lib/utils";

/**
 * 실적 · 공시 일정 (옵션 1, 08-11 채택).
 *
 * 국내는 DART 공시, 해외는 Finnhub 실적 캘린더에서 날짜가 온다. 둘을 한 타임라인에
 * 섞되 **출처를 배지로 남긴다** — "예정"과 "이미 제출됨"은 성격이 다르고, 예정일은
 * 바뀔 수 있다는 걸 읽는 사람이 알아야 한다.
 *
 * 정렬은 최신순이되 **다가오는 일정(upcoming)을 맨 위로 올린다**. 지난 공시보다
 * 다음에 무슨 일이 있는지가 먼저 궁금한 화면이다.
 */
export function StockScheduleCard({ symbol }: { symbol: string }) {
  const [events, setEvents] = useState<EarningsCalendarEvent[] | null>(null);

  useEffect(() => {
    let alive = true;
    fetchEarningsClient(symbol)
      .then((data) => {
        if (!alive) return;
        const rows = [...(data.calendar ?? [])];
        rows.sort((a, b) => {
          if (a.kind !== b.kind) return a.kind === "upcoming" ? -1 : 1;
          // upcoming은 가까운 날짜부터, filed는 최근 것부터
          return a.kind === "upcoming"
            ? a.date.localeCompare(b.date)
            : b.date.localeCompare(a.date);
        });
        setEvents(rows.slice(0, 6));
      })
      .catch(() => {
        if (alive) setEvents([]);
      });
    return () => {
      alive = false;
    };
  }, [symbol]);

  return (
    <SectionCard
      title={
        <span className="flex items-center gap-1.5">
          <CalendarDays className="size-3.5" />
          실적 · 공시 일정
        </span>
      }
    >
      {events === null ? (
        <SectionSkeleton rows={4} />
      ) : events.length === 0 ? (
        <SectionEmpty text="등록된 일정이 없습니다" />
      ) : (
        <ol className="relative min-w-0 pl-3.5">
          {/* 세로 줄 */}
          <span
            className="absolute bottom-1 left-[3px] top-1 w-px bg-border"
            aria-hidden
          />
          {events.map((ev, i) => (
            <li key={`${ev.date}-${i}`} className="relative py-1">
              <span
                className={cn(
                  "absolute -left-[13.5px] top-[9px] size-[7px] rounded-full border-2 bg-card",
                  ev.kind === "upcoming" ? "border-[var(--brand)]" : "border-muted-foreground",
                )}
                aria-hidden
              />
              <p className="line-clamp-2 text-xs leading-snug">{ev.label.trim()}</p>
              <p className="mt-0.5 flex items-center gap-1.5 text-[11px] text-muted-foreground">
                {formatDateKST(ev.date)}
                <span className="rounded bg-muted px-1 text-[10px] uppercase">
                  {ev.source === "dart" ? "DART" : "Finnhub"}
                </span>
                {ev.kind === "upcoming" && (
                  <span className="text-[10px] text-[var(--brand)]">예정</span>
                )}
              </p>
            </li>
          ))}
        </ol>
      )}
    </SectionCard>
  );
}
