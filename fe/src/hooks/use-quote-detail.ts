"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import type { OrderBook, QuoteDetail, QuoteStreamEvent, TradeTick } from "@/lib/quotes-types";
import { fetchQuoteTerminalSnapshotClient, quotePriceStreamUrl } from "@/lib/market-api";
import { isPageVisible } from "@/hooks/use-page-visible";

const POLL_MS = 2_000;

type UseQuoteDetailOptions = {
  /** Poll terminal snapshot (default true). Set false for analysis-only pages. */
  live?: boolean;
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
      const snap = await fetchQuoteTerminalSnapshotClient(requested);
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
  }, [symbol]);

  useEffect(() => {
    symbolRef.current = symbol;
    setDetail(null);
    setOrderBook(null);
    setTrades([]);
    setLoading(true);
  }, [symbol]);

  useEffect(() => {
    void load();
    if (!live) return;

    const id = window.setInterval(load, POLL_MS);
    const onVis = () => {
      if (isPageVisible()) void load();
    };
    document.addEventListener("visibilitychange", onVis);
    return () => {
      window.clearInterval(id);
      document.removeEventListener("visibilitychange", onVis);
    };
  }, [load, live]);

  useEffect(() => {
    if (!live || !symbol) return;

    let es: EventSource | null = null;
    let cancelled = false;

    const connect = () => {
      if (cancelled || !isPageVisible()) return;
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
    const onVis = () => {
      if (document.visibilityState === "visible") connect();
      else es?.close();
    };
    document.addEventListener("visibilitychange", onVis);

    return () => {
      cancelled = true;
      document.removeEventListener("visibilitychange", onVis);
      es?.close();
    };
  }, [symbol, live]);

  return { detail, orderBook, trades, loading };
}
