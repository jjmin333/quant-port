"use client";

import { useEffect, useMemo, useState } from "react";
import { useAuth } from "@/components/providers/auth-provider";
import { fetchQuotesBatchClient, fetchWatchlistClient } from "@/lib/market-api";
import { fetchUserHoldingsClient } from "@/lib/holdings-import-api";
import type { QuoteSummary } from "@/lib/quotes-types";

/**
 * 뉴스 화면이 쓰는 종목 정보 두 가지 — **시세**와 **내 종목** — 를 한 곳에서 받는다.
 *
 * 「오늘의 이슈 종목」(본문 상단), 기사 행의 종목 칩, 레일의 「내 종목 뉴스」가 모두
 * 같은 심볼을 필요로 한다. 셋이 각자 부르면 같은 화면에서 배치 요청이 세 번 나간다.
 */

/**
 * 비로그인 사용자에게 「내 종목」이 어떤 기능인지 **실제로 동작시켜** 보여주기 위한
 * 임시 목록.
 *
 * 잠긴 버튼은 "나중에"가 되지만, 한 번 써 본 기능은 "내 종목으로 바꾸고 싶다"가 된다.
 * 다만 남의 포트폴리오처럼 보이면 안 되므로 화면에서는 pill과 안내 띠 **두 곳**에
 * 예시임을 적는다.
 */
export const SAMPLE_SYMBOLS: Record<"domestic" | "us", string[]> = {
  domestic: ["005930", "000660", "035420"],
  us: ["NVDA", "AAPL", "TSLA"],
};

export type MySymbols = {
  symbols: string[];
  /** true면 로그인해서 받은 진짜 내 종목, false면 인기 종목 예시 */
  personal: boolean;
  loading: boolean;
};

/** 보유 ∪ 관심 종목. 비로그인이면 인기 종목 예시로 대신한다. */
export function useMySymbols(market: "domestic" | "us"): MySymbols {
  const { session, loading: authLoading } = useAuth();
  // 어느 세션의 결과인지 함께 들고 있는다 — 로그아웃했을 때 이펙트에서 동기로
  // 비우면 캐스케이드 렌더가 된다. 세션 키가 안 맞으면 아래 useMemo가 없는 셈 친다.
  const sessionKey = session?.user?.id ? String(session.user.id) : "";
  const [loaded, setLoaded] = useState<{ key: string; symbols: string[] } | null>(null);
  const mine = loaded && loaded.key === sessionKey ? loaded.symbols : null;

  useEffect(() => {
    if (!sessionKey) return;
    let alive = true;
    Promise.all([
      fetchWatchlistClient().catch(() => []),
      fetchUserHoldingsClient().catch(() => []),
    ])
      .then(([watchlist, holdings]) => {
        if (!alive) return;
        const seen = new Set<string>();
        for (const item of holdings) {
          const sym = item.symbol?.trim().toUpperCase();
          if (sym) seen.add(sym);
        }
        for (const item of watchlist) {
          const sym = item.symbol?.trim().toUpperCase();
          if (sym) seen.add(sym);
        }
        setLoaded({ key: sessionKey, symbols: [...seen] });
      })
      .catch(() => {
        if (alive) setLoaded({ key: sessionKey, symbols: [] });
      });
    return () => {
      alive = false;
    };
  }, [sessionKey]);

  return useMemo(() => {
    if (session && mine !== null) {
      // 로그인했지만 아무것도 담지 않은 사람에게 빈 목록을 주면 pill이 늘 0건이 된다.
      // 그때도 예시를 보여 주는 편이 "이 기능은 무엇인가"를 설명한다.
      if (mine.length > 0) {
        return { symbols: mine, personal: true, loading: false };
      }
      return { symbols: SAMPLE_SYMBOLS[market], personal: false, loading: false };
    }
    return {
      symbols: SAMPLE_SYMBOLS[market],
      personal: false,
      loading: authLoading || (Boolean(session) && mine === null),
    };
  }, [session, mine, market, authLoading]);
}

export type SymbolQuotes = {
  quotes: Map<string, QuoteSummary>;
  /**
   * 아직 배치가 돌아오지 않았다.
   *
   * 「시세를 못 찾은 종목」과 **구분되어야 한다.** 이 둘이 같은 빈 맵으로 보이던
   * 동안, 화면은 시세가 오기 전 종목 칩에 `quote?.name || symbol`로 종목코드를
   * 그렸다가 이름으로 바꿨다 — 사용자에게 그건 로딩이 아니라 「종목코드가 뜨는
   * 버그」였다. 로딩 중에는 자리만 잡고, 정말 못 찾았을 때만 코드를 적는다.
   */
  loading: boolean;
};

/**
 * 심볼 → 시세. 목록이 바뀔 때만 다시 부르고, 5초 폴링에는 태우지 않는다 —
 * 뉴스 화면에서 시세는 맥락이지 감시 대상이 아니다.
 */
export function useSymbolQuotesState(symbols: string[]): SymbolQuotes {
  const key = useMemo(() => [...symbols].sort().join(","), [symbols]);
  const [loaded, setLoaded] = useState<{ key: string; quotes: Map<string, QuoteSummary> }>({
    key: "",
    quotes: new Map(),
  });

  useEffect(() => {
    // key가 비면 아무것도 하지 않는다 — 아래 반환문이 키 불일치를 빈 맵으로 처리하므로
    // 여기서 동기로 상태를 비울 필요가 없다(캐스케이드 렌더 방지).
    if (!key) return;
    let alive = true;
    fetchQuotesBatchClient(key.split(","))
      .then((rows) => {
        if (!alive) return;
        const map = new Map<string, QuoteSummary>();
        for (const row of rows) {
          map.set(row.symbol.trim().toUpperCase(), row);
        }
        setLoaded({ key, quotes: map });
      })
      .catch(() => {
        if (alive) setLoaded({ key, quotes: new Map() });
      });
    return () => {
      alive = false;
    };
  }, [key]);

  if (loaded.key === key) {
    return { quotes: loaded.quotes, loading: false };
  }
  // 심볼이 하나도 없으면 받을 것도 없다 — 영원한 로딩으로 두지 않는다.
  return { quotes: EMPTY_QUOTES, loading: key.length > 0 };
}

const EMPTY_QUOTES: Map<string, QuoteSummary> = new Map();

/** 로딩 여부가 필요 없는 호출부용. */
export function useSymbolQuotes(symbols: string[]): Map<string, QuoteSummary> {
  return useSymbolQuotesState(symbols).quotes;
}
