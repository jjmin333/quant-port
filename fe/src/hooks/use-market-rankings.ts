"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { useAuth } from "@/components/providers/auth-provider";
import { isPageVisible } from "@/hooks/use-page-visible";
import { fetchUserHoldingsClient } from "@/lib/holdings-import-api";
import {
  MARKET_RADAR_DEFAULT_COUNT,
  marketRadarParamsKey,
  marketRadarSearchParams,
  parseMarketRadarParams,
} from "@/lib/market-radar-config";
import { useMarketDataSettings } from "@/hooks/use-market-data-settings";
import type { EnrichedRankingItem, MarketRadarParams, RankingsResponse } from "@/lib/market-radar-types";
import type { StockInfo } from "@/lib/quotes-types";
import {
  fetchRankingsClient,
  fetchStocksBatchClient,
  fetchWatchlistClient,
} from "@/lib/market-api";

function computeRankDeltas(
  prev: Map<string, number>,
  items: RankingsResponse["rankings"],
): Map<string, number | null> {
  const out = new Map<string, number | null>();
  for (const item of items) {
    const before = prev.get(item.symbol);
    out.set(item.symbol, before != null ? before - item.rank : null);
  }
  return out;
}

function enrichRows(
  resp: RankingsResponse,
  stockMap: Map<string, StockInfo>,
  watch: Set<string>,
  held: Set<string>,
  rankDeltas: Map<string, number | null>,
): EnrichedRankingItem[] {
  return resp.rankings.map((row) => {
    const stock = stockMap.get(row.symbol.toUpperCase());
    const sym = row.symbol.toUpperCase();
    return {
      ...row,
      name: stock?.name ?? row.symbol,
      market: stock?.market ?? "—",
      assetType: stock?.assetType,
      logoUrl: stock?.logoUrl,
      rankDelta: rankDeltas.get(row.symbol) ?? null,
      isWatchlisted: watch.has(sym),
      isHeld: held.has(sym),
    };
  });
}

function patchWatchlistFlag(
  rows: EnrichedRankingItem[],
  symbol: string,
  watchlisted: boolean,
): EnrichedRankingItem[] {
  const sym = symbol.toUpperCase();
  return rows.map((item) =>
    item.symbol.toUpperCase() === sym ? { ...item, isWatchlisted: watchlisted } : item,
  );
}

function scheduleIdle(task: () => void): () => void {
  if (typeof window.requestIdleCallback === "function") {
    const id = window.requestIdleCallback(task, { timeout: 2000 });
    return () => window.cancelIdleCallback(id);
  }
  const id = window.setTimeout(task, 300);
  return () => window.clearTimeout(id);
}

