"use client";

import { useEffect, useState } from "react";

import {
  fetchStockTimelineClient,
  fetchTimelineDayNewsClient,
  type StockTimeline,
  type TimelineNewsItem,
} from "@/lib/stock-timeline-api";

/**
 * 한 종목의 타임라인.
 *
 * 결과와 요청 키를 함께 들어 「불러오는 중」을 **파생값**으로 만든다 — 이펙트 안에서
 * setState를 동기로 부르지 않기 위해서다(저장소 린트 규칙, `use-score-backtest`와 같은 꼴).
 *
 * `null` 데이터와 오류를 가른다: 「기록이 아직 없다」와 「불러오지 못했다」는 화면이
 * 다르게 말해야 하는 두 상태다 (요구 8).
 */
export function useStockTimeline(symbol: string, days: number) {
  const key = `${symbol}:${days}`;
  const [loaded, setLoaded] = useState<{
    key: string;
    data: StockTimeline | null;
    error: string | null;
  }>({ key: "", data: null, error: null });

  useEffect(() => {
    if (!symbol) return;
    let cancelled = false;
    fetchStockTimelineClient(symbol, days)
      .then((res) => {
        if (!cancelled) setLoaded({ key, data: res, error: null });
      })
      .catch((err: unknown) => {
        if (cancelled) return;
        setLoaded({
          key,
          data: null,
          error: err instanceof Error ? err.message : "타임라인을 불러오지 못했습니다.",
        });
      });
    return () => {
      cancelled = true;
    };
    // symbol·days는 key에 녹아 있다.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [key]);

  const fresh = loaded.key === key;
  return {
    data: fresh ? loaded.data : null,
    loading: Boolean(symbol) && !fresh,
    error: fresh ? loaded.error : null,
  };
}

/**
 * 펼친 하루의 기사 — **열었을 때만** 부른다.
 *
 * 타임라인 본문은 건수만 싣는다(하루 평균 180건 × 90일). `date`가 비면 아무것도
 * 부르지 않는다 — 닫힌 행이 요청을 내면 목록을 훑는 것만으로 수십 번이 나간다.
 */
export function useTimelineDayNews(symbol: string, date: string | null) {
  const key = date ? `${symbol}:${date}` : "";
  const [loaded, setLoaded] = useState<{
    key: string;
    items: TimelineNewsItem[];
    error: string | null;
  }>({ key: "", items: [], error: null });

  useEffect(() => {
    if (!key || !date) return;
    let cancelled = false;
    fetchTimelineDayNewsClient(symbol, date)
      .then((res) => {
        if (!cancelled) setLoaded({ key, items: res?.items ?? [], error: null });
      })
      .catch((err: unknown) => {
        if (cancelled) return;
        setLoaded({
          key,
          items: [],
          error: err instanceof Error ? err.message : "뉴스를 불러오지 못했습니다.",
        });
      });
    return () => {
      cancelled = true;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [key]);

  const fresh = loaded.key === key;
  return {
    items: fresh ? loaded.items : [],
    loading: Boolean(key) && !fresh,
    error: fresh ? loaded.error : null,
  };
}
