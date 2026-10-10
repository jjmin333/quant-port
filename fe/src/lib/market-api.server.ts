import { fetchWithDeadline } from "@/lib/fetch-timeout";
import type { MarketRadarParams, RankingsResponse } from "@/lib/market-radar-types";

/**
 * 마켓 레이더의 **첫 목록을 서버에서 미리 받아 온다** (2026-09-22 QA 기획서 §0 문제 3).
 *
 * # 무엇이 잘못돼 있었나
 *
 * `/market`은 `page.tsx`가 통째로 `"use client"`라 서버 HTML에 데이터가 한 글자도
 * 없었다. 실측(2026-09-22, 공개 사이트):
 *
 *   `/market` → 「**마켓 레이더 로딩 중…** / 랭킹 로딩 중… / 차트 패널 준비 중…」
 *
 * 종목명이 한 개도 없다. 홈과 `/analysis`가 09-20에 같은 상태에서 빠져나왔고
 * (`lib/home-api.server.ts` 머리글), 이 파일은 그 패턴을 세 번째 화면에 붙인다.
 *
 * # 왜 쿠키를 안 보내는가
 *
 * `/rankings`는 `router.go`의 `public` 그룹이다 — 실측으로 확인했다(쿠키 없이
 * 200 · 23.3KB · 100행). 쿠키를 붙이면 Next의 데이터 캐시가 한 사용자의 쿠키로
 * 받은 응답을 모두에게 돌려줄 위험이 생기고, 응답이 사용자마다 달라질 이유가
 * 없는데 캐시가 사용자 수만큼 갈라진다.
 *
 * 관심·보유 표시는 **선반입하지 않는다** — 그건 마운트 뒤에 클라이언트가 얹는다
 * (`use-market-rankings`의 `enrichRows`). 서버가 그것까지 그리면 hydration이
 * 어긋난다. 어긋나지 않는 이유까지 확인해 두었다: `lib/watchlist-store`의 집합은
 * 모듈 전역이고 localStorage를 읽지 않아, 서버에서도 브라우저 첫 프레임에서도
 * **비어 있다.**
 *
 * # 실패는 화면을 막지 않는다
 *
 * `catch`로 `null`을 돌려준다. 선반입은 **먼저 보여 주는 일**이지 화면이 설 조건이
 * 아니다 — 클라이언트가 어차피 마운트 직후 다시 받는다. 상한도 3초로 짧다
 * (`SSR_TIMEOUT_MS`): 여기서 기다리는 시간은 TTFB에 그대로 더해진다.
 */

/** 서버 안에서는 nginx를 거치지 않고 BE로 직접 간다. */
function apiBase(): string {
  return process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080";
}

/**
 * 재검증 60초.
 *
 * 순위 스냅샷 자체는 BE 폴러가 5분 주기로 갱신하지만 **가격·등락률이 같이 실려
 * 오므로** 5분을 통째로 캐시할 수는 없다. 클라이언트의 `cachedGet` TTL이 30초라
 * 그보다 길게 두어, 마운트 직후의 클라이언트 요청이 선반입값을 **덮어쓰는 쪽**이
 * 되게 한다 — `fetchScoreTopServer`가 같은 이유로 같은 숫자를 쓴다.
 */
const RANKINGS_REVALIDATE_SEC = 60;

export async function fetchRankingsServer(
  params: MarketRadarParams,
): Promise<RankingsResponse | null> {
  const q = new URLSearchParams({
    type: params.type,
    marketCountry: params.market,
    duration: params.duration,
  });
  if (params.count > 0) q.set("count", String(params.count));

  try {
    const res = await fetchWithDeadline(`${apiBase()}/api/v1/rankings?${q}`, {
      next: { revalidate: RANKINGS_REVALIDATE_SEC },
    });
    if (!res.ok) return null;
    const body = (await res.json()) as { data: RankingsResponse | null };
    const data = body.data;
    /*
      **빈 목록은 심지 않는다.** 마감 세션에도 `/rankings`는 마지막 스냅샷을 주므로
      (실측: 마감 중 23.3KB·100행) 여기서 0건이 오는 것은 정상 상태가 아니라
      사실상 실패다. 그걸 씨앗으로 넘기면 화면이 `loading:false` + 0건으로 서서
      「표시할 종목이 없습니다」라고 **단정**한다 — 클라이언트가 곧 받아 올 값을
      두고 하는 거짓말이다. `null`을 주면 화면은 평소대로 로딩부터 시작한다.
    */
    if (!data || data.rankings.length === 0) return null;
    return data;
  } catch {
    return null;
  }
}
