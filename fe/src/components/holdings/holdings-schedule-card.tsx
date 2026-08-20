"use client";

import { useEffect, useState } from "react";
import { CalendarDays } from "lucide-react";
import { SectionCard, SectionEmpty, SectionSkeleton } from "@/components/ui/section-card";
import { fetchEarningsClient } from "@/lib/earnings-api";
import type { EarningsCalendarEvent } from "@/lib/earnings-types";
import { cn } from "@/lib/utils";

type Row = EarningsCalendarEvent & { symbol: string; name: string };

/**
 * 보유 종목의 다가오는 일정.
 *
 * 종목 분석의 `StockScheduleCard`가 한 종목의 실적·공시를 보여주는데, 보유 화면에서
 * 궁금한 건 "내가 들고 있는 것들 중 **다음에** 무슨 일이 있나"다. 종목별로 들어가
 * 확인하게 두면 아무도 안 본다.
 *
 * **예정된 일정만** 싣는다. 이미 제출된 공시는 지난 일이고, 보유 화면에서 할 일은
 * 마음의 준비지 복기가 아니다.
 *
 * 종목 수만큼 요청이 나가므로 **상위 12종목**(비중순으로 넘어온 순서)까지만 조회한다.
 * 비중이 작은 종목의 실적일은 알아도 할 일이 없다.
 */
const MAX_SYMBOLS = 12;

export function HoldingsScheduleCard({
  symbols,
  onPick,
}: {
  /** 비중 높은 순으로 정렬된 [symbol, name] */
  symbols: Array<{ symbol: string; name: string }>;
  /** 행을 누르면 그 종목 퀵룩이 열린다 — 일정 다음 질문은 「그 종목은 지금 어떤가」다. */
  onPick: (symbol: string) => void;
}) {
  // 어느 심볼 집합에 대한 결과인지 함께 들고 있는다. 이게 없으면 "불러오는 중"을
  // 별도 boolean으로 두게 되고, 이펙트 안에서 동기 setState를 하게 된다.
  const [loaded, setLoaded] = useState<{ key: string; rows: Row[] } | null>(null);

  const key = symbols
    .slice(0, MAX_SYMBOLS)
    .map((s) => s.symbol)
    .join(",");
  const rows = !key ? [] : loaded?.key === key ? loaded.rows : null;

  useEffect(() => {
    if (!key) return;
    let alive = true;
    const targets = symbols.slice(0, MAX_SYMBOLS);
    Promise.all(
      targets.map((t) =>
        fetchEarningsClient(t.symbol)
          .then((data) =>
            (data.calendar ?? [])
              .filter((ev) => ev.kind === "upcoming")
              .map((ev) => ({ ...ev, symbol: t.symbol, name: t.name })),
          )
          // 한 종목이 실패해도 나머지 일정은 유효하다.
          .catch(() => [] as Row[]),
      ),
    )
      .then((lists) => {
        if (!alive) return;
        const merged = lists.flat().sort((a, b) => a.date.localeCompare(b.date));
        setLoaded({ key, rows: merged.slice(0, 6) });
      })
      .catch(() => {
        if (alive) setLoaded({ key, rows: [] });
      });
    return () => {
      alive = false;
    };
    // key로 의존성을 잡는다 — 배열 참조는 렌더마다 새로 만들어져 무한 루프가 된다.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [key]);

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
          {rows.map((ev, i) => (
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
          ))}
        </ul>
      )}
    </SectionCard>
  );
}
