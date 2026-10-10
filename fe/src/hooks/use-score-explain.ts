"use client";

import { useEffect, useState } from "react";
import { fetchScoreExplainClient, type ScoreExplainData } from "@/lib/score-explain-api";

/**
 * 한 종목의 점수 근거 전체 (15KB).
 *
 * **펼쳤을 때만 부른다.** AI 분석 탭이 지연 로딩이라 이 훅은 그 탭이 실제로 열릴 때
 * 처음 돈다 — 종목을 열기만 한 사람은 이 요청을 내지 않는다.
 *
 * 점수가 없는 종목은 `null`이고 그것은 오류가 아니다. 오류(네트워크·서버)와는 갈라
 * 두어야 화면이 「데이터 없음」과 「불러오지 못했습니다」를 다르게 말할 수 있다.
 */
export function useScoreExplain(symbol: string) {
  const [data, setData] = useState<ScoreExplainData | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!symbol) {
      setData(null);
      setLoading(false);
      return;
    }
    let cancelled = false;
    setLoading(true);
    setError(null);
    fetchScoreExplainClient(symbol)
      .then((res) => {
        if (!cancelled) setData(res);
      })
      .catch((err: unknown) => {
        if (cancelled) return;
        setData(null);
        setError(err instanceof Error ? err.message : "점수 근거를 불러오지 못했습니다.");
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [symbol]);

  return { data, loading, error };
}
