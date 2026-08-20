"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import {
  fetchMyResearchCallsClient,
  fetchResearchCallsClient,
  fetchResearchSummaryClient,
  retractResearchClient,
} from "@/lib/research-api";
import {
  type ResearchCall,
  type ResearchDirection,
  type ResearchSummary,
} from "@/lib/research-types";

export type ResearchScope = "feed" | "mine";
export type ResearchSort = "recent" | "views" | "likes";

/**
 * Loads a page of reports and filters/sorts client-side.
 *
 * Raw return is deliberately absent as a sort: ordering a feed by headline gain
 * puts whatever the market carried at the top, which is the exact failure this
 * feature exists to avoid.
 */
export function useResearchCalls(initialScope: ResearchScope = "feed") {
  const [scope, setScope] = useState<ResearchScope>(initialScope);
  const [calls, setCalls] = useState<ResearchCall[]>([]);
  const [summary, setSummary] = useState<ResearchSummary | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const [directionFilter, setDirectionFilter] = useState<ResearchDirection | "all">("all");
  const [contrarianOnly, setContrarianOnly] = useState(false);
  const [sort, setSort] = useState<ResearchSort>("recent");

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const list =
        scope === "mine"
          ? await fetchMyResearchCallsClient({ limit: 200 })
          : await fetchResearchCallsClient({ limit: 200 });
      setCalls(list);
      // The scorecard is always the caller's own, whichever tab is showing.
      const stats = await fetchResearchSummaryClient().catch(() => null);
      setSummary(stats);
    } catch {
      setError("리포트를 불러오지 못했습니다.");
    } finally {
      setLoading(false);
    }
  }, [scope]);

  useEffect(() => {
    load();
  }, [load]);

  const retract = useCallback(async (id: number) => {
    const updated = await retractResearchClient(id);
    setCalls((prev) => prev.map((c) => (c.id === id ? updated : c)));
    const stats = await fetchResearchSummaryClient().catch(() => null);
    if (stats) setSummary(stats);
  }, []);

  const filtered = useMemo(() => {
    const rows = calls.filter((c) => {
      if (directionFilter !== "all" && c.direction !== directionFilter) return false;
      if (contrarianOnly && !c.contrarian) return false;
      return true;
    });

    const newest = (c: ResearchCall) => new Date(c.publishedAt).getTime();
    const sorted = [...rows];
    switch (sort) {
      case "views":
        // Ties fall back to newest, so a page of 0-view reports still reads in
        // a stable, sensible order instead of whatever the API returned.
        sorted.sort((a, b) => b.viewCount - a.viewCount || newest(b) - newest(a));
        break;
      case "likes":
        sorted.sort((a, b) => b.likeCount - a.likeCount || newest(b) - newest(a));
        break;
      default:
        sorted.sort((a, b) => newest(b) - newest(a));
    }
    return sorted;
  }, [calls, directionFilter, contrarianOnly, sort]);

  return {
    scope,
    setScope,
    calls: filtered,
    /**
     * 필터 전 원본. 우측 레일의 집계(많이 다룬 종목·많이 본 리포트)가 쓴다 — 화면에
     * 이미 있는 목록으로 세므로 별도 요청이 없고, 필터를 바꿔도 집계가 흔들리지 않는다.
     */
    allCalls: calls,
    totalCount: calls.length,
    summary,
    loading,
    error,
    directionFilter,
    setDirectionFilter,
    contrarianOnly,
    setContrarianOnly,
    sort,
    setSort,
    reload: load,
    retract,
  };
}
