"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import type { Candle, CandleInterval, ChartStyle } from "@/lib/quotes-types";
import { mergeCandlesByTime } from "@/lib/chart-time";
import {
  fetchCandleSnapshotClient,
  fetchCandlesPageClient,
  quoteStreamUrl,
} from "@/lib/market-api";
import type { StreamEvent } from "@/lib/quotes-types";

function candlesFromSnapshot(
  completed: Candle[],
  forming: Candle | null | undefined,
  interval: CandleInterval,
): Candle[] {
  if (!forming) return completed;
  return mergeCandlesByTime(completed, [forming], interval);
}

export function useQuoteCandles(symbol: string, interval: CandleInterval) {
  const [candles, setCandles] = useState<Candle[]>([]);
  const [nextBefore, setNextBefore] = useState<string | null>(null);
  const [source, setSource] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [loadingMore, setLoadingMore] = useState(false);
  const loadingMoreRef = useRef(false);
  const revRef = useRef(0);

  useEffect(() => {
    loadingMoreRef.current = loadingMore;
  }, [loadingMore]);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    setCandles([]);
    setNextBefore(null);
    setSource(null);
    revRef.current = 0;

    fetchCandleSnapshotClient(symbol, interval)
      .then((snap) => {
        if (cancelled) return;
        setCandles(candlesFromSnapshot(snap.completed, snap.forming, interval));
        setNextBefore(snap.nextBefore ?? null);
        setSource(snap.source ?? null);
        revRef.current = snap.rev;
      })
      .catch(() => {
        if (cancelled) return;
        return fetchCandlesPageClient(symbol, interval)
          .then((page) => {
            if (cancelled) return;
            setCandles(page.candles);
            setNextBefore(page.nextBefore);
            setSource(page.source ?? null);
          })
          .catch(() => {
            if (!cancelled) {
              setCandles([]);
              setNextBefore(null);
            }
          });
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });

    return () => {
      cancelled = true;
    };
  }, [symbol, interval]);

  useEffect(() => {
    if (!symbol) return;

    let es: EventSource | null = null;
    let cancelled = false;

    const applyEvent = (ev: StreamEvent) => {
      if (ev.rev && ev.rev < revRef.current) return;
      if (ev.rev) revRef.current = ev.rev;

      if (ev.type === "snapshot_bars" && ev.completed) {
        const snap = candlesFromSnapshot(ev.completed, ev.forming, interval);
        setCandles((prev) => mergeCandlesByTime(prev, snap, interval));
        return;
      }

      if (ev.type === "bar_close" && ev.bar) {
        setCandles((prev) => mergeCandlesByTime(prev, [ev.bar!], interval));
        return;
      }

      if (ev.type === "bar_update" && ev.forming) {
        setCandles((prev) => {
          const withoutForming = prev.length > 0 ? prev.slice(0, -1) : prev;
          const base =
            withoutForming.length > 0 &&
            withoutForming[withoutForming.length - 1]?.time === ev.forming!.time
              ? withoutForming.slice(0, -1)
              : withoutForming;
          return mergeCandlesByTime(base, [ev.forming!], interval);
        });
      }
    };

    const connect = () => {
      if (cancelled || document.visibilityState !== "visible") return;
      es?.close();
      es = new EventSource(quoteStreamUrl(symbol, interval));

      es.onmessage = (msg) => {
        try {
          applyEvent(JSON.parse(msg.data) as StreamEvent);
        } catch {
          // ignore malformed
        }
      };

      es.addEventListener("snapshot_bars", (msg) => {
        try {
          const ev = JSON.parse((msg as MessageEvent).data) as StreamEvent;
          applyEvent({ ...ev, type: "snapshot_bars" });
        } catch {
          // ignore
        }
      });
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
  }, [symbol, interval]);

  const loadMore = useCallback(async () => {
    if (!symbol || !nextBefore || loadingMoreRef.current) return;
    loadingMoreRef.current = true;
    setLoadingMore(true);
    try {
      const page = await fetchCandlesPageClient(symbol, interval, nextBefore);
      setCandles((prev) => mergeCandlesByTime(prev, page.candles, interval, false));
      setNextBefore(page.nextBefore);
    } catch {
      // keep last
    } finally {
      loadingMoreRef.current = false;
      setLoadingMore(false);
    }
  }, [symbol, interval, nextBefore]);

  return {
    candles,
    loading,
    loadMore,
    hasMore: Boolean(nextBefore),
    loadingMore,
    source,
  };
}
