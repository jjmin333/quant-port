"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import type { OrderBook, QuoteDetail, QuoteStreamEvent, TradeTick } from "@/lib/quotes-types";
import { fetchQuoteTerminalSnapshotClient, quotePriceStreamUrl } from "@/lib/market-api";
import { isPageVisible } from "@/hooks/use-page-visible";
import { useStreamAwake } from "@/hooks/use-stream-awake";
import { PRICE_POLL_MS } from "@/lib/price-cadence";

/*
상세 화면의 폴링 주기 — **자기 숫자를 들고 있지 않는다.**

`price-cadence.ts`가 「숫자는 여기 하나뿐」이라고 적힌 채로 만들어졌는데, 이 파일만
그 import를 빠뜨리고 `const POLL_MS = 2_000`을 그대로 들고 있었다. 그래서 09-04에
서버 주기를 1초로 내릴 때 이 화면 하나만 2초에 남을 뻔했다 — 상수를 한 곳에 모으는
일이 실패하는 방식이 정확히 이것이다(모으고 나서 한 곳이 안 따라온다).
*/
const POLL_MS = PRICE_POLL_MS;

/*
장이 완전히 닫힌 동안에는 **묻지 않는다** (2026-09-04 사용자 지적).

실측(2026-09-03 마감 후 API 로그): SK하이닉스 상세를 열어 둔 브라우저 하나가
`/quotes/000660/terminal-snapshot`을 2초마다 쳤다. 그 경로는 종목당 Toss 시세를
건드리는 자리인데, 다음 장이 열릴 때까지 가격은 바뀔 수 없다 — 응답이 매번 같다.

세션은 서버가 판정해 응답에 실어 준다(`marketSession`). 닫혀 있으면 폴링 타이머와
SSE 연결을 둘 다 걷고, 서버가 함께 준 `nextSessionAt`(국내 프리마켓 08:00 등)에
타이머 하나로 깨어난다. 화면이 시간대를 다시 계산하지 않는 이유는
`lib/market-session` 머리글에 있다 — 서버 판정과 어긋나는 두 번째 진실을 만들지 않는다.
*/
function isClosedSession(detail: QuoteDetail | null): boolean {
  return detail?.marketSession === "closed";
}

/** 개장 시각을 모를 때의 되묻기 주기 — 아래 「시각을 모르면」 주석 참조. */
const CLOSED_RECHECK_MS = 5 * 60_000;

/** `at`까지 남은 밀리초. 이미 지났거나 값이 없으면 null. */
function delayUntil(at: string | undefined): number | null {
  if (!at) return null;
  const ms = new Date(at).getTime();
  if (Number.isNaN(ms)) return null;
  // 개장 직후 한 박자 뒤에 묻는다 — 종이 울리는 순간에는 서버 달력도 막 넘어간다.
  return Math.min(Math.max(ms - Date.now() + 2_000, 1_000), 0x7fffffff);
}

type UseQuoteDetailOptions = {
  /** Poll terminal snapshot (default true). Set false for analysis-only pages. */
  live?: boolean;
  /**
   * 호가창을 **지금 그리고 있는가.** 서버가 이 신호로 웹소켓 호가 구독을 정한다.
   *
   * 상세 화면에 들어간 것과 호가를 보는 것은 다르다 — 호가창은 차트 탭에서만
   * 그려지는데, 예전에는 상세 화면 진입만으로 호가까지 구독해서 아무도 안 보는
   * 프레임을 받았다(체결의 4.3배).
   */
  wantsOrderBook?: boolean;
};

function applyQuoteStreamEvent(prev: QuoteDetail | null, ev: QuoteStreamEvent): QuoteDetail | null {
  if (!prev || prev.symbol.toUpperCase() !== ev.symbol.toUpperCase()) {
    return prev;
  }
  return {
    ...prev,
    currentPrice: ev.currentPrice,
    change: ev.change,
    changeRate: ev.changeRate,
    prevClose: ev.prevClose,
    regularClose: ev.regularClose ?? prev.regularClose,
    marketSession: ev.marketSession ?? prev.marketSession,
    updatedAt: ev.updatedAt,
  };
}

function toMillis(iso: string | undefined): number {
  if (!iso) return 0;
  const t = new Date(iso).getTime();
  return Number.isNaN(t) ? 0 : t;
}

