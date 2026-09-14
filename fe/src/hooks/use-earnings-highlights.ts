"use client";

import { useEffect, useState } from "react";
import { fetchEarningsHighlights } from "@/lib/earnings-api";
import type { EarningsHighlights, EarningsMarket } from "@/lib/earnings-types";

/*
실적 하이라이트(`GET /earnings/highlights`)를 **화면당 한 번** 받는다.

레일 안에 있던 `useHighlights`를 끌어올린 것이다 (2026-09-05 사용자 요청 7). 옮긴
이유는 홈의 「주요 실적 일정」과 실적 레일의 「다가오는 대형 발표」가 **같은 응답의
같은 칸**(`big`)을 쓰면서 각자 부르고 있었기 때문이다 — 카드를 한 부품으로 모으면서
가져오는 자리도 하나가 됐다.

`fetchEarningsHighlights`가 이미 `cachedGet`을 지나므로 같은 시장을 동시에 두 번 물어도
왕복은 하나다. 그래도 훅으로 모으는 이유는 **상태 처리가 틀리기 쉬운 자리**여서다 —
아래 「키드 상태」 참조.
*/

const EMPTY: EarningsHighlights = { big: [], divergent: [], consistent: [] };

/**
 * @returns 아직 받는 중이면 `null`. 실패는 빈 하이라이트로 떨어진다 — 전부 부가
 *          정보라 본문은 그대로 떠야 한다.
 */
export function useEarningsHighlights(market: EarningsMarket): EarningsHighlights | null {
  /*
    **어느 시장의 결과인지 함께 들고 있는다.** 시장이 바뀔 때 이펙트에서 동기로 null을
    넣으면 렌더가 한 번 더 돌고(`react-hooks/set-state-in-effect`), 안 넣으면 새 응답이
    오기 전까지 앞 시장의 목록이 그대로 서 있다. 키가 안 맞으면 「받는 중」으로 읽는다.
  */
  const [loaded, setLoaded] = useState<{ key: string; data: EarningsHighlights } | null>(null);

  useEffect(() => {
    let alive = true;
    fetchEarningsHighlights(market)
      .then((data) => {
        if (alive) setLoaded({ key: market, data });
      })
      .catch(() => {
        if (alive) setLoaded({ key: market, data: EMPTY });
      });
    return () => {
      alive = false;
    };
  }, [market]);

  return loaded?.key === market ? loaded.data : null;
}
