"use client";

import { useEffect, useState } from "react";
import { fetchExchangeRateClient } from "@/lib/market-api";

let cachedRate = 0;
let cachedAt = 0;
let inflight: Promise<number> | null = null;

/** Client-side cache — BE poller keeps data fresh; avoid hammering the API. */
const CACHE_MS = 5 * 60_000;

async function loadExchangeRate(): Promise<number> {
  if (cachedRate > 0 && Date.now() - cachedAt < CACHE_MS) {
    return cachedRate;
  }
  if (inflight) {
    return inflight;
  }
  inflight = fetchExchangeRateClient()
    .then((next) => {
      if (next > 0) {
        cachedRate = next;
        cachedAt = Date.now();
      }
      return cachedRate;
    })
    .catch(() => cachedRate)
    .finally(() => {
      inflight = null;
    });
  return inflight;
}

export function seedExchangeRate(rate: number) {
  if (rate > 0) {
    cachedRate = rate;
    cachedAt = Date.now();
  }
}

export function useExchangeRate(enabled = true): number {
  const [rate, setRate] = useState(() => (enabled && cachedRate > 0 ? cachedRate : 0));

  useEffect(() => {
    if (!enabled) return;
    let cancelled = false;
    void loadExchangeRate().then((next) => {
      if (!cancelled && next > 0) setRate(next);
    });
    return () => {
      cancelled = true;
    };
  }, [enabled]);

  return rate;
}

export function resolveExchangeRate(explicit?: number): number {
  if (explicit != null && explicit > 0) return explicit;
  return cachedRate;
}
