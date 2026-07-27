"use client";

import { useCallback, useEffect, useState } from "react";
import type { OrderBook, QuoteDetail, TradeTick } from "@/lib/quotes-types";
import { fetchQuoteTerminalSnapshotClient } from "@/lib/market-api";
import { isPageVisible } from "@/hooks/use-page-visible";

const POLL_MS = 5_000;

export function useQuoteDetail(symbol: string) {
  const [detail, setDetail] = useState<QuoteDetail | null>(null);
  const [orderBook, setOrderBook] = useState<OrderBook | null>(null);
  const [trades, setTrades] = useState<TradeTick[]>([]);
  const [loading, setLoading] = useState(true);

  const load = useCallback(async () => {
    if (!symbol || !isPageVisible()) return;
    const requested = symbol;
    try {
      const snap = await fetchQuoteTerminalSnapshotClient(requested);
      if (requested !== symbol) return;
      setDetail(snap.detail);
      if (snap.orderBook) setOrderBook(snap.orderBook);
      setTrades(snap.trades ?? []);
    } catch {
      // keep last
    } finally {
      if (requested === symbol) setLoading(false);
    }
  }, [symbol]);

  useEffect(() => {
    setDetail(null);
    setOrderBook(null);
    setTrades([]);
    setLoading(true);
  }, [symbol]);

  useEffect(() => {
    void load();
    const id = window.setInterval(load, POLL_MS);
    const onVis = () => {
      if (isPageVisible()) void load();
    };
    document.addEventListener("visibilitychange", onVis);
    return () => {
      window.clearInterval(id);
      document.removeEventListener("visibilitychange", onVis);
    };
  }, [load]);

  return { detail, orderBook, trades, loading };
}
