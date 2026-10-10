/**
 * 「지금 어느 시장이 열려 있나」 — **서버에 물어서** 안다.
 *
 * # 왜 서버인가
 *
 * `lib/market-session`이 같은 이유를 이미 적어 두었다: 세션 창은 종목마다 다르고
 * (넥스트레이드 08:00, 나머지 09:00, 시간외 20:00), 미국은 서머타임에 창이 통째로
 * 한 시간 밀린다. 브라우저에서 다시 계산하면 **서버 판정과 어긋나는 두 번째
 * 진실**이 생긴다. 그래서 거래소 달력을 가진 쪽이 답하고(`/market/clocks`),
 * 화면은 읽기만 한다.
 *
 * # 무엇에 쓰나 (2026-09-11 사용자 요청 9)
 *
 * 국내/해외를 한 카드에서 고르는 자리의 **기본값**이다. 장이 닫힌 시장을 먼저
 * 보여 주면 사람이 매번 토글을 한 번 더 눌러야 한다.
 *
 * 카드 여러 장이 같은 값을 쓰므로 `cachedGet`으로 한 요청을 함께 기다린다.
 * TTL 30초는 세션 경계(09:00 · 15:30 · 22:30/23:30)를 늦지 않게 따라가는 값이다.
 */

import { getApiBaseUrl, mergeFetchInit, parseJson } from "@/lib/api-client";
import type { ApiResponse } from "@/lib/api-types";
import { cachedGet } from "@/lib/fetch-cache";
import { fetchWithTimeout } from "@/lib/fetch-timeout";
import { isLiveSession } from "@/lib/market-session";

export type MarketClock = {
  marketSession?: string;
  sessionClosedAt?: string;
  nextSessionAt?: string;
};

export type MarketClocks = {
  domestic: MarketClock;
  us: MarketClock;
};

/** 국내/해외 두 갈래를 부르는 이름이 화면마다 다르다 — 그 값을 여기 싣는다. */
export type MarketPair<T> = { domestic: T; us: T };

const CACHE_KEY = "market:clocks";
const CACHE_TTL_MS = 30_000;

export async function fetchMarketClocks(): Promise<MarketClocks> {
  return cachedGet(
    CACHE_KEY,
    async () => {
      /*
        **상한을 건다** (2026-09-22 전수 점검에서 남은 두 곳 중 하나).

        09-20이 `lib/fetch-timeout.ts`로 앱 전체에 상한을 걸었는데 이 호출과
        `news-briefing-api`가 빠져 있었다. 여기는 빠지면 특히 아픈 자리다 —
        `cachedGet`이 **모두가 함께 기다리는 promise 하나**를 들고 있어서, 이
        요청이 안 끝나면 시장 세션을 묻는 카드가 전부 같이 멈춘다. 09-20이
        `inflightRefresh`에서 고친 것과 같은 모양이다.
      */
      const res = await fetchWithTimeout(
        `${getApiBaseUrl()}/api/v1/market/clocks`,
        mergeFetchInit({ cache: "no-store" }),
      );
      if (!res.ok) throw new Error("시장 세션을 불러오지 못했습니다");
      const body = await parseJson<ApiResponse<MarketClocks>>(res);
      return body.data ?? { domestic: {}, us: {} };
    },
    CACHE_TTL_MS,
  );
}

/**
 * 열려 있는 쪽을 고른다. **둘 다 열렸거나 둘 다 닫혔으면 국내**
 * (2026-09-11 사용자 결정) — 국내 사용자가 쓰는 앱이라 무차별 상태의 기본이 국내다.
 *
 * 세션을 모르면(응답 실패·구버전 서버) 국내로 떨어진다. `isLiveSession`은 값이
 * 없으면 false를 주므로 이 함수만으로 「모름」이 해외로 새지 않는다.
 */
export function openMarketDefault<T>(clocks: MarketClocks | null, pair: MarketPair<T>): T {
  if (!clocks) return pair.domestic;
  const krLive = isLiveSession(clocks.domestic?.marketSession);
  const usLive = isLiveSession(clocks.us?.marketSession);
  if (usLive && !krLive) return pair.us;
  return pair.domestic;
}
