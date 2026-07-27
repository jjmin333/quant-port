"use client";

import { useCallback, useEffect, useState } from "react";
import type { QuoteFilter, QuoteSummary } from "@/lib/quotes-types";
import { fetchQuotesClient } from "@/lib/market-api";
import { isPageVisible } from "@/hooks/use-page-visible";

const POLL_MS = 5_000;

export function useQuotesList(filter: QuoteFilter, query: string) {
  const [items, setItems] = useState<QuoteSummary[]>([]);
  const [loading, setLoading] = useState(true);

  const load = useCallback(async () => {
    if (!isPageVisible()) return;
    try {
      setItems(await fetchQuotesClient(filter, query) ?? []);
    } catch {
      // keep last
    } finally {
      setLoading(false);
    }
  }, [filter, query]);

  useEffect(() => {
    setLoading(true);
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

  return { items, loading, refresh: load };
}
