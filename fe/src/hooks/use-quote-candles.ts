"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import type { Candle, CandleInterval, ChartStyle } from "@/lib/quotes-types";
import { candleTimeSortKey, mergeCandlesByTime } from "@/lib/chart-time";
import {
  fetchCandleSnapshotClient,
  fetchCandlesPageClient,
  quoteStreamUrl,
} from "@/lib/market-api";
import type { StreamEvent } from "@/lib/quotes-types";

// Fallback cadence for re-fetching the forming bar over REST. The SSE stream
// above is the primary path, but has no retry of its own if messages stop
// arriving after connecting (a buffering proxy between client and API can
// hold the connection open while never flushing chunks through) — without
// this, formingTime and the live-price overlay both go stale for the rest
// of the page's life. Matches useQuoteDetail's poll interval so the header
// and this forming-bar refresh land in step.
const FORMING_BAR_POLL_MS = 2_000;

function candlesFromSnapshot(
  completed: Candle[],
  forming: Candle | null | undefined,
  interval: CandleInterval,
): Candle[] {
  if (!forming) return completed;
  return mergeCandlesByTime(completed, [forming], interval);
}

/**
 * Redraws the in-progress bar at `livePrice`, mirroring the patch the backend
 * applies when it builds a snapshot. `formingTime` comes from the server, so a
 * completed bar is never rewritten — once the bucket closes there is nothing to
 * patch until the next one opens.
 */
function applyLivePrice(
  candles: Candle[],
  formingTime: string | null,
  interval: CandleInterval,
  livePrice: number,
): Candle[] {
  if (livePrice <= 0 || !formingTime || candles.length === 0) return candles;
  const key = candleTimeSortKey(formingTime, interval);
  const idx = candles.findIndex((c) => candleTimeSortKey(c.time, interval) === key);
  if (idx < 0) return candles;

  const bar = candles[idx]!;
  if (bar.close === livePrice && bar.high >= livePrice && bar.low <= livePrice) return candles;

  const patched = candles.slice();
  patched[idx] = {
    ...bar,
    close: livePrice,
    high: Math.max(bar.high, livePrice),
    low: bar.low > 0 ? Math.min(bar.low, livePrice) : livePrice,
  };
  return patched;
}

/**
 * @param livePrice Current quote price. The backend only emits `bar_update` on
 *   its poller's fast tick, which backs off under upstream rate limits, so the
 *   chart would otherwise sit at the price it loaded with while the header
 *   ticks. Pass 0 to disable the overlay.
 */
export function useQuoteCandles(symbol: string, interval: CandleInterval, livePrice = 0) {
  const [candles, setCandles] = useState<Candle[]>([]);
  const [formingTime, setFormingTime] = useState<string | null>(null);
  const [nextBefore, setNextBefore] = useState<string | null>(null);
  const [source, setSource] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [loadingMore, setLoadingMore] = useState(false);
  const loadingMoreRef = useRef(false);
  const nextBeforeRef = useRef<string | null>(null);
  const revRef = useRef(0);

  useEffect(() => {
    loadingMoreRef.current = loadingMore;
  }, [loadingMore]);

  useEffect(() => {
    nextBeforeRef.current = nextBefore;
  }, [nextBefore]);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    setCandles([]);
    setFormingTime(null);
    setNextBefore(null);
    setSource(null);
    revRef.current = 0;

    fetchCandleSnapshotClient(symbol, interval)
      .then((snap) => {
        if (cancelled) return;
        setCandles(candlesFromSnapshot(snap.completed, snap.forming, interval));
        setFormingTime(snap.forming?.time ?? null);
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
        setFormingTime(ev.forming?.time ?? null);
        return;
      }

      if (ev.type === "bar_close" && ev.bar) {
        setCandles((prev) => mergeCandlesByTime(prev, [ev.bar!], interval));
        return;
      }

      if (ev.type === "bar_update" && ev.forming) {
        // Merge by bucket key — dropping the tail positionally would discard the
        // newest completed bar whenever no forming bar was in the array yet.
        setCandles((prev) => mergeCandlesByTime(prev, [ev.forming!], interval));
        setFormingTime(ev.forming.time);
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

  useEffect(() => {
    if (!symbol) return;
    let cancelled = false;

    const poll = () => {
      if (document.visibilityState !== "visible") return;
      fetchCandleSnapshotClient(symbol, interval)
        .then((snap) => {
          if (cancelled) return;
          if (snap.rev && snap.rev < revRef.current) return;
          if (snap.rev) revRef.current = snap.rev;
          const merged = candlesFromSnapshot(snap.completed, snap.forming, interval);
          setCandles((prev) => mergeCandlesByTime(prev, merged, interval));
          setFormingTime(snap.forming?.time ?? null);
        })
        .catch(() => {
          // keep last known state; retried on the next tick
        });
    };

    const id = window.setInterval(poll, FORMING_BAR_POLL_MS);
    return () => {
      cancelled = true;
      window.clearInterval(id);
    };
  }, [symbol, interval]);

  const loadMore = useCallback(async () => {
    const cursor = nextBeforeRef.current;
    if (!symbol || !cursor || loadingMoreRef.current) return;
    loadingMoreRef.current = true;
    setLoadingMore(true);
    try {
      const page = await fetchCandlesPageClient(symbol, interval, cursor);
      setCandles((prev) => mergeCandlesByTime(prev, page.candles, interval, false));
      setNextBefore(page.nextBefore);
    } catch {
      // keep last
    } finally {
      loadingMoreRef.current = false;
      setLoadingMore(false);
    }
  }, [symbol, interval]);

  const liveCandles = useMemo(
    () => applyLivePrice(candles, formingTime, interval, livePrice),
    [candles, formingTime, interval, livePrice],
  );

  return {
    candles: liveCandles,
    loading,
    loadMore,
    hasMore: Boolean(nextBefore),
    loadingMore,
    source,
  };
}
