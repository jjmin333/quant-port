"use client";

import { useEffect, useMemo, useState } from "react";

import {
  fetchScoreBacktestClient,
  type BacktestQuery,
  type BacktestResult,
} from "@/lib/score-backtest-api";

/**
 * AI 랭킹 백테스트 한 판.
 *
 * # 결과와 요청 키를 함께 든다
 *
 * `useStockScores`·`useScoreChanges`와 같은 모양이고, 같은 두 가지를 얻는다:
 *
 *   · 손잡이가 막 바뀐 한 프레임 동안 **이전 조합의 결과**가 새 제목 밑에 앉지 않는다.
 *   · 「불러오는 중」이 **파생값**이라 이펙트 안에서 setState를 동기로 부르지 않는다
 *     (그렇게 하면 렌더가 연쇄되고, 저장소의 린트 규칙이 그걸 막는다).
 *
 * # `null`과 오류를 가른다
 *
 * 「이력이 아직 없다」(`data === null`, 오류 아님)와 「불러오지 못했다」(`error`)는
 * 화면이 다르게 말해야 하는 두 상태다 (요구 8).
 */
export function useScoreBacktest(query: BacktestQuery, enabled = true) {
  const { market, axis, topN, rebalance, benchmarks } = query;
  const benchKey = (benchmarks ?? []).join("|");
  const key = useMemo(
    () => [market, axis, topN ?? "", rebalance ?? "", benchKey].join(":"),
    [market, axis, topN, rebalance, benchKey],
  );

  const [loaded, setLoaded] = useState<{
    key: string;
    data: BacktestResult | null;
    error: string | null;
  }>({ key: "", data: null, error: null });

  useEffect(() => {
    if (!enabled || !key) return;
    let cancelled = false;
    fetchScoreBacktestClient({ market, axis, topN, rebalance, benchmarks })
      .then((res) => {
        if (!cancelled) setLoaded({ key, data: res, error: null });
      })
      .catch((err: unknown) => {
        if (cancelled) return;
        setLoaded({
          key,
          data: null,
          error: err instanceof Error ? err.message : "백테스트를 불러오지 못했습니다.",
        });
      });
    return () => {
      cancelled = true;
    };
    // market·axis·topN·rebalance·benchmarks는 전부 `key`에 녹아 있다 — 배열 참조를
    // 의존성에 넣으면 부모가 리렌더될 때마다 새 참조라 매번 다시 부른다.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [key, enabled]);

  const fresh = loaded.key === key;
  return {
    data: fresh ? loaded.data : null,
    loading: enabled && !fresh,
    error: fresh ? loaded.error : null,
  };
}
