import { Suspense } from "react";
import { ScreenerClient } from "@/components/screener/screener-client";
import { resolvePageMetadata } from "@/lib/page-meta.server";
import { PageHeading } from "@/components/layout/page-heading";
import {
  fetchDefaultScreenServer,
  fetchScreenCatalogServer,
} from "@/lib/screener-api.server";

/*
 * 제목·설명은 `lib/page-meta-routes`의 기본값이고, 관리자가 `/admin/seo`에서
 * 덮어쓸 수 있다 — 다른 화면과 같은 규약이다(렌더 시점에 읽어야 해서 함수다).
 */
export function generateMetadata() {
  return resolvePageMetadata("/screener");
}

/**
 * 조건 목록과 **기본 조건의 결과**를 서버에서 미리 받는다 (2026-09-22 QA 기획서 §16).
 *
 * 그전까지 이 화면의 서버 HTML에는 「**스크리너를 불러오는 중…** / 조건 목록을
 * 불러오는 중입니다…」가 전부였다 — 종목명이 한 개도 없었다. 실측과 두 값을
 * 각각 어떻게 캐시하는지는 `lib/screener-api.server.ts` 머리글.
 *
 * **둘을 나란히 기다린다.** 순서대로 기다리면 상한(`SSR_TIMEOUT_MS`) 두 개가
 * 더해져 최악 6초가 TTFB에 실린다. 서로가 서로를 필요로 하지 않으므로 같이 간다.
 *
 * 손잡이를 보지 않는 이유는 `/market`·`/analysis`와 다르다 — 이 화면은 조건을
 * **URL에 싣지 않는다**(조회가 POST인 이유와 같다). 그래서 어느 주소로 들어오든
 * 첫 화면은 언제나 기본 조건이고, 심는 값이 틀릴 경우가 없다.
 */
export default async function ScreenerPage() {
  const [catalogSeed, resultSeed] = await Promise.all([
    fetchScreenCatalogServer(),
    fetchDefaultScreenServer(),
  ]);

  return (
    <>
      <PageHeading>종목 스크리너</PageHeading>
      <Suspense
        fallback={
          <main className="flex flex-1 items-center justify-center p-8 text-sm text-[var(--text-muted)]">
            스크리너를 불러오는 중…
          </main>
        }
      >
        <ScreenerClient catalogSeed={catalogSeed} resultSeed={resultSeed} />
      </Suspense>
    </>
  );
}
