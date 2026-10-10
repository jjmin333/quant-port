import { Suspense } from "react";

import { MarketRadarPageClient } from "@/components/market-radar/market-radar-page-client";
import { fetchRankingsServer } from "@/lib/market-api.server";
import { parseMarketRadarParams } from "@/lib/market-radar-config";
import { PageHeading } from "@/components/layout/page-heading";

/*
 * 메타데이터는 `market/layout.tsx`가 들고 있다. 이 페이지가 `"use client"`였던
 * 동안 `generateMetadata`를 내보낼 수 없어 그리로 갔던 것인데(2026-09-22에 서버
 * 컴포넌트로 되돌아왔다), **그대로 둔다** — 같은 값을 두 곳에서 내보내면 어느
 * 쪽이 이기는지가 규약이 아니라 프레임워크 세부가 된다. 옮길 거면 한 번에 옮기고
 * 레이아웃을 지우는 게 맞고, 그건 이 작업의 범위가 아니다.
 */

/**
 * 마켓 레이더의 첫 목록을 **서버에서 미리 받는다** (2026-09-22 QA 기획서 §0 문제 3).
 *
 * 그전까지 이 화면의 서버 HTML에는 「**마켓 레이더 로딩 중…** / 랭킹 로딩 중… /
 * 차트 패널 준비 중…」이 전부였다 — 종목명이 한 개도 없었다. 실측은
 * `lib/market-api.server.ts` 머리글.
 *
 * **기본 손잡이일 때만 받는다.** 사용자가 `?market=US&duration=1w`를 달고 오면
 * 서버가 미리 받은 국내·거래대금·실시간 목록은 **틀린 목록**이고, 그걸 심으면
 * 화면이 한 프레임 동안 거짓말을 한다. `analysis/page.tsx`가 `!params.market &&
 * !params.axis`로, `news/domestic/page.tsx`가 `hasNewsFilterParams`로 같은 판단을
 * 한다 — 크롤러는 언제나 맨 주소로 오므로 이 조건이 크롤러를 놓치지 않는다.
 *
 * `?symbol=`은 손잡이가 아니라 **오른쪽 칸**이 무엇을 여는지라 목록과 무관하다 —
 * 그래서 여기서 보지 않는다.
 */
export default async function MarketRadarPage({
  searchParams,
}: {
  searchParams: Promise<Record<string, string | string[] | undefined>>;
}) {
  const params = await searchParams;
  const defaultKnobs = !params.market && !params.type && !params.duration;
  /*
    기본값은 지어내지 않고 **화면과 같은 파서**에서 받는다. 빈 쿼리를 넣으면
    `parseMarketRadarParams`가 국내·거래대금·실시간·100건을 돌려주는데, 그건
    손잡이 없이 들어온 브라우저가 첫 렌더에서 계산할 값과 정의상 같다. 여기에
    리터럴을 따로 적으면 기본값이 바뀌는 날 둘이 조용히 갈라진다.
  */
  const seed = defaultKnobs
    ? await fetchRankingsServer(parseMarketRadarParams(new URLSearchParams()))
    : null;

  return (
    <>
      <PageHeading>실시간 차트</PageHeading>
      <Suspense
        fallback={
          <main className="flex flex-1 items-center justify-center p-8 text-sm text-[var(--text-muted)]">
            마켓 레이더 로딩 중…
          </main>
        }
      >
        <MarketRadarPageClient seed={seed} />
      </Suspense>
    </>
  );
}
