import { Suspense } from "react";
import { PageHeader } from "@/components/layout/header";
import { AnalysisClient } from "@/components/analysis/analysis-client";

/** URL에 `?symbol=`이 없을 때 기본으로 여는 종목 (삼성전자). */
const DEFAULT_SYMBOL = "005930";

export default async function AnalysisPage() {
  return (
    <>
      <PageHeader title="AI 추천 랭킹" subtitle="기업 정보 · 밸류에이션 · 재무제표" />
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
