"use client";

import { useCallback, useEffect, useState } from "react";
import type { IndexGroupMeta, MarketIndex } from "@/lib/quotes-types";
import { fetchMarketIndicesClient } from "@/lib/market-api";
import { mergeMarketIndices, MARKET_INDEX_PLACEHOLDERS } from "@/lib/market-indices-catalog";
import { isPageVisible } from "@/hooks/use-page-visible";

const POLL_MS = 30_000;

export function useMarketIndices() {
  const [indices, setIndices] = useState<MarketIndex[]>(MARKET_INDEX_PLACEHOLDERS);
  const [updatedAt, setUpdatedAt] = useState<string | null>(null);
  const [groups, setGroups] = useState<Record<string, IndexGroupMeta>>({});

  const load = useCallback(async () => {
    if (!isPageVisible()) return;
    try {
      const { indices: next, updatedAt: at, groups: nextGroups } = await fetchMarketIndicesClient();
      setIndices(mergeMarketIndices(next));
      if (at) setUpdatedAt(at);
      if (Object.keys(nextGroups).length > 0) setGroups(nextGroups);
    } catch {
      // keep last snapshot
    }
  }, []);

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

  return { indices, updatedAt, groups };
}
