import { Suspense } from "react";
import { AnalysisClient } from "@/components/analysis/analysis-client";
import { resolvePageMetadata } from "@/lib/page-meta.server";
import { PageHeading } from "@/components/layout/page-heading";
import { fetchScoreTopServer } from "@/lib/home-api.server";

/** URL에 `?symbol=`이 없을 때 기본으로 여는 종목 (삼성전자). */
const DEFAULT_SYMBOL = "005930";

/*
 * canonical을 `?symbol=` 없는 `/analysis` 하나로 고정한다. 이 화면은 쿼리로만
 * 종목이 바뀌는데, 그대로 두면 종목 수만큼의 주소가 **거의 같은 문서**로 색인돼
 * 서로의 순위를 깎는다. 종목별 페이지가 필요해지면 `/stocks/<종목>`을 서버에서
 * 그리도록 만드는 게 먼저다 — `docs/SEO.md` 후속 항목.
 */
/*
 * 제목·설명은 `lib/page-meta-routes`의 기본값이고, 관리자가 `/admin/seo`에서
 * 덮어쓸 수 있다 (2026-09-15 사용자 요청 2). `const metadata`가 아니라 함수인
 * 이유는 그 덮어쓰기를 **렌더 시점에** 읽어야 하기 때문이다.
 */
export function generateMetadata() {
  return resolvePageMetadata("/analysis");
}

/** 목록이 그리는 종목 수. `ScoreRankingList`의 `SCORE_RANKING_COUNT`와 같아야 한다. */
const RANKING_SEED_COUNT = 100;

/**
 * 랭킹 첫 목록을 **서버에서 미리 받는다** (2026-09-20 점검 요청 5·9).
 *
 * 그전까지 이 화면의 서버 HTML에는 「**랭킹 로딩 중…** / 시세 로딩 중… / 점수
 * 불러오는 중…」이 전부였다 — 종목명이 한 개도 없었다. 실측은
 * `lib/home-api.server.ts` 머리글.
 *
 * **기본 손잡이일 때만 받는다.** 사용자가 `?market=overseas&axis=growth`를 달고
 * 오면 서버가 미리 받은 국내·종합 목록은 **틀린 목록**이고, 그걸 심으면 화면이
 * 한 프레임 동안 거짓말을 한다. `news/domestic/page.tsx`가 `hasNewsFilterParams`로
 * 같은 판단을 한다 — 크롤러는 언제나 맨 주소로 오므로 이 조건이 크롤러를 놓치지
 * 않는다.
 *
 * `?symbol=`은 손잡이가 아니라 **오른쪽 칸**이 무엇을 여는지라 목록과 무관하다 —
 * 그래서 여기서 보지 않는다.
 */
export default async function AnalysisPage({
  searchParams,
}: {
  searchParams: Promise<Record<string, string | string[] | undefined>>;
}) {
  const params = await searchParams;
  const defaultRanking = !params.market && !params.axis && !params.duration;
  const rankingSeed = defaultRanking
    ? await fetchScoreTopServer("domestic", RANKING_SEED_COUNT)
    : undefined;

  return (
    <>
      <PageHeading>AI 점수 순위</PageHeading>
      <Suspense
        fallback={
          <main className="flex flex-1 items-center justify-center p-8 text-sm text-[var(--text-muted)]">
            분석 화면을 불러오는 중…
          </main>
        }
      >
        <AnalysisClient defaultSymbol={DEFAULT_SYMBOL} rankingSeed={rankingSeed} />
      </Suspense>
    </>
  );
}
