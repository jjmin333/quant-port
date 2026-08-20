"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { useAuth } from "@/components/providers/auth-provider";
import { addWatchlistClient, fetchWatchlistClient, removeWatchlistClient } from "@/lib/market-api";

type UseWatchlistToggleOptions = {
  loginNext?: string;
  onChange?: (symbol: string, watchlisted: boolean) => void;
};

export function useWatchlistToggle(symbol: string, options: UseWatchlistToggleOptions = {}) {
  const { session } = useAuth();
  const router = useRouter();
  const [watchlisted, setWatchlisted] = useState(false);
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    if (!session || !symbol) {
      setWatchlisted(false);
      return;
    }
    fetchWatchlistClient()
      .then((list) => setWatchlisted(list.some((item) => item.symbol === symbol)))
      .catch(() => setWatchlisted(false));
  }, [symbol, session]);

  async function toggleWatchlist() {
    if (!session) {
      const next = options.loginNext ?? "/";
      router.push(`/login?next=${encodeURIComponent(next)}`);
      return;
    }
    if (busy || !symbol) return;

    const prev = watchlisted;
    const next = !watchlisted;
    setWatchlisted(next);
    options.onChange?.(symbol, next);
    setBusy(true);

    try {
      if (next) await addWatchlistClient(symbol);
      else await removeWatchlistClient(symbol);
    } catch {
      setWatchlisted(prev);
      options.onChange?.(symbol, prev);
    } finally {
      setBusy(false);
    }
  }

  return { watchlisted, busy, toggleWatchlist };
}
