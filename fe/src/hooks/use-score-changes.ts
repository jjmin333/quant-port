"use client";

import { useEffect, useMemo, useState } from "react";
import { fetchScoreChangesClient, type ScoreChangeSummary } from "@/lib/score-explain-api";

const EMPTY = new Map<string, ScoreChangeSummary>();

/**
 * 여러 종목의 점수 변화를 **한 요청으로**.
 *
 * `useStockScores`와 같은 모양이고 같은 이유다 — 행마다 부르면 요청이 종목 수만큼
 * 곱해진다. 다른 점은 이쪽이 훨씬 가볍다는 것뿐이다(10종목 2.6KB 실측).
 *
 * # 점수 카드 한 장에도 이 훅을 쓴다
 *
 * 종목 하나짜리 배열을 넘기면 된다. 설명 전체(`/score/explain`, 15KB)를 받아 한 줄만
 * 쓰는 것보다 싸고, 개요 탭은 그 나머지를 쓰지 않는다 — 쓰는 곳은 AI 분석 탭이고
 * 그 탭은 열릴 때 자기 것을 받는다.
 *
 * 요청 키를 결과와 함께 들고 있는 이유는 저장소의 공통 패턴과 같다: 종목 목록이 바뀐
 * 직후 한 프레임 동안 **이전 목록의 변화**가 새 행에 붙는 것을 막는다.
 */
export function useScoreChanges(symbols: string[]): Map<string, ScoreChangeSummary> {
  const key = useMemo(
    () => [...new Set(symbols.map((s) => s.trim().toUpperCase()).filter(Boolean))].sort().join(","),
    [symbols],
  );
  const [loaded, setLoaded] = useState<{ key: string; rows: Map<string, ScoreChangeSummary> }>({
    key: "",
    rows: EMPTY,
  });

  useEffect(() => {
    if (!key) return;
    let alive = true;
    fetchScoreChangesClient(key.split(","))
      .then((rows) => {
        if (!alive) return;
        const map = new Map<string, ScoreChangeSummary>();
        for (const row of rows) {
          map.set(row.symbol.trim().toUpperCase(), row);
        }
        setLoaded({ key, rows: map });
      })
      .catch(() => {
        // 점수가 없는 종목은 정상 상태다 — 행은 그냥 사유 줄을 그리지 않는다.
        if (alive) setLoaded({ key, rows: EMPTY });
      });
    return () => {
      alive = false;
    };
  }, [key]);

  return loaded.key === key ? loaded.rows : EMPTY;
}