export function useQuoteDetail(symbol: string, options: UseQuoteDetailOptions = {}) {
  const live = options.live !== false;
  const wantsOrderBook = options.wantsOrderBook === true;
  const [detail, setDetail] = useState<QuoteDetail | null>(null);
  const [orderBook, setOrderBook] = useState<OrderBook | null>(null);
  const [trades, setTrades] = useState<TradeTick[]>([]);
  const [loading, setLoading] = useState(true);
  const detailRef = useRef<QuoteDetail | null>(null);

  // symbolRef holds the symbol currently being displayed. `load` is recreated
  // per symbol, so comparing against its own closure variable can never detect
  // a stale response — a ref that the effect keeps current can.
  const symbolRef = useRef(symbol);

  useEffect(() => {
    detailRef.current = detail;
  }, [detail]);

  const load = useCallback(async () => {
    if (!symbol || !isPageVisible()) return;
    const requested = symbol;
    try {
      const snap = await fetchQuoteTerminalSnapshotClient(requested, wantsOrderBook);
      if (requested !== symbolRef.current) return;
      setDetail((prev) => {
        // The SSE stream pushes prices between polls; do not let an older
        // snapshot roll the displayed price backwards.
        if (prev && toMillis(prev.updatedAt) > toMillis(snap.detail?.updatedAt)) {
          return prev;
        }
        return snap.detail;
      });
      if (snap.orderBook) setOrderBook(snap.orderBook);
      setTrades(snap.trades ?? []);
    } catch {
      // keep last
    } finally {
      if (requested === symbolRef.current) setLoading(false);
    }
  }, [symbol, wantsOrderBook]);

  useEffect(() => {
    symbolRef.current = symbol;
    setDetail(null);
    setOrderBook(null);
    setTrades([]);
    setLoading(true);
  }, [symbol]);

  const closed = isClosedSession(detail);
  const nextSessionAt = detail?.nextSessionAt;

  /** 숨은 지 3시간이 안 됐는가 — 그 동안은 체결 스트림을 유지한다(`use-stream-awake`). */
  const streamAwake = useStreamAwake();

  /*
    첫 적재와 「다시 보이는 순간」은 **세션과 무관하게** 한 번 부른다.

    세션은 이 응답이 알려 주는 값이라, 모르는 채로 멈추면 아무것도 못 받는다. 탭을
    밤새 열어 두고 아침에 돌아온 경우도 이 경로이고, 그 한 번이 「이제 열렸다」를
    알려 준다 — 주기 폴링은 그때 아래에서 다시 켜진다.

    **주기 폴링과 이펙트를 나눈 이유**는 세션이 바뀔 때 요청이 한 번 더 나가지
    않게 하기 위해서다. 한 이펙트에 두면 `closed`가 바뀔 때마다 첫 줄의 `load()`가
    다시 돌아, 마감을 알아챈 그 순간에 같은 값을 한 번 더 받는다(실측 3회 → 2회).
  */
  useEffect(() => {
    void load();
    if (!live) return;
    const onVis = () => {
      if (isPageVisible()) void load();
    };
    document.addEventListener("visibilitychange", onVis);
    return () => document.removeEventListener("visibilitychange", onVis);
  }, [load, live]);

  // 주기 폴링은 **값이 움직이는 동안만**. 마감 중에는 타이머 자체를 걸지 않는다.
  useEffect(() => {
    if (!live || closed) return;
    const id = window.setInterval(load, POLL_MS);
    return () => window.clearInterval(id);
  }, [load, live, closed]);

  /*
    다음 창이 열리는 시각에 스스로 깨어난다 — 그 응답이 세션을 바꾸면 위 폴링이 켜진다.

    **시각을 모르면 5분마다 되묻는다.** 서버가 거래소 달력을 못 구했거나, 세션이
    SSE 이벤트로만 「닫힘」으로 넘어와 시각이 함께 오지 않은 경우다. 그때 타이머를
    아예 안 걸면 화면이 개장을 영영 못 알아채고 멈춘 값을 든 채로 남는다 —
    2초 폴링에 견주면 여전히 150분의 1이고, 멈추는 것보다는 낫다.
  */
  useEffect(() => {
    if (!live || !closed) return;
    const delay = delayUntil(nextSessionAt) ?? CLOSED_RECHECK_MS;
    const id = window.setTimeout(() => void load(), delay);
    return () => window.clearTimeout(id);
  }, [live, closed, nextSessionAt, load]);

  /*
    체결 스트림도 마감 중에는 열지 않는다. 열어 두면 서버가 그 종목을 「보고 있는
    종목」으로 세어 웹소켓 구독까지 따라붙는데, 흘러올 체결이 없다.

    **탭이 뒤로 가는 것만으로는 끊지 않는다** (2026-09-05 사용자 요청 5) — 규칙은
    `useStreamAwake`가 들고 있다(숨어도 3시간). 위의 폴링은 그대로 숨으면 멈춘다.
  */
  useEffect(() => {
    if (!live || !symbol || closed || !streamAwake) return;

    let es: EventSource | null = null;
    let cancelled = false;

    const connect = () => {
      // 보이는지는 더 안 본다 — 이 이펙트가 도는 것 자체가 「붙어 있을 때」라는 뜻이다.
      if (cancelled) return;
      es?.close();
      es = new EventSource(quotePriceStreamUrl(symbol));
      es.onmessage = (msg) => {
        try {
          const ev = JSON.parse(msg.data) as QuoteStreamEvent;
          if (ev.type !== "quote_update") return;
          setDetail((prev) => applyQuoteStreamEvent(prev ?? detailRef.current, ev));
        } catch {
          // ignore malformed
        }
      };
    };

    connect();
    /*
      숨을 때는 아무것도 하지 않는다. 다시 보일 때도 **죽어 있을 때만** 다시 붙는다 —
      이유는 `use-quote-stream`의 같은 자리에 적어 두었다.
    */
    const onVis = () => {
      if (!isPageVisible()) return;
      if (!es || es.readyState === EventSource.CLOSED) connect();
    };
    document.addEventListener("visibilitychange", onVis);

    return () => {
      cancelled = true;
      document.removeEventListener("visibilitychange", onVis);
      es?.close();
    };
  }, [symbol, live, closed, streamAwake]);

  return { detail, orderBook, trades, loading };
}
