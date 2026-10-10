import { Suspense } from "react";
import { LoansClient } from "@/components/finance/loans-client";
import { PageHeading } from "@/components/layout/page-heading";
import { resolvePageMetadata } from "@/lib/page-meta.server";

export function generateMetadata() {
  return resolvePageMetadata("/finance/loans");
}

/** 대출 분석 (2026-09-29, 10-09 이름 변경). 데이터는 worker-filings `fin_products`가 매일 05:10 채운다. */
export default function FinanceLoansPage() {
  return (
    <>
      <PageHeading>대출 분석</PageHeading>
      <Suspense>
        <LoansClient />
      </Suspense>
    </>
  );
}
