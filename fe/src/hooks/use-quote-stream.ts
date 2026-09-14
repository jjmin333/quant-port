"use client";

import { useCallback, useMemo, useSyncExternalStore } from "react";
import { EMPTY_STATE, getHubState, subscribeListPrices } from "@/lib/list-price-hub";
import { ensureBrowserStreamPort } from "@/lib/list-price-hub-browser";
import { streamSymbolsKey, symbolsFromKey, type ListPriceMap } from "@/lib/quote-stream-state";

/*
목록 화면의 가격을 **연결 하나로** 받는다 — 마켓 랭킹·AI 랭킹·보유·관심이 함께 쓴다.

# 왜 폴링 대신인가 (2026-09-04 사용자 요청 1)

폴링은 뷰어 한 명이 늘 때마다 서버 작업이 한 벌 더 생긴다. 값을 만드는 일은 이미
서버 폴러가 2초마다 한 번 하고 있는데, 그걸 꺼내 오는 왕복만 뷰어 수에 비례해
늘어난다. 실측한 요청당 비용(2026-09-04):

	/rankings/prices      100종목    4 SELECT
	/quotes/batch          20종목   20 SELECT   (종목당 1)
	/stocks/scores/top     20종목  100 SELECT   (종목당 5, 기준가 해석)

마지막 줄이 AI 랭킹이다. 그 화면을 2초 폴링으로 만들면 뷰어당 초당 50 SELECT이고,
운영 DB는 1 ECPU다. 스트림은 그 곱셈을 없앤다 — 서버가 만들어 둔 값을 복사할 뿐이라
뷰어가 몇이든 DB 작업이 0이다.

# 폴링을 없애지는 않는다

호출부는 폴링을 **그대로 두고** 이 훅을 얹는다. 반환값의 `streaming`이 true인 동안
폴링 주기를 `PRICE_POLL_MS_WHILE_STREAMING`(30초)으로 늦추고, 스트림이 끊기면
저절로 2초로 돌아온다.

0으로 끄지 않는 이유: SSE가 조용한 것과 「가격이 안 변한 것」은 화면에서 구분되지
않는다. 프록시가 연결을 조용히 끊어도 EventSource는 한동안 열려 있는 것처럼 보일 수
있고, 그때 폴링까지 0이면 화면이 **낡은 값을 든 채 멀쩡해 보인다.**

# 마감이면 붙지 않는다 (2026-09-04 사용자 추가 요청)

서버가 열자마자 `session` 이벤트 하나를 주고 끊는다. 그 안의 `nextSessionAt`으로
타이머를 걸어 개장하는 순간 다시 붙는다. 깨어나는 길은 셋이다 — 그 타이머,
탭이 다시 보이는 순간, 그리고 호출부의 목록 갱신(심볼이 바뀌면 새로 연결한다).

**시각을 못 받으면 5분마다 되묻는다.** 「모른다」를 「닫혔다」로 굳히면 화면이
개장을 영영 못 알아챈다 — `use-quote-detail`이 같은 이유로 같은 값을 쓴다.

# 탭이 뒤로 가도 **끊지 않는다** (2026-09-05 사용자 요청 5)

예전에는 `visibilitychange`에서 곧바로 닫았다. 규칙은 이제 `useStreamAwake`가
들고 있다 — 숨어도 3시간은 붙어 있고, 그 뒤에 놓는다. 이유는 그쪽 머리글에 있다.

**폴링은 그대로 숨으면 멈춘다.** 스트림은 서버가 이미 만들어 둔 값을 밀어 주는
것이라 뷰어가 늘어도 DB 작업이 0이지만, 폴링은 왕복이 뷰어 수에 정비례한다.
*/

export type UseQuoteStreamResult = {
  /** 심볼(대문자) → 가장 최근 가격. 아직 못 받은 종목은 없다. */
  prices: ListPriceMap;
  /**
   * 지금 스트림으로 값이 흐르고 있는가.
   *
   * 호출부는 이 값으로 **폴링 주기만** 고른다. 화면에 「실시간」이라고 적는 근거로
   * 쓰면 안 된다 — 그건 세션이 정하는 것이고(`lib/market-session`), 스트림이 붙어
   * 있다는 사실과는 다른 질문이다.
   */
  streaming: boolean;
};

/**
 * 이 심볼들의 가격을 스트림으로 받는다.
 *
 * **연결은 이 훅이 만들지 않는다** (2026-09-05 사용자 지적 6). `lib/list-price-hub`가
 * 시장마다 하나씩 들고 있고, 여기서는 심볼을 등록하고 그 시장의 가격 맵을 읽을 뿐이다.
 * 같은 화면에 이 훅이 몇 개 있든 연결은 시장 수만큼이다.
 *
 * 돌려주는 맵에는 **다른 카드가 구독한 종목도 함께** 들어 있다. 호출부는 자기 행의
 * 심볼만 꺼내 쓰므로 화면에는 영향이 없고, 맵을 나눠 들면 같은 종목이 카드마다 다른
 * 값이 될 수 있다 — 09-04에 고친 「한 값은 한 값이다」가 다시 갈라지는 자리다.
 */
