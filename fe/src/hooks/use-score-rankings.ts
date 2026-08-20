"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { useAuth } from "@/components/providers/auth-provider";
import { fetchUserHoldingsClient } from "@/lib/holdings-import-api";
import { fetchWatchlistClient } from "@/lib/market-api";
import {
  fetchScoreRankingsClient,
  type ScoreRankingMarket,
  type StockScoreListItem,
} from "@/lib/stock-score-api";

export type EnrichedScoreItem = StockScoreListItem & {
  isWatchlisted: boolean;
  isHeld: boolean;
};

/**
 * Score-ranked 국내/해외 leaderboard for the 종목 분석 tab. Unlike market radar's
 * live rankings, this is once-a-day batch data, so no polling — just refetch
 * when the market tab changes.
 */
export function useScoreRankings(market: ScoreRankingMarket, limit = 20) {
  const { session } = useAuth();
  const [items, setItems] = useState<EnrichedScoreItem[]>([]);
  const [tradeDate, setTradeDate] = useState("");
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const watchHeldRef = useRef<{ watch: Set<string>; held: Set<string> }>({
    watch: new Set(),
    held: new Set(),
  });

  const enrich = useCallback((rows: StockScoreListItem[]): EnrichedScoreItem[] => {
    const { watch, held } = watchHeldRef.current;
    return rows.map((row) => {
      const sym = row.symbol.toUpperCase();
      return { ...row, isWatchlisted: watch.has(sym), isHeld: held.has(sym) };
    });
  }, []);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    setError(null);
    fetchScoreRankingsClient(market, limit)
      .then(({ items: rows, tradeDate: date }) => {
        if (cancelled) return;
        setItems(enrich(rows));
        setTradeDate(date);
      })
      .catch((e) => {
        if (cancelled) return;
        setError(e instanceof Error ? e.message : "종목 랭킹을 불러오지 못했습니다.");
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [market, limit, enrich]);

  useEffect(() => {
    if (!session) {
      watchHeldRef.current = { watch: new Set(), held: new Set() };
      setItems((prev) => enrich(prev));
      return;
    }
    let cancelled = false;
    Promise.all([
      fetchWatchlistClient().catch(() => []),
      fetchUserHoldingsClient().catch(() => []),
    ]).then(([watchlist, holdings]) => {
      if (cancelled) return;
      watchHeldRef.current = {
        watch: new Set(watchlist.map((w) => w.symbol.toUpperCase())),
        held: new Set(holdings.map((h) => h.symbol.toUpperCase())),
      };
      setItems((prev) => enrich(prev));
    });
    return () => {
      cancelled = true;
    };
  }, [session, enrich]);

  return { items, tradeDate, loading, error };
}
