"use client";

import { useEffect, useState } from "react";
import { apiRequest } from "@/lib/api-client";

/** Mirrors service.IndexScoreSummary. */
export type IndexScoreSummary = {
  indexId: string;
  tradeDate: string;
  score: number | null;
  grade?: string;
  aiSummary?: string;
};

/**
 * All index scores in one request, keyed by index id.
 *
 * Batched on purpose: the overview renders a dozen cards at once, and a request
 * per card would make the page feel slow for a dozen tiny rows.
 */
export function useIndexScores() {
  const [byId, setById] = useState<Record<string, IndexScoreSummary>>({});

  useEffect(() => {
    let cancelled = false;
    apiRequest<IndexScoreSummary[]>("/api/v1/indices/scores", undefined, "지수 점수 조회 실패")
      .then((rows) => {
        if (cancelled) return;
        const map: Record<string, IndexScoreSummary> = {};
        for (const row of rows ?? []) map[row.indexId] = row;
        setById(map);
      })
      .catch(() => {
        // Scores are supplementary — an index card renders fine without one.
        if (!cancelled) setById({});
      });
    return () => {
      cancelled = true;
    };
  }, []);

  return byId;
}
