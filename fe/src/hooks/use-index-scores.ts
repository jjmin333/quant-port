"use client";

import { useEffect, useState } from "react";
import { apiRequest } from "@/lib/api-client";

/** Mirrors service.IndexScoreSummary. */
export type IndexScoreSummary = {
  indexId: string;
  tradeDate: string;
  score: number | null;
  grade?: string;
  /**
   * 규칙이 만든 **지표 나열** 한 줄. 이름의 「AI」는 남아 있는 옛 이름이고, 이 문장에는
   * 모델이 관여하지 않는다. 열세 지수 전부에 언제나 있다.
   */
  aiSummary?: string;
  /**
   * 그 지표들이 **함께 가리키는 국면**을 적은 한 문단. 배치가 실제로 돌았을 때만 온다 —
   * 모델이 꺼져 있거나 실패한 지수는 비어 있고, 그때 화면은 위 규칙 문장만 보여 준다.
   */
  llmNote?: string;
  /** 「상승 흐름」·「방향 없음」·「하락 흐름」 중 하나. */
  llmStance?: string;
  /**
   * 상태 키워드 **코드**다(`high_52w`, `streak_up:4` …). 문구는 화면이 붙인다 —
   * `lib/index-tags.ts`.
   */
  tags?: string[];
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
