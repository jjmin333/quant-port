"use client";

import { useEffect, useState } from "react";

import { fetchSymbolContextClient, type SymbolContext } from "@/lib/symbol-context-api";

/**
 * 「이 종목에 대한 내 기록」.
 *
 * 비로그인이면 **부르지 않습니다** — 응답이 전부 「내」 기록이라 401이 정상이고,
 * 화면은 줄 자체를 안 그립니다. 401을 받아 놓고 빈 줄을 그리는 것보다 안 묻는 쪽이
 * 요청 하나를 아낍니다.
 *
 * `reloadKey`는 보유·관심·스탬프가 바뀌었을 때 올립니다. 그 동작들은 전부 이 화면
 * 안에서 일어나므로, 폴링으로 맞추면 사용자가 자기 동작의 결과를 기다리게 됩니다.
 *
 * 「불러오는 중」은 파생값입니다(저장소 패턴) — 이펙트 안에서 setState를 동기로
 * 부르지 않고, 종목이 막 바뀐 한 프레임 동안 이전 종목의 기록이 앉지 않습니다.
 */
export function useSymbolContext(symbol: string, enabled: boolean, reloadKey = 0) {
  const key = enabled && symbol ? `${symbol.trim().toUpperCase()}:${reloadKey}` : "";

  const [loaded, setLoaded] = useState<{ key: string; data: SymbolContext | null }>({
    key: "",
    data: null,
  });

  useEffect(() => {
    if (!key) return;
    let cancelled = false;
    fetchSymbolContextClient(key.split(":")[0])
      .then((data) => {
        if (!cancelled) setLoaded({ key, data });
      })
      .catch(() => {
        /*
          실패해도 **줄만 안 그립니다.** 이 줄은 보조 정보라 오류 문구를 띄우면
          종목 화면 맨 위에 빨간 줄이 서는데, 정작 사용자가 보러 온 시세·차트는
          멀쩡합니다.
        */
        if (!cancelled) setLoaded({ key, data: null });
      });
    return () => {
      cancelled = true;
    };
  }, [key]);

  return loaded.key === key ? loaded.data : null;
}
