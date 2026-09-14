"use client";

import { useEffect, useState } from "react";
import {
  fetchMarketClocks,
  openMarketDefault,
  type MarketPair,
} from "@/lib/market-clocks";

/**
 * 국내/해외 토글의 **기본값을 열려 있는 시장으로** 맞춘다 (2026-09-11 사용자 요청 9).
 *
 * # 왜 null로 시작하나
 *
 * 세션은 서버가 답한다(`/market/clocks`). 그 답이 오기 전에 국내로 시작해 버리면
 * 해외장이 열린 새벽에 **국내 목록을 한 번 받아 그린 다음 해외로 갈아치우는** 깜박임이
 * 생기고, 버린 요청 한 벌이 매번 나간다. 그래서 시장이 정해질 때까지는 `null`이고,
 * 카드는 그동안 이미 갖고 있는 스켈레톤을 보여 준다.
 *
 * # 사람의 선택이 이긴다
 *
 * `setMarket`이 한 번이라도 불린 뒤에는 세션 응답이 늦게 와도 덮지 않는다 —
 * 아직 로딩 중에 사용자가 해외를 눌렀다면 그 뜻이 세션 추정보다 앞선다.
 */
export function useOpenMarket<T extends string>(
  pair: MarketPair<T>,
): [T | null, (next: T) => void] {
  const [market, setMarket] = useState<T | null>(null);
  const [picked, setPicked] = useState(false);

  useEffect(() => {
    let alive = true;
    fetchMarketClocks()
      .then((clocks) => {
        if (!alive || picked) return;
        setMarket(openMarketDefault(clocks, pair));
      })
      .catch(() => {
        // 세션을 못 받아도 카드는 떠야 한다 — 국내로 떨어진다.
        if (alive && !picked) setMarket(pair.domestic);
      });
    return () => {
      alive = false;
    };
    // pair는 렌더마다 새 객체이므로 값만 의존한다.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [pair.domestic, pair.us, picked]);

  return [
    market,
    (next: T) => {
      setPicked(true);
      setMarket(next);
    },
  ];
}
