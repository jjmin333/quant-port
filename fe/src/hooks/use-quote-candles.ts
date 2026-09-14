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
import { PRICE_POLL_MS } from "@/lib/price-cadence";
import { isPageVisible } from "@/hooks/use-page-visible";
import { useStreamAwake } from "@/hooks/use-stream-awake";

// Fallback cadence for re-fetching the forming bar over REST. The SSE stream
// above is the primary path, but has no retry of its own if messages stop
// arriving after connecting (a buffering proxy between client and API can
// hold the connection open while never flushing chunks through) — without
// this, formingTime and the live-price overlay both go stale for the rest
// of the page's life. Shares the app-wide price cadence so the header and this
// forming-bar refresh land in step — a local copy of the number is how the two
// drift apart (see the price-cadence header).
const FORMING_BAR_POLL_MS = PRICE_POLL_MS;

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
 * @param live 지금 봉이 자라고 있는가. **장이 닫혀 있으면 false를 넘긴다** —
 *   그러면 아래 2초 스냅샷 폴링과 SSE를 둘 다 걷는다. 마감 중에는 만들어지는 봉이
 *   없어서 그 왕복이 매번 같은 응답을 받는다(2026-09-04 사용자 지적: 상세 화면
 *   하나가 `/candles/snapshot`을 2초마다 쳤다). 첫 적재는 이 값과 무관하게 한다.
 */
export function useQuoteCandles(
  symbol: string,
  interval: CandleInterval,
  livePrice = 0,
  live = true,
) {
  const [candles, setCandles] = useState<Candle[]>([]);
  const [formingTime, setFormingTime] = useState<string | null>(null);
  const [nextBefore, setNextBefore] = useState<string | null>(null);
  const [source, setSource] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [loadingMore, setLoadingMore] = useState(false);
  /** 숨은 지 3시간이 안 됐는가 — 그 동안은 봉 스트림을 유지한다(`use-stream-awake`). */
  const streamAwake = useStreamAwake();
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

  /*
    마감 중에는 스트림을 열지 않는다 — 자라는 봉이 없으므로 흘러올 이벤트도 없다.

    **탭이 뒤로 가는 것만으로는 끊지 않는다** (2026-09-05 사용자 요청 5) — 규칙은
    `useStreamAwake`가 들고 있다(숨어도 3시간). 아래 폴백 폴링은 그대로 숨으면 멈춘다.
  */
  useEffect(() => {
    if (!symbol || !live || !streamAwake) return;

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
      // 보이는지는 더 안 본다 — 이 이펙트가 도는 것 자체가 「붙어 있을 때」라는 뜻이다.
      if (cancelled) return;
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
  }, [symbol, interval, live, streamAwake]);

  // 봉이 자라지 않는 구간(마감)에는 폴백 폴링도 멈춘다 — 매번 같은 스냅샷을 받는다.
  useEffect(() => {
    if (!symbol || !live) return;
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
  }, [symbol, interval, live]);

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
