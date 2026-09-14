import { Suspense } from "react";
import { AnalysisClient } from "@/components/analysis/analysis-client";
import { pageMetadata } from "@/lib/seo";

/** URL에 `?symbol=`이 없을 때 기본으로 여는 종목 (삼성전자). */
const DEFAULT_SYMBOL = "005930";

/*
 * canonical을 `?symbol=` 없는 `/analysis` 하나로 고정한다. 이 화면은 쿼리로만
 * 종목이 바뀌는데, 그대로 두면 종목 수만큼의 주소가 **거의 같은 문서**로 색인돼
 * 서로의 순위를 깎는다. 종목별 페이지가 필요해지면 `/stocks/<종목>`을 서버에서
 * 그리도록 만드는 게 먼저다 — `docs/SEO.md` 후속 항목.
 */
export const metadata = pageMetadata({
  title: "AI 추천 랭킹",
  description:
    "재무·가격·수급 등 다섯 축의 점수로 종목을 줄 세운 랭킹입니다. 점수의 근거가 된 지표와 최근 공시·실적을 같은 화면에서 확인할 수 있습니다.",
  path: "/analysis",
});

export default async function AnalysisPage() {
  return (
    <>
      <Suspense
        fallback={
          <main className="flex flex-1 items-center justify-center p-8 text-sm text-[var(--text-muted)]">
            분석 화면을 불러오는 중…
          </main>
        }
      >
        <AnalysisClient defaultSymbol={DEFAULT_SYMBOL} />
      </Suspense>
    </>
  );
}
