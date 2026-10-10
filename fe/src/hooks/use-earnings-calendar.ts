"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { useMySymbols } from "@/hooks/use-news-symbols";
import { useVisibleInterval } from "@/hooks/use-visible-interval";
import { SCREEN_SLOW_REFRESH_MS } from "@/lib/screen-refresh";
import { fetchQuotesBatchClient } from "@/lib/market-api";
import type { QuoteSummary } from "@/lib/quotes-types";
import {
  fetchEarningsCalendar,
  fetchEarningsStats,
  fetchEarningsSummary,
} from "@/lib/earnings-api";
import type {
  EarningsDaySummary,
  EarningsEvent,
  EarningsMarketFilter,
  EarningsStats,
} from "@/lib/earnings-types";

/**
 * 실적 화면의 데이터 세 갈래.
 *
 * **히트맵·상세·집계를 따로 받는다.** 단위를 넘길 때 필요한 것은 날짜별 건수(≤31행)
 * 뿐이고, 상세는 고른 구간만, 집계는 숫자 몇 개다. 하나로 합치면 달을 한 번 넘길
 * 때마다 시즌 달의 3,000행이 따라온다(PRINCIPLES 응답 속도 원칙 #4).
 *
 * 세 훅이 모두 **키드 상태**를 쓴다 — 이펙트에서 동기로 `null`을 넣으면 캐스케이드
 * 렌더가 되므로(react-hooks/set-state-in-effect), 키가 안 맞으면 아직 안 온 것으로
 * 읽는다. 이 앱의 기존 패턴이다(use-news-symbols.ts).
 *
 * # 30분마다 스스로 다시 받는다 (2026-09-16 사용자 요청)
 *
 * 실적 화면을 열어 둔 채로는 마운트 시각의 일정이 그대로 서 있었다. 발표가 하루
 * 종일 들어오는 화면인데(국내 배치 18:30, 해외는 5분마다 당일 창) 그 사실이 화면에
 * 닿지 않았다.
 *
 * **느린 쪽을 쓴다** (`SCREEN_SLOW_REFRESH_MS`). 이 목록을 만드는 것은 하루 한두 번
 * 도는 적재 배치이고, 시즌의 한 달 창은 300행 상한까지 차는 큰 응답이라 5분마다
 * 왕복시키면 1 ECPU짜리 DB에 그대로 얹힌다.
 *
 * **키가 바뀐 회차와 갱신 회차가 같은 길을 쓴다** — 키드 상태라 갱신은 같은 키로
 * 덮어쓰는 일이고, 그래서 화면이 「불러오는 중」으로 떨어지지 않는다. 갱신이 실패하면
 * 직전 값이 그대로 남는다(`load`의 catch가 키가 없는 회차에서만 비운다).
 */

/**
 * 달 전체(또는 주 전체) 보기의 행 상한.
 *
 * 실적 시즌의 한 달은 3,089행(gzip 136KB)이다 — 그만큼을 스크롤로 읽는 사람은
 * 없고, DOM 3천 줄은 그 자체로 느리다. 히트맵이 「언제 몰리나」를 이미 말하므로
 * 목록은 날짜를 골라 보는 것이 정상 경로다.
 */
export const MONTH_ROW_LIMIT = 300;

export function useEarningsSummary(
  from: string,
  to: string,
  market: EarningsMarketFilter,
): { days: EarningsDaySummary[] | null; loading: boolean } {
  const key = `${from}|${to}|${market}`;
  const [state, setState] = useState<{ key: string; days: EarningsDaySummary[] } | null>(null);

  useEffect(() => {
    let alive = true;
    fetchEarningsSummary(from, to, market)
      .then((days) => {
        if (alive) setState({ key, days: days ?? [] });
      })
      .catch(() => {
        if (alive) setState({ key, days: [] });
      });
    return () => {
      alive = false;
    };
  }, [key, from, to, market]);

  /*
    갱신은 **같은 키로 덮어쓴다** — 실패하면 손대지 않는다(직전 히트맵이 남는다).
    첫 적재의 실패만 위 이펙트에서 빈 배열이 되고, 화면은 그것을 「없음」으로 읽는다.
  */
  useVisibleInterval(() => {
    fetchEarningsSummary(from, to, market)
      .then((days) => setState({ key, days: days ?? [] }))
      .catch(() => {});
  }, SCREEN_SLOW_REFRESH_MS);

  const days = state?.key === key ? state.days : null;
  return { days, loading: days === null };
}

