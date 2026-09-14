"use client";

import { useEffect, useState } from "react";
import { fetchEarningsBatchClient } from "@/lib/earnings-api";
import type { EarningsCalendarEvent } from "@/lib/earnings-types";

export type UpcomingEvent = EarningsCalendarEvent & { symbol: string; name: string };

/*
여러 종목의 **예정된** 실적·공시를 한 번에.

# 종목당 1요청 → 배치 1요청 (2026-09-04 사용자 지적 11)

이 훅의 머리글에는 「배치 엔드포인트가 없어 종목당 1요청이다」라고 적혀 있었다.
**그 문장이 오래전에 거짓이 됐다** — `/stocks/earnings/batch`도, 그것을 부르는
`fetchEarningsBatchClient`도 이미 있었고, 이 훅만 옛 경로에 남아 있었다.

값은 컸다. 이 훅은 관심·보유·뉴스 레일 세 화면이 쓰고 최대 12종목이라, 그 화면을
여는 것만으로 **요청 12개가 한꺼번에** 나갔다. 로그인 사용자 한도가 초당 10건이라
(burst 20) 화면 하나가 그 절반을 쓰고, 화면을 옮기면 앞 화면의 요청과 겹쳐
「요청한 횟수가 너무 잦습니다」가 떴다. 지금은 **1건**이다.

`max` 상한은 남긴다 — 이유가 달라졌다. 요청 수가 아니라 응답 크기와 서버가 한
요청에서 도는 종목 수를 묶는다(배치 핸들러는 종목마다 고루틴을 띄운다).

# 한 번 부른 결과를 둘이 나눠 쓴다

목록 행(임박 표식)과 레일 카드(일정 목록)가 **같은 호출**을 쓴다. 같은 데이터를
두 컴포넌트가 각자 부르면 같은 화면에서 요청이 두 배가 된다.

이미 제출된 공시(`filed`)는 뺀다. 관심종목 화면에서 할 일은 앞으로의 준비지 복기가
아니다.
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
    fetchEarningsBatchClient(list)
      .then((bySymbol) => {
        if (!alive) return;
        /*
          응답은 심볼을 키로 한 객체이고 **일정을 못 가져온 종목은 아예 빠진다** —
          종목당 한 번씩 부르던 시절 그 종목만 실패하고 나머지는 떴던 것과 같은
          결과다. 그래서 `list`를 돌면서 없는 키는 건너뛴다.
        */
        const rows: UpcomingEvent[] = [];
        for (const symbol of list) {
          for (const ev of bySymbol[symbol]?.calendar ?? []) {
            if (ev.kind !== "upcoming") continue;
            rows.push({ ...ev, symbol, name: nameBySymbol.get(symbol) || symbol });
          }
        }
        rows.sort((a, b) => a.date.localeCompare(b.date));
        setLoaded({ key, rows });
      })
      // 배치 하나가 실패하면 이 목록은 통째로 빈다 — 종목당 요청 시절의 「한
      // 종목만 빠진다」와 달라진 점이다. 그 대신 요청이 12분의 1이다.
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
