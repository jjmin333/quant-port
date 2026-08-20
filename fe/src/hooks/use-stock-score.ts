"use client";

import { useEffect, useState } from "react";
import { fetchStockScoreClient, type StockScoreData } from "@/lib/stock-score-api";

/**
 * Shared 종합 점수 fetch. The AI 분석 tab shows the full breakdown and several
 * other tabs show just their own axis, so the workspace fetches once at the top
 * and passes the result down rather than each tab issuing its own request.
 */
export function useStockScore(symbol: string) {
  const [data, setData] = useState<StockScoreData | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    if (!symbol) {
      setData(null);
      setLoading(false);
      return;
    }
    let cancelled = false;
    setLoading(true);
    fetchStockScoreClient(symbol)
      .then((res) => {
        if (!cancelled) setData(res);
      })
      .catch(() => {
        // A symbol the batch has not scored yet is a normal state, not an
        // error — the UI simply shows nothing.
        if (!cancelled) setData(null);
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [symbol]);

  return { data, loading };
}