/** 선택 구간의 상세 행. 서버 구간 상한이 31일이라 달 전체도 한 요청이다. */
export function useEarningsEvents(
  from: string,
  to: string,
  market: EarningsMarketFilter,
  symbols: string[] | null,
  limit = MONTH_ROW_LIMIT,
  /**
   * false면 아무것도 부르지 않고 빈 목록으로 답한다.
   *
   * 레일의 「내 종목 다음 발표」와 주간 보드가 이걸 쓴다 — 담은 종목이 없으면
   * 필터가 빈 배열이 되고, 그러면 이 훅이 **필터 없이 그 구간 전체**를 받아 온다
   * (실측에서 비로그인 방문자에게 16KB 요청이 한 번 더 나갔고, 카드는 그 결과를
   * 쓰지도 않고 "관심·보유 종목이 없습니다"를 띄웠다).
   */
  enabled = true,
): { events: EarningsEvent[] | null; loading: boolean } {
  const symbolsKey = symbols && symbols.length > 0 ? [...symbols].sort().join(",") : "";
  const key = `${from}|${to}|${market}|${symbolsKey}|${limit}`;
  const [state, setState] = useState<{ key: string; events: EarningsEvent[] } | null>(null);

  const fetchRows = useCallback(
    () =>
      fetchEarningsCalendar(
        from,
        to,
        market,
        symbolsKey ? symbolsKey.split(",") : undefined,
        limit,
      ),
    [from, to, market, symbolsKey, limit],
  );

  useEffect(() => {
    if (!enabled || !from || !to) return;
    let alive = true;
    fetchRows()
      .then((events) => {
        if (alive) setState({ key, events: events ?? [] });
      })
      .catch(() => {
        // 실패와 빈 목록을 같은 안내로 다룬다 — 둘 다 "지금은 못 보여준다"다.
        if (alive) setState({ key, events: [] });
      });
    return () => {
      alive = false;
    };
  }, [key, from, to, enabled, fetchRows]);

  // 갱신은 같은 키로 덮어쓰고, 실패하면 직전 목록을 남긴다(위 훅 머리글).
  useVisibleInterval(
    () => {
      fetchRows()
        .then((events) => setState({ key, events: events ?? [] }))
        .catch(() => {});
    },
    SCREEN_SLOW_REFRESH_MS,
    enabled && Boolean(from) && Boolean(to),
  );

  if (!enabled) return { events: EMPTY_EVENTS, loading: false };
  const events = state?.key === key ? state.events : null;
  return { events, loading: events === null };
}

/** 상단 스코어보드 집계. */
export function useEarningsStats(
  from: string,
  to: string,
  market: EarningsMarketFilter,
): { stats: EarningsStats | null } {
  const key = `${from}|${to}|${market}`;
  const [state, setState] = useState<{ key: string; stats: EarningsStats | null } | null>(null);

  useEffect(() => {
    if (!from || !to) return;
    let alive = true;
    fetchEarningsStats(from, to, market)
      .then((stats) => {
        if (alive) setState({ key, stats });
      })
      .catch(() => {
        if (alive) setState({ key, stats: null });
      });
    return () => {
      alive = false;
    };
  }, [key, from, to, market]);

  // 갱신은 같은 키로 덮어쓰고, 실패하면 직전 집계를 남긴다(위 훅 머리글).
  useVisibleInterval(
    () => {
      fetchEarningsStats(from, to, market)
        .then((stats) => setState({ key, stats }))
        .catch(() => {});
    },
    SCREEN_SLOW_REFRESH_MS,
    Boolean(from) && Boolean(to),
  );

  return { stats: state?.key === key ? state.stats : null };
}

/**
 * 「내 종목」 필터에 쓸 심볼.
 *
 * `useMySymbols`를 그대로 쓰되 **예시 폴백은 버린다.** 뉴스 화면에서는 비로그인
 * 방문자에게 인기 종목을 보여 주는 것이 기능 설명이 되지만, 여기서는 토글 이름이
 * 「내 종목」이라 예시를 채우면 남의 종목을 내 것이라고 말하는 셈이 된다.
 */
export function useMyEarningsSymbols(): { symbols: string[]; available: boolean } {
  const { symbols, personal } = useMySymbols("domestic");
  return useMemo(
    () => ({ symbols: personal ? symbols : [], available: personal }),
    [symbols, personal],
  );
}

/**
 * 표에 보이는 행의 현재가 — **배치 1회**.
 *
 * 행마다 시세를 부르면 300행에 300 요청이다(항목당 I/O 금지). 심볼을 모아 한 번
 * 부르고, 폴링하지 않는다 — 이 화면에서 시세는 맥락이지 감시 대상이 아니다
 * (뉴스 화면의 `useSymbolQuotes`와 같은 판단).
 *
 * 상한을 두는 이유: 달 전체 보기는 300행이라 심볼도 그만큼인데, `quotes/batch`에
 * 수백 심볼을 얹으면 그쪽이 느려진다. 표는 어차피 위에서부터 읽는다.
 */
export function useEarningsQuotes(events: EarningsEvent[] | null): Map<string, QuoteSummary> {
  const key = useMemo(() => {
    const seen = new Set<string>();
    for (const ev of events ?? []) {
      seen.add(ev.symbol);
      if (seen.size >= QUOTE_SYMBOL_LIMIT) break;
    }
    return [...seen].sort().join(",");
  }, [events]);

  const [loaded, setLoaded] = useState<{ key: string; quotes: Map<string, QuoteSummary> }>({
    key: "",
    quotes: new Map(),
  });

  useEffect(() => {
    if (!key) return;
    let alive = true;
    fetchQuotesBatchClient(key.split(","))
      .then((rows) => {
        if (!alive) return;
        setLoaded({ key, quotes: new Map(rows.map((q) => [q.symbol, q])) });
      })
      .catch(() => {
        // 시세를 못 받아도 표는 그대로 뜬다 — 그 열만 「—」다.
        if (alive) setLoaded({ key, quotes: new Map() });
      });
    return () => {
      alive = false;
    };
  }, [key]);

  return loaded.key === key ? loaded.quotes : EMPTY_QUOTES;
}

const QUOTE_SYMBOL_LIMIT = 120;
const EMPTY_QUOTES = new Map<string, QuoteSummary>();

/** 고정 참조 — 매 렌더 새 배열을 만들면 소비자의 useMemo가 늘 다시 돈다. */
const EMPTY_EVENTS: EarningsEvent[] = [];
