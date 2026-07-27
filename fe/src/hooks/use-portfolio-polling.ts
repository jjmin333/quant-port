"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import type { PortfolioSummary } from "@/lib/portfolio-types";
import {
  fetchPortfolioLiveClient,
  fetchPortfolioSummaryClient,
} from "@/lib/api-client";

const DEFAULT_LIVE_INTERVAL_MS = 5_000;
const DEFAULT_FULL_INTERVAL_MS = 60_000;
const MAX_BACKOFF_MS = 120_000;

function parseError(err: unknown): string {
  let message = err instanceof Error ? err.message : "갱신 실패";
  if (message === "Failed to fetch" || message.includes("NetworkError")) {
    return "백엔드 서버에 연결할 수 없습니다 (BE :8080 확인)";
  }
  if (message.includes("429") || message.toLowerCase().includes("limit")) {
    return "API 호출 한도 초과 — 잠시 후 자동 재시도합니다";
  }
  return message;
}

function resolveLiveIntervalMs(summary: PortfolioSummary): number {
  const sec = summary.liveRefreshIntervalSec ?? DEFAULT_LIVE_INTERVAL_MS / 1000;
  if (sec <= 0) return 0;
  return Math.max(sec * 1000, DEFAULT_LIVE_INTERVAL_MS);
}

function resolveFullIntervalMs(summary: PortfolioSummary): number {
  const sec =
    summary.fullRefreshIntervalSec ??
    summary.refreshIntervalSec ??
    DEFAULT_FULL_INTERVAL_MS / 1000;
  if (sec <= 0) return 0;
  return Math.max(sec * 1000, DEFAULT_FULL_INTERVAL_MS);
}

export function usePortfolioPolling(initial: PortfolioSummary) {
  const [summary, setSummary] = useState(initial);
  const [isLiveRefreshing, setIsLiveRefreshing] = useState(false);
  const [isFullRefreshing, setIsFullRefreshing] = useState(false);
  const [lastError, setLastError] = useState<string | null>(null);
  const backoffRef = useRef(0);
  const liveTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const fullTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const summaryRef = useRef(initial);

  useEffect(() => {
    summaryRef.current = summary;
  }, [summary]);

  const refreshLive = useCallback(async () => {
    setIsLiveRefreshing(true);
    try {
      const next = await fetchPortfolioLiveClient();
      setSummary(next);
      summaryRef.current = next;
      setLastError(null);
      backoffRef.current = 0;
    } catch (err) {
      setLastError(parseError(err));
      if (parseError(err).includes("한도")) {
        const liveMs = resolveLiveIntervalMs(summaryRef.current);
        backoffRef.current = Math.min(
          backoffRef.current === 0 ? liveMs * 2 : backoffRef.current * 2,
          MAX_BACKOFF_MS,
        );
      }
    } finally {
      setIsLiveRefreshing(false);
    }
  }, []);

  const refreshFull = useCallback(async () => {
    setIsFullRefreshing(true);
    try {
      const next = await fetchPortfolioSummaryClient();
      setSummary(next);
      summaryRef.current = next;
      setLastError(null);
      backoffRef.current = 0;
    } catch (err) {
      setLastError(parseError(err));
    } finally {
      setIsFullRefreshing(false);
    }
  }, []);

  useEffect(() => {
    setSummary(initial);
    summaryRef.current = initial;
  }, [initial]);

  useEffect(() => {
    const scheduleLive = () => {
      const liveMs = resolveLiveIntervalMs(summaryRef.current);
      if (liveMs <= 0) return;

      liveTimerRef.current = setTimeout(async () => {
        await refreshLive();
        scheduleLive();
      }, liveMs + backoffRef.current);
    };

    const scheduleFull = () => {
      const fullMs = resolveFullIntervalMs(summaryRef.current);
      if (fullMs <= 0) return;

      fullTimerRef.current = setTimeout(async () => {
        await refreshFull();
        scheduleFull();
      }, fullMs);
    };

    scheduleLive();
    scheduleFull();

    return () => {
      if (liveTimerRef.current) clearTimeout(liveTimerRef.current);
      if (fullTimerRef.current) clearTimeout(fullTimerRef.current);
    };
  }, [refreshLive, refreshFull, summary.pollingMode, summary.liveRefreshIntervalSec, summary.fullRefreshIntervalSec]);

  return {
    summary,
    isRefreshing: isLiveRefreshing || isFullRefreshing,
    isLiveRefreshing,
    isFullRefreshing,
    lastError,
    refreshLive,
    refreshFull,
  };
}