export function useMarketRankings(params: MarketRadarParams) {
  const { session } = useAuth();
  const { rankingsClientPollMs } = useMarketDataSettings();
  const paramsKey = marketRadarParamsKey(params);

  const [data, setData] = useState<RankingsResponse | null>(null);
  const [items, setItems] = useState<EnrichedRankingItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const prevRanksRef = useRef<Map<string, number>>(new Map());
  const stockCacheRef = useRef<Map<string, StockInfo>>(new Map());
  const watchHeldRef = useRef<{ watch: Set<string>; held: Set<string> }>({
    watch: new Set(),
    held: new Set(),
  });
  const mountedRef = useRef(true);
  const enrichingRef = useRef(false);
  const lastRespRef = useRef<RankingsResponse | null>(null);

  useEffect(() => {
    mountedRef.current = true;
    return () => {
      mountedRef.current = false;
    };
  }, []);

  const loadWatchHeld = useCallback(async () => {
    if (!session) {
      watchHeldRef.current = { watch: new Set(), held: new Set() };
      return;
    }
    try {
      const [watchlist, holdings] = await Promise.all([
        fetchWatchlistClient().catch(() => []),
        fetchUserHoldingsClient().catch(() => []),
      ]);
      if (!mountedRef.current) return;
      watchHeldRef.current = {
        watch: new Set(watchlist.map((w) => w.symbol.toUpperCase())),
        held: new Set(holdings.map((h) => h.symbol.toUpperCase())),
      };
      const resp = lastRespRef.current;
      if (resp) {
        const rankDeltas = computeRankDeltas(prevRanksRef.current, resp.rankings);
        setItems(
          enrichRows(
            resp,
            stockCacheRef.current,
            watchHeldRef.current.watch,
            watchHeldRef.current.held,
            rankDeltas,
          ),
        );
      }
    } catch {
      if (mountedRef.current) {
        watchHeldRef.current = { watch: new Set(), held: new Set() };
      }
    }
  }, [session]);

  const enrichStockMeta = useCallback(
    async (symbols: string[], resp: RankingsResponse, rankDeltas: Map<string, number | null>) => {
      if (symbols.length === 0 || enrichingRef.current) return;
      enrichingRef.current = true;
      try {
        const missing = symbols.filter((s) => !stockCacheRef.current.has(s.toUpperCase()));
        if (missing.length > 0) {
          const stocks = await fetchStocksBatchClient(missing);
          if (!mountedRef.current) return;
          for (const st of stocks) {
            stockCacheRef.current.set(st.symbol.toUpperCase(), st);
          }
        }
        if (!mountedRef.current) return;
        const { watch, held } = watchHeldRef.current;
        setItems(enrichRows(resp, stockCacheRef.current, watch, held, rankDeltas));
      } catch {
        // keep symbol-only rows
      } finally {
        enrichingRef.current = false;
      }
    },
    [],
  );

  const syncWatchlistSymbol = useCallback((symbol: string, watchlisted: boolean) => {
    const sym = symbol.trim().toUpperCase();
    if (!sym) return;
    const { held } = watchHeldRef.current;
    const watch = new Set(watchHeldRef.current.watch);
    if (watchlisted) watch.add(sym);
    else watch.delete(sym);
    watchHeldRef.current = { watch, held };
    setItems((prev) => patchWatchlistFlag(prev, sym, watchlisted));
  }, []);

  const refresh = useCallback(
    async (initial = false) => {
      if (!isPageVisible() || !mountedRef.current) return;
      try {
        if (initial) setError(null);

        const resp = await fetchRankingsClient({
          type: params.type,
          marketCountry: params.market,
          duration: params.duration,
          count: MARKET_RADAR_DEFAULT_COUNT,
        });
        if (!mountedRef.current) return;

        lastRespRef.current = resp;
        const rankDeltas = computeRankDeltas(prevRanksRef.current, resp.rankings);
        const nextPrev = new Map<string, number>();
        for (const row of resp.rankings) {
          nextPrev.set(row.symbol, row.rank);
        }
        prevRanksRef.current = nextPrev;

        const { watch, held } = watchHeldRef.current;
        const rows = enrichRows(resp, stockCacheRef.current, watch, held, rankDeltas);

        setData(resp);
        setItems(rows);
        if (initial) setLoading(false);

        scheduleIdle(() => {
          void enrichStockMeta(
            resp.rankings.map((r) => r.symbol),
            resp,
            rankDeltas,
          );
        });
      } catch (e) {
        if (!mountedRef.current) return;
        setError(e instanceof Error ? e.message : "랭킹을 불러오지 못했습니다.");
        if (initial) setLoading(false);
      }
    },
    [params.type, params.market, params.duration, enrichStockMeta],
  );

  useEffect(() => {
    const cancel = scheduleIdle(() => {
      void loadWatchHeld();
    });
    return cancel;
  }, [loadWatchHeld]);

  useEffect(() => {
    prevRanksRef.current = new Map();
    setLoading(true);
    setError(null);
    void refresh(true);
  }, [paramsKey, refresh]);

  useEffect(() => {
    const id = window.setInterval(() => void refresh(false), rankingsClientPollMs);
    return () => window.clearInterval(id);
  }, [refresh, rankingsClientPollMs]);

  return { data, items, loading, error, refresh, syncWatchlistSymbol };
}

export function syncMarketRadarUrl(
  router: { replace: (url: string, opts?: { scroll?: boolean }) => void },
  params: MarketRadarParams,
  symbol?: string | null,
) {
  const q = marketRadarSearchParams(params);
  if (symbol) q.set("symbol", symbol);
  router.replace(`/market?${q.toString()}`, { scroll: false });
}

export function parseMarketRadarFromSearchParams(searchParams: URLSearchParams): MarketRadarParams {
  return parseMarketRadarParams(searchParams);
}
