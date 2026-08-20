import { PageHeader } from "@/components/layout/header";
import { PageScrollShell } from "@/components/layout/page-content-shell";
import { DashboardOverview } from "@/components/dashboard/dashboard-overview";
import { IndexTickerStrip } from "@/components/market/index-ticker-strip";

export default async function HomePage() {
  return (
    <>
      <PageHeader title="홈" subtitle="시장 현황 · 관심종목 · 뉴스" />
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
