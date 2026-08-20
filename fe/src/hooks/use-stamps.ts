"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { isSampleModeActive } from "@/components/common/sample-mode";
import { fetchSampleStampSummary, fetchSampleStamps } from "@/lib/sample-api";
import {
  deleteStampClient,
  fetchStampSummaryClient,
  fetchStampsClient,
  openStampClient,
  updateStampClient,
} from "@/lib/stamp-api";
import type {
  Stamp,
  StampStance,
  StampSummary,
  StampUpdateInput,
  StampVerdict,
} from "@/lib/stamp-types";

export type StampGrouping = "timeline" | "symbol";
export type StampVerdictFilter = "all" | "due" | StampVerdict;

/**
 * Loads every stamp once and filters client-side. Personal stamp counts are in
 * the hundreds at most, so a server round trip per filter change would only add
 * latency (same reasoning as the 메모장 list).
 */
export function useStamps({ sample = false }: { sample?: boolean } = {}) {
  const [stamps, setStamps] = useState<Stamp[]>([]);
  // The hit-rate rule lives on the server so one definition serves every view.
  const [summary, setSummary] = useState<StampSummary | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const [symbolFilter, setSymbolFilter] = useState<string | null>(null);
  const [stanceFilter, setStanceFilter] = useState<StampStance | "all">("all");
  const [verdictFilter, setVerdictFilter] = useState<StampVerdictFilter>("all");
  const [grouping, setGrouping] = useState<StampGrouping>("timeline");

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    const isSample = isSampleModeActive(sample);
    try {
      // 예시 모드는 공개 픽스처를 읽는다 — 로그인 여부와 무관하게 같은 화면이 뜬다.
      const [list, stats] = isSample
        ? await Promise.all([fetchSampleStamps(), fetchSampleStampSummary().catch(() => null)])
        : await Promise.all([
            fetchStampsClient({ limit: 500 }),
            fetchStampSummaryClient().catch(() => null),
          ]);
      setStamps(list);
      setSummary(stats);
    } catch {
      setError("스탬프를 불러오지 못했습니다.");
    } finally {
      setLoading(false);
    }
  }, [sample]);

  useEffect(() => {
    load();
  }, [load]);

  const symbols = useMemo(() => {
    const map = new Map<string, { name: string; count: number }>();
    for (const s of stamps) {
      const prev = map.get(s.symbol);
      map.set(s.symbol, {
        name: s.symbolName || s.symbol,
        count: (prev?.count ?? 0) + 1,
      });
    }
    return Array.from(map.entries()).map(([symbol, v]) => ({ symbol, ...v }));
  }, [stamps]);

  const visible = useMemo(() => {
    return stamps.filter((s) => {
      if (symbolFilter && s.symbol !== symbolFilter) return false;
      if (stanceFilter !== "all" && s.stance !== stanceFilter) return false;
      if (verdictFilter === "due") return s.due && !s.opened;
      if (verdictFilter !== "all" && s.verdict !== verdictFilter) return false;
      return true;
    });
  }, [stamps, symbolFilter, stanceFilter, verdictFilter]);

  const grouped = useMemo(() => {
    if (grouping === "symbol") {
      const map = new Map<string, Stamp[]>();
      for (const s of visible) {
        const list = map.get(s.symbol) ?? [];
        list.push(s);
        map.set(s.symbol, list);
      }
      return Array.from(map.entries()).map(([symbol, items]) => ({
        key: symbol,
        label: items[0]?.symbolName || symbol,
        items,
      }));
    }
    const map = new Map<string, Stamp[]>();
    for (const s of visible) {
      const day = s.tradeDate;
      const list = map.get(day) ?? [];
      list.push(s);
      map.set(day, list);
    }
    return Array.from(map.entries())
      .sort((a, b) => (a[0] < b[0] ? 1 : -1))
      .map(([day, items]) => ({ key: day, label: day, items }));
  }, [visible, grouping]);

  const dueStamps = useMemo(() => stamps.filter((s) => s.due && !s.opened), [stamps]);

  const replace = useCallback((next: Stamp) => {
    setStamps((prev) => prev.map((s) => (s.id === next.id ? next : s)));
  }, []);

  const refreshSummary = useCallback(async () => {
    try {
      setSummary(await fetchStampSummaryClient());
    } catch {
      // Keep the previous numbers rather than blanking the header.
    }
  }, []);

  // 예시 행은 서버에 존재하지 않는다. 저장 버튼을 눌러도 조용히 아무 일도 없게 두면
  // "저장이 안 되네"가 되므로, 화면 쪽에서 편집 UI 자체를 숨기고 여기서도 막는다.
  const updateStamp = useCallback(
    async (id: number, input: StampUpdateInput) => {
      if (sample) return;
      replace(await updateStampClient(id, input));
      await refreshSummary();
    },
    [sample, replace, refreshSummary],
  );

  const openStamp = useCallback(
    async (id: number) => {
      if (sample) return;
      replace(await openStampClient(id));
      await refreshSummary();
    },
    [sample, replace, refreshSummary],
  );

  const removeStamp = useCallback(
    async (id: number) => {
      if (sample) return;
      await deleteStampClient(id);
      setStamps((prev) => prev.filter((s) => s.id !== id));
      await refreshSummary();
    },
    [sample, refreshSummary],
  );

  return {
    stamps: visible,
    summary,
    grouped,
    allStamps: stamps,
    dueStamps,
    totalCount: stamps.length,
    symbols,
    loading,
    error,
    symbolFilter,
    setSymbolFilter,
    stanceFilter,
    setStanceFilter,
    verdictFilter,
    setVerdictFilter,
    grouping,
    setGrouping,
    reload: load,
    updateStamp,
    openStamp,
    removeStamp,
  };
}