export function useQuoteStream(
  symbols: readonly string[],
  assetType: string,
  enabled = true,
): UseQuoteStreamResult {
  /*
    심볼 목록은 렌더마다 새 배열이다. 키로 바꿔 의존성에 걸지 않으면 **렌더마다**
    구독을 끊고 다시 건다 — 허브는 그것을 합집합이 바뀐 것으로 보고 연결을 다시 연다.
  */
  const key = useMemo(() => streamSymbolsKey(symbols), [symbols]);
  const active = enabled && key !== "";

  const subscribe = useCallback(
    (notify: () => void) => {
      if (!active) return () => {};
      // 브라우저 이음매는 처음 한 번만 꽂힌다 — 두 번째부터는 아무 일도 없다.
      ensureBrowserStreamPort();
      return subscribeListPrices(assetType, symbolsFromKey(key), notify);
    },
    [active, assetType, key],
  );

  /*
    `useSyncExternalStore`를 쓰는 이유는 `use-market-indices`와 같다 — 값이 React 밖
    (모듈)에 살기 때문이다. `useState` + `useEffect`로 흉내 내면 구독을 걸기 전에
    도착해 있던 스냅샷을 이펙트에서 맞춰 줘야 하고, 그게 `set-state-in-effect`다.
  */
  const state = useSyncExternalStore(
    subscribe,
    () => (active ? getHubState(assetType) : EMPTY_STATE),
    () => EMPTY_STATE,
  );

  /*
    「스트림 중」은 **연결이 살아 있고 + 구독할 것이 있을 때**다.

    `connected`만으로는 부족하다. 목록이 비어 심볼이 하나도 없으면 구독 자체를 안
    하는데, 그 시장 허브는 다른 카드 때문에 붙어 있을 수 있다. 그 상태로 호출부가
    폴링을 30초로 늦추면 목록이 다시 채워진 순간 그 행들이 30초 동안 값 없이 선다.
  */
  return { prices: state.prices, streaming: state.connected && active };
}

/*
보유·관심은 **한 목록에 두 시장이 섞인다.**

그래서 `useQuoteStream`을 그대로 쓸 수 없다. 자산 구분은 서버가 마감을 판정하는
달력이라, 섞인 목록에 하나를 골라 주면 반드시 한쪽이 틀린다 — 국내로 주면 KRX가
닫히는 15:30에 미국 종목까지 스트림이 끊기고, 미국으로 주면 국내 종목이 새벽까지
빈 파이프를 문다.

시장별로 나눠 **연결 둘**을 연다. 한 시장만 있으면 하나만 열린다(빈 목록은
`useQuoteStream`이 연결하지 않는다). 서버 쪽 비용은 연결 수가 아니라 구독 종목
수에 붙으므로, 나누는 것 자체는 공짜다.
*/
export type StreamableRow = { symbol: string; assetType?: string };

/**
 * 국내 달력을 쓰는 자산 구분. 나머지(us·etf)는 미국 달력이다.
 *
 * **`assetType`이 없으면 심볼 모양으로 판단한다.** 예전에는 빈 값을 무조건 국내로
 * 봤는데, 그러면 자산 구분을 안 실어 주는 경로(뉴스 기사의 `symbols` 등)에서 미국
 * 티커가 국내 목록에 섞인다 — 그 목록은 KRX가 닫히는 15:30에 스트림이 끊기므로
 * 미국 정규장 내내 값이 멈춘다. 국내 종목 코드는 여섯 자리 숫자라 모양만으로
 * 갈린다(2026-09-04 사용자 요청 9·12).
 */
export function isDomesticAsset(assetType: string | undefined, symbol?: string): boolean {
  const at = (assetType ?? "").toLowerCase();
  if (at === "domestic" || at === "bond" || at === "cash") return true;
  if (at === "us" || at === "etf") return false;
  return /^\d{6}$/.test((symbol ?? "").trim());
}

export function useQuoteStreamByMarket(rows: readonly StreamableRow[]): UseQuoteStreamResult {
  const domestic = useMemo(
    () => rows.filter((r) => isDomesticAsset(r.assetType, r.symbol)).map((r) => r.symbol),
    [rows],
  );
  const overseas = useMemo(
    () => rows.filter((r) => !isDomesticAsset(r.assetType, r.symbol)).map((r) => r.symbol),
    [rows],
  );

  const kr = useQuoteStream(domestic, "domestic", domestic.length > 0);
  const us = useQuoteStream(overseas, "us", overseas.length > 0);

  const prices = useMemo(() => {
    // 한쪽이 비어 있으면 **다른 쪽 맵을 그대로** 돌려준다 — 합치느라 새 맵을 만들면
    // 참조가 매번 바뀌어 구독한 이펙트가 헛돈다(대부분의 목록이 이 경우다).
    if (kr.prices.size === 0) return us.prices;
    if (us.prices.size === 0) return kr.prices;
    return new Map([...kr.prices, ...us.prices]);
  }, [kr.prices, us.prices]);

  /*
    **둘 다 붙어 있어야 「스트림 중」이다.** 한쪽만 붙은 상태에서 폴링을 늦추면
    다른 쪽 시장의 행들이 30초짜리 값이 된다 — 그 목록에서 가장 눈에 띄는 고장이다.
  */
  const streaming =
    (domestic.length === 0 || kr.streaming) &&
    (overseas.length === 0 || us.streaming) &&
    domestic.length + overseas.length > 0;

  return { prices, streaming };
}
