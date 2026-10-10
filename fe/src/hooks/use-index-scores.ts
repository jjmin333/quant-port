"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { useVisibleInterval } from "@/hooks/use-visible-interval";
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
 *
 * @param refreshMs 0보다 크면 **탭이 보이는 동안** 이 주기로 다시 받는다. 기본은
 *                  0(한 번만) — 점수는 하루 한 번 도는 배치가 만들지만, 홈처럼 **열어
 *                  둔 채 날짜가 바뀌는** 화면은 주기를 준다(2026-09-16 사용자 요청).
 */
export function useIndexScores(refreshMs = 0) {
  const [byId, setById] = useState<Record<string, IndexScoreSummary>>({});

  /** 마지막으로 건 요청만 쓴다 — 요청을 거는 자리가 첫 적재와 주기 둘이다. */
  const seqRef = useRef(0);
  useEffect(
    () => () => {
      seqRef.current += 1;
    },
    [],
  );

  const load = useCallback((initial: boolean) => {
    seqRef.current += 1;
    const seq = seqRef.current;
    apiRequest<IndexScoreSummary[]>("/api/v1/indices/scores", undefined, "지수 점수 조회 실패")
      .then((rows) => {
        if (seq !== seqRef.current) return;
        const map: Record<string, IndexScoreSummary> = {};
        for (const row of rows ?? []) map[row.indexId] = row;
        setById(map);
      })
      .catch(() => {
        // Scores are supplementary — an index card renders fine without one.
        // 갱신의 실패는 들고 있던 점수를 지우지 않는다(첫 적재만 빈 표로).
        if (seq === seqRef.current && initial) setById({});
      });
  }, []);

  useEffect(() => {
    load(true);
  }, [load]);

  useVisibleInterval(() => load(false), refreshMs, refreshMs > 0);

  return byId;
}
