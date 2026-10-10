"use client";

import { useEffect, useState } from "react";

import { fetchPriceScenarioClient, type PriceScenario } from "@/lib/price-scenario-api";

/**
 * 한 종목의 시나리오.
 *
 * 탭이 열릴 때만 부릅니다(`enabled`) — 밸류에이션 탭은 아홉 탭 중 하나라, 종목을
 * 열 때마다 받으면 대부분 안 볼 화면을 위해 요청이 나갑니다.
 *
 * 「불러오는 중」은 **파생값**입니다: 결과에 요청 키를 함께 들어 두면 이펙트 안에서
 * setState를 동기로 부르지 않아도 되고(저장소 린트 규칙), 종목이 막 바뀐 한 프레임
 * 동안 이전 종목의 밴드가 새 제목 밑에 앉지 않습니다.
 */
export function usePriceScenario(symbol: string, enabled = true) {
  const key = enabled && symbol ? symbol.trim().toUpperCase() : "";

  const [loaded, setLoaded] = useState<{
    key: string;
    data: PriceScenario | null;
    error: string | null;
  }>({ key: "", data: null, error: null });

  useEffect(() => {
    if (!key) return;
    let cancelled = false;
    fetchPriceScenarioClient(key)
      .then((data) => {
        if (!cancelled) setLoaded({ key, data, error: null });
      })
      .catch((err: unknown) => {
        if (cancelled) return;
        setLoaded({
          key,
          data: null,
          error: err instanceof Error ? err.message : "시나리오를 불러오지 못했습니다.",
        });
      });
    return () => {
      cancelled = true;
    };
  }, [key]);

  const fresh = loaded.key === key;
  return {
    scenario: fresh ? loaded.data : null,
    loading: Boolean(key) && !fresh,
    error: fresh ? loaded.error : null,
  };
}
