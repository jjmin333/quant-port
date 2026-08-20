"use client";

import { useEffect, useState } from "react";
import {
  MARKET_RADAR_POLL_MS,
  MARKET_RADAR_QUOTE_POLL_MS,
} from "@/lib/market-radar-config";
import { fetchMarketDataSettingsClient } from "@/lib/market-api";

export type MarketDataSettings = {
  rankingsPollRealtimeMinutes: number;
  rankingsPollShortMinutes: number;
  rankingsPollLongMinutes: number;
  quoteOverlayPollSeconds: number;
  rankingsClientPollSeconds: number;
  sharesPollHours: number;
  fundamentalsPollHours: number;
  updatedAt?: string;
};

const DEFAULTS: MarketDataSettings = {
  rankingsPollRealtimeMinutes: 5,
  rankingsPollShortMinutes: 15,
  rankingsPollLongMinutes: 60,
  quoteOverlayPollSeconds: MARKET_RADAR_QUOTE_POLL_MS / 1000,
  rankingsClientPollSeconds: MARKET_RADAR_POLL_MS / 1000,
  sharesPollHours: 24,
  fundamentalsPollHours: 24,
};

let cached: MarketDataSettings | null = null;

export function useMarketDataSettings() {
  const [settings, setSettings] = useState<MarketDataSettings>(cached ?? DEFAULTS);

  useEffect(() => {
    let cancelled = false;
    fetchMarketDataSettingsClient()
      .then((data) => {
        if (cancelled) return;
        cached = data;
        setSettings(data);
      })
      .catch(() => {
        if (!cancelled) setSettings(DEFAULTS);
      });
    return () => {
      cancelled = true;
    };
  }, []);

  return {
    settings,
    rankingsClientPollMs: Math.max(10, settings.rankingsClientPollSeconds) * 1000,
    quoteOverlayPollMs: Math.max(1, settings.quoteOverlayPollSeconds) * 1000,
    fundamentalsPollMs: Math.max(1, settings.fundamentalsPollHours) * 60 * 60 * 1000,
  };
}
