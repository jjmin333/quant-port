import { Suspense } from "react";
import { SavingsClient } from "@/components/finance/savings-client";
import { PageHeading } from "@/components/layout/page-heading";
import { resolvePageMetadata } from "@/lib/page-meta.server";

export function generateMetadata() {
  return resolvePageMetadata("/finance/savings");
}

/** 예·적금 분석 (2026-09-29, 10-09 이름 변경). 데이터는 worker-filings `fin_products`가 매일 05:10 채운다. */
export default function FinanceSavingsPage() {
  return (
    <>
      <PageHeading>예·적금 분석</PageHeading>
      <Suspense>
        <SavingsClient />
      </Suspense>
    </>
  );
}
