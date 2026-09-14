import { PageScrollShell } from "@/components/layout/page-content-shell";
import { DashboardOverview } from "@/components/dashboard/dashboard-overview";
import { IndexTickerStrip } from "@/components/market/index-ticker-strip";
import type { Metadata } from "next";

/*
 * 홈은 제목·설명·og를 루트 레이아웃 것 그대로 쓴다(브랜드 문구가 곧 홈 문구다).
 * canonical만 여기서 붙인다 — 루트에 두면 모든 화면이 홈을 정규 주소라고
 * 말하게 된다(`layout.tsx` 주석 참조).
 */
export const metadata: Metadata = { alternates: { canonical: "/" } };

export default async function HomePage() {
  return (
    <>
      <PageScrollShell contentClassName="space-y-3">
        {/*
          지수 전 종목이 한 줄로 흐른다. 예전에는 다섯 장만 깔고 나머지 일곱을 화면 맨
          아래 「전체 지수」 접힘에 넣어 뒀는데, 접힌 것은 사실상 아무도 펼치지 않아
          홈이 실제로 보여주는 지수는 다섯이었다. 그래서 그 접힘 섹션은 없앴다.
        */}
        <IndexTickerStrip />

        <DashboardOverview />
      </PageScrollShell>
    </>
  );
}
