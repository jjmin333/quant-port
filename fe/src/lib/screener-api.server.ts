import { unstable_cache } from "next/cache";

import { fetchWithDeadline } from "@/lib/fetch-timeout";
import {
  defaultScreenQuery,
  SCREENER_DEFAULT_MARKET,
  type ScreenCatalog,
  type ScreenResult,
} from "@/lib/screener-api";

/**
 * 스크리너의 **첫 화면을 서버에서 미리 받아 온다** (2026-09-22 QA 기획서 §16).
 *
 * # 무엇이 잘못돼 있었나
 *
 * 실측(2026-09-22, 공개 사이트): `/screener`의 서버 HTML에 「**스크리너를 불러오는
 * 중…** / 조건 목록을 불러오는 중입니다…」만 있고 종목명이 한 개도 없었다.
 * 홈·`/analysis`(09-20)·`/market`(이번)과 같은 모양이다.
 *
 * # 조건 목록만으로는 모자란다
 *
 * 조건 목록(`/screen/fields`)만 심으면 HTML에 들어가는 것은 **지표 이름**뿐이다
 * (「종합 점수」·「추세」…). 이 화면의 내용은 결과 표에 있고, 거기 종목명이 있다.
 * 그래서 **기본 조건의 조회 결과까지** 심는다 — 실측 19.3KB · 183종목
 * (리튬포어스 88 A+ 등).
 *
 * # 조회가 POST라서 `unstable_cache`가 필요하다
 *
 * Next의 데이터 캐시(`next: { revalidate }`)는 **GET만** 보장한다(Next 16.2.10
 * `fetch` 레퍼런스: 메모이제이션·캐시 설명이 전부 `GET` 기준). 조회를 POST로 둔
 * 이유는 `screener-api.ts` 쪽 주석과 같고(조건 12개를 URL에 담으면 프록시마다 다른
 * 상한에 걸린다), 그 결정은 그대로 둔다. 대신 **기본 조건 하나는 고정값**이므로
 * 함수 단위 캐시로 감싼다 — 비로그인 방문자·크롤러가 전부 같은 질의를 하므로
 * 적중률이 사실상 1이다.
 *
 * 사용자가 고친 조건은 여기를 타지 않는다. 서버가 심는 것은 **기본 조건 하나뿐**이고,
 * 나머지는 전부 브라우저에서 POST로 나간다.
 *
 * # 실패는 화면을 막지 않는다
 *
 * 둘 다 `catch`로 `null`을 돌려준다. 상한 3초(`SSR_TIMEOUT_MS`)는 TTFB에 그대로
 * 더해지는 시간이라, 백엔드가 흔들릴 때 페이지를 붙잡지 않는 쪽을 고른다.
 */

/** 서버 안에서는 nginx를 거치지 않고 BE로 직접 간다. */
function apiBase(): string {
  return process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080";
}

/**
 * 조건 목록 + 그날의 커버리지.
 *
 * 재검증 900초 — 커버리지는 하루 한 번 계산되는 점수에서 나오고 **BE도 15분
 * 캐시한다**(`use-screener.ts` 주석). 우리가 더 자주 물어봐야 같은 답을 받는다.
 */
export async function fetchScreenCatalogServer(
  market: string = SCREENER_DEFAULT_MARKET,
): Promise<ScreenCatalog | null> {
  try {
    const res = await fetchWithDeadline(
      `${apiBase()}/api/v1/stocks/screen/fields?market=${encodeURIComponent(market)}`,
      { next: { revalidate: 900 } },
    );
    if (!res.ok) return null;
    const body = (await res.json()) as { data: ScreenCatalog | null };
    return body.data ?? null;
  } catch {
    return null;
  }
}

async function runDefaultScreen(): Promise<ScreenResult | null> {
  try {
    const res = await fetchWithDeadline(`${apiBase()}/api/v1/stocks/screen`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(defaultScreenQuery()),
      /*
        데이터 캐시를 **명시적으로 끈다.** 바깥의 `unstable_cache`가 이미 결과를
        들고 있으므로 같은 값을 두 겹으로 두면 무효화 시점이 둘로 갈린다 — 하나가
        먼저 만료돼도 다른 하나가 옛 값을 계속 돌려준다.
      */
      cache: "no-store",
    });
    if (!res.ok) return null;
    const body = (await res.json()) as { data: ScreenResult | null };
    const data = body.data;
    /*
      0건은 심지 않는다. 기본 조건(점수 70 이상)에 맞는 종목이 하나도 없는 날은
      점수 배치가 돌지 않은 날이고, 그건 「조건이 빡빡하다」가 아니라 **아직
      모른다**이다. `null`이면 화면은 평소대로 조회부터 시작한다.
    */
    if (!data || data.rows.length === 0) return null;
    return data;
  } catch {
    return null;
  }
}

/**
 * 기본 조건의 조회 결과.
 *
 * 재검증 300초. 점수·재무는 하루 한 번 바뀌지만 표에 **종가와 등락률**이 같이
 * 실려 오므로 장중에는 그보다 자주 움직인다. 5분은 그 둘 사이의 타협이고,
 * 어차피 브라우저가 마운트 직후 같은 질의를 다시 보내 덮어쓴다.
 */
export const fetchDefaultScreenServer = unstable_cache(
  runDefaultScreen,
  ["screener-default-run", SCREENER_DEFAULT_MARKET],
  { revalidate: 300, tags: ["screener-default"] },
);
