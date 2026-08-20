"use client";

import { useEffect, useMemo, useState } from "react";
import { fetchScoresBatchClient, type StockScoreData } from "@/lib/stock-score-api";

const EMPTY = new Map<string, StockScoreData>();

/**
 * 여러 종목 점수를 **한 요청으로**.
 *
 * 카드를 여러 장 늘어놓는 화면(기사 모달의 관련 종목 등)에서 카드마다 단건 API를
 * 부르면 종목 수만큼 요청이 나간다. 보유·관심 화면이 이미 배치 엔드포인트를 쓰고
 * 있었는데 각자 손으로 fetch를 짜고 있어, 같은 코드를 여기로 모았다.
 *
 * 결과에 요청 키를 함께 담아 두는 이유는 저장소의 공통 패턴과 같다 — 심볼 목록이
 * 바뀐 직후 한 프레임 동안 **이전 목록의 점수**가 새 카드에 붙는 것을 막는다.
 */
export function useStockScores(symbols: string[]): Map<string, StockScoreData> {
  const key = useMemo(
    () =>
      [...new Set(symbols.map((s) => s.trim().toUpperCase()).filter(Boolean))].sort().join(","),
    [symbols],
  );
  const [loaded, setLoaded] = useState<{ key: string; scores: Map<string, StockScoreData> }>({
    key: "",
    scores: EMPTY,
  });

  useEffect(() => {
    if (!key) return;
    let alive = true;
    fetchScoresBatchClient(key.split(","))
      .then((rows) => {
        if (!alive) return;
        const map = new Map<string, StockScoreData>();
        for (const row of rows) {
          map.set(row.symbol.trim().toUpperCase(), row);
        }
        setLoaded({ key, scores: map });
      })
      .catch(() => {
        if (alive) setLoaded({ key, scores: EMPTY });
      });
    return () => {
      alive = false;
    };
  }, [key]);

  return loaded.key === key ? loaded.scores : EMPTY;
}
