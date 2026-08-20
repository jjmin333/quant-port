"use client";

import { useEffect, useState } from "react";
import { fetchStockSummaryClient } from "@/lib/stock-summary-api";
import type { StockSummaryData } from "@/lib/stock-summary-types";

/**
 * Shared AI 요약 fetch. The header line and the AI 분석 tab both need the same
 * payload, and the endpoint can synchronously build a summary on a cold cache,
 * so they must not each trigger their own request.
 */
export function useStockSummary(symbol: string) {
  const [data, setData] = useState<StockSummaryData | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!symbol) {
      setData(null);
      setLoading(false);
      return;
    }
    let cancelled = false;
    setLoading(true);
    setError(null);
    fetchStockSummaryClient(symbol)
      .then((res) => {
        if (!cancelled) setData(res);
      })
      .catch((e) => {
        if (!cancelled) {
          setData(null);
          setError(e instanceof Error ? e.message : "AI 요약 조회 실패");
        }
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [symbol]);

  return { data, loading, error };
}

/**
 * The single sentence shown in the header. Prefers the valuation line, falling
 * back through the other sections so a symbol with partial data still says
 * something useful instead of rendering an empty strip.
 */
export function stockSummaryHeadline(data: StockSummaryData | null): string {
  if (!data) return "";
  for (const section of [data.valuation, data.technical, data.news, data.insider]) {
    const trimmed = section?.trim();
    if (trimmed) return trimmed;
  }
  return "";
}
