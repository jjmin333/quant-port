"use client";

import { useEffect, useState } from "react";

import {
  fetchHoldingsDiagnosisClient,
  type HoldingsDiagnosisData,
} from "@/lib/holdings-diagnosis-api";

/**
 * 보유 이유 점검 한 번 (V2 요구 6).
 *
 * # 매매가 바뀌면 다시 묻는다
 *
 * `reloadKey`는 보유 화면의 `tradesVersion`이다. 종목을 더하거나 지우면 판정의 전제인
 * **보유 기간**이 바뀌므로 — 실적·내부자를 자르는 기준이 그 날짜다 — 같은 답을 계속
 * 들고 있으면 안 된다.
 *
 * # 「불러오는 중」은 파생값이다
 *
 * `useScoreBacktest`와 같은 모양이다: 결과에 요청 키를 함께 들어 두면 이펙트 안에서
 * setState를 동기로 부르지 않아도 되고(저장소 린트 규칙), 키가 막 바뀐 한 프레임 동안
 * **이전 요청의 답**이 새 화면에 앉지 않는다.
 *
 * # `null`과 오류를 가른다
 *
 * 「보유 종목이 없다」(`data === null`)와 「불러오지 못했다」(`error`)는 화면이 다르게
 * 말해야 하는 두 상태다 (요구 8).
 */
export function useHoldingsDiagnosis(reloadKey: number, enabled = true) {
  const key = enabled ? String(reloadKey) : "";

  const [loaded, setLoaded] = useState<{
    key: string;
    data: HoldingsDiagnosisData | null;
    error: string | null;
  }>({ key: "", data: null, error: null });

  useEffect(() => {
    if (!enabled) return;
    let cancelled = false;
    fetchHoldingsDiagnosisClient()
      .then((res) => {
        if (!cancelled) setLoaded({ key, data: res, error: null });
      })
      .catch((err: unknown) => {
        if (cancelled) return;
        setLoaded({
          key,
          data: null,
          error: err instanceof Error ? err.message : "보유 진단을 불러오지 못했습니다.",
        });
      });
    return () => {
      cancelled = true;
    };
  }, [key, enabled]);

  const fresh = loaded.key === key;
  return {
    data: fresh ? loaded.data : null,
    loading: enabled && !fresh,
    error: fresh ? loaded.error : null,
  };
}
