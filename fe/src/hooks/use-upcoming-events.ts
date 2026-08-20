"use client";

import { useEffect, useState } from "react";
import { fetchEarningsClient } from "@/lib/earnings-api";
import type { EarningsCalendarEvent } from "@/lib/earnings-types";

export type UpcomingEvent = EarningsCalendarEvent & { symbol: string; name: string };

/**
 * 여러 종목의 **예정된** 실적·공시를 한 번에.
 *
 * 배치 엔드포인트가 없어 종목당 1요청이다. 그래서 `max`로 상한을 두고, 한 번 부른
 * 결과를 목록 행(임박 표식)과 레일 카드(일정 목록)가 **함께** 쓴다 — 같은 데이터를
 * 두 컴포넌트가 각자 부르면 같은 화면에서 요청이 두 배가 된다.
 *
 * 이미 제출된 공시(`filed`)는 뺀다. 관심종목 화면에서 할 일은 앞으로의 준비지 복기가
 * 아니다.
 */
export function useUpcomingEvents(
  symbols: Array<{ symbol: string; name: string }>,
  max = 12,
): { events: UpcomingEvent[] | null; bySymbol: Map<string, UpcomingEvent[]> } {
  // 어느 심볼 집합의 결과인지 함께 들고 있는다 — 별도 loading 플래그를 이펙트에서
  // 동기로 세팅하지 않기 위해서다(캐스케이드 렌더).
  const [loaded, setLoaded] = useState<{ key: string; rows: UpcomingEvent[] } | null>(null);

  const targets = symbols.slice(0, max);
  const key = targets.map((t) => t.symbol).join(",");

  useEffect(() => {
    if (!key) return;
    let alive = true;
    const list = key.split(",");
    const nameBySymbol = new Map(targets.map((t) => [t.symbol, t.name]));
    Promise.all(
      list.map((symbol) =>
        fetchEarningsClient(symbol)
          .then((data) =>
            (data.calendar ?? [])
              .filter((ev) => ev.kind === "upcoming")
              .map((ev) => ({ ...ev, symbol, name: nameBySymbol.get(symbol) || symbol })),
          )
          // 한 종목이 실패해도 나머지 일정은 유효하다.
          .catch(() => [] as UpcomingEvent[]),
      ),
    )
      .then((lists) => {
        if (!alive) return;
        setLoaded({ key, rows: lists.flat().sort((a, b) => a.date.localeCompare(b.date)) });
      })
      .catch(() => {
        if (alive) setLoaded({ key, rows: [] });
      });
    return () => {
      alive = false;
    };
    // 배열 참조는 렌더마다 새로 만들어지므로 심볼 문자열로 의존성을 잡는다.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [key]);

  const events = !key ? [] : loaded?.key === key ? loaded.rows : null;

  const bySymbol = new Map<string, UpcomingEvent[]>();
  for (const ev of events ?? []) {
    const rows = bySymbol.get(ev.symbol);
    if (rows) rows.push(ev);
    else bySymbol.set(ev.symbol, [ev]);
  }

  return { events, bySymbol };
}

/** 오늘로부터 남은 일수. 과거 날짜면 null(이미 지난 일정은 표시하지 않는다). */
export function daysUntil(date: string): number | null {
  const target = new Date(`${date}T00:00:00+09:00`).getTime();
  if (!Number.isFinite(target)) return null;
  const today = new Date();
  const todayKst = new Date(
    `${today.toLocaleDateString("en-CA", { timeZone: "Asia/Seoul" })}T00:00:00+09:00`,
  ).getTime();
  const diff = Math.round((target - todayKst) / 86_400_000);
  return diff < 0 ? null : diff;
}
