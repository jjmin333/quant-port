import { PageScrollShell } from "@/components/layout/page-content-shell";
import { DashboardOverview } from "@/components/dashboard/dashboard-overview";
/*
  줄 수 상수를 `dashboard-overview`에서 직접 가져오면 **`undefined`가 온다** —
  그쪽은 `"use client"`라 서버 그래프에서 클라이언트 참조로 바뀐다. 컴파일도
  lint도 통과하고 **요청만 조용히 틀린다.** 실측과 증상은 `lib/dashboard-rows.ts`.
*/
import { DASHBOARD_RANK_ROWS } from "@/lib/dashboard-rows";
import { IndexTickerStrip } from "@/components/market/index-ticker-strip";
import { fetchIndicesServer, fetchScoreTopServer } from "@/lib/home-api.server";
import type { Metadata } from "next";
import { PageHeading } from "@/components/layout/page-heading";
import { SITE_TITLE } from "@/lib/seo";
import { cookies } from "next/headers";
import { HomeGuestShowcase } from "@/components/dashboard/home-guest-showcase";
import { fetchGuestShowcase } from "@/lib/home-guest.server";

/*
 * 홈은 제목·설명·og를 루트 레이아웃 것 그대로 쓴다(브랜드 문구가 곧 홈 문구다).
 * canonical만 여기서 붙인다 — 루트에 두면 모든 화면이 홈을 정규 주소라고
 * 말하게 된다(`layout.tsx` 주석 참조).
 */
export const metadata: Metadata = { alternates: { canonical: "/" } };

/**
 * 홈의 두 블록을 **서버에서 미리 받는다** (2026-09-20 SSR/CSR 점검 요청 1·5·9).
 *
 * 그전까지 홈의 서버 HTML에는 데이터가 한 글자도 없었다 — 실측하면 「오늘의 시장
 * **갱신 중…** / AI 추천 랭킹 TOP / 거래대금 상위 …」처럼 **구획 이름만** 나왔다.
 * 크롤러에게도, JS가 늦은 사용자에게도 그 문서는 목차였다.
 *
 * 둘만 고른 이유는 **값 하나가 TTFB 하나**이기 때문이다. 지수와 AI 랭킹은 종목명과
 * 숫자를 HTML에 싣는 데 가장 크게 기여하고 응답이 각각 3.8ms·13.2ms다(실측
 * 2026-09-20). 브리핑·뉴스 카드·거래대금은 CSR로 남는다 — 운영 DB는 1 ECPU 하나이고,
 * 홈은 가장 많이 열리는 화면이다.
 *
 * `Promise.all`인 이유: 둘을 차례로 기다리면 TTFB에 둘의 합이 붙는다. 실패는 각
 * 함수가 안에서 삼키고 빈 값을 주므로 여기서 `catch`할 것이 없다 —
 * 선반입은 **먼저 보여 주는 일**이지 화면이 서 있을 조건이 아니다.
 */
export default async function HomePage() {
  /*
    비로그인 블록(2026-10-10 사용자 결정 — 시안 home-guest-v2 A안)은 **세션 쿠키가 없을 때만** 받는다.
    로그인한 사람의 홈은 그대로이고 그 요청도 만들지 않는다.
  */
  const jar = await cookies();
  const guest = !jar.has("qp_access") && !jar.has("qp_refresh");
  const [indices, scoreTop, showcase] = await Promise.all([
    fetchIndicesServer(),
    // 홈 카드가 그리는 줄 수와 같아야 한다 — 더 받으면 HTML만 커지고, 덜 받으면
    // 마운트 직후 목록이 한 번 늘어나며 레이아웃이 튄다.
    fetchScoreTopServer("domestic", DASHBOARD_RANK_ROWS),
    guest ? fetchGuestShowcase() : Promise.resolve(null),
  ]);

  return (
    <>
      <PageHeading>{SITE_TITLE}</PageHeading>
      <PageScrollShell contentClassName="space-y-3">
        {/*
          지수 전 종목이 한 줄로 흐른다. 예전에는 다섯 장만 깔고 나머지 일곱을 화면 맨
          아래 「전체 지수」 접힘에 넣어 뒀는데, 접힌 것은 사실상 아무도 펼치지 않아
          홈이 실제로 보여주는 지수는 다섯이었다. 그래서 그 접힘 섹션은 없앴다.
          맨 위에 둔다(10-11 사용자 요청) — 비로그인 소개 블록보다도 위.
        */}
        <IndexTickerStrip seed={indices} />
        {showcase && <HomeGuestShowcase data={showcase} />}

        <DashboardOverview scoreSeed={scoreTop} />
      </PageScrollShell>
    </>
  );
}
