import { Suspense } from "react";
import { EarningsClient } from "@/components/earnings/earnings-client";
import { EarningsSeoList } from "@/components/earnings/earnings-seo-list";
import { PageHeading } from "@/components/layout/page-heading";
import { resolvePageMetadata } from "@/lib/page-meta.server";

/*
 * 제목·설명은 `lib/page-meta-routes`의 기본값이고, 관리자가 `/admin/seo`에서
 * 덮어쓸 수 있다 (2026-09-15 사용자 요청 2). `const metadata`가 아니라 함수인
 * 이유는 그 덮어쓰기를 **렌더 시점에** 읽어야 하기 때문이다.
 */
export function generateMetadata() {
  return resolvePageMetadata("/earnings/domestic");
}

export default function DomesticEarningsPage() {
  return (
    <>
      <PageHeading>국내 실적 발표 일정</PageHeading>
      {/* 서버가 그리는 2주 발표 목록 — 크롤러가 종목 페이지로 가는 길(2026-10-10). */}
      <EarningsSeoList market="domestic" />
      <Suspense>
        <EarningsClient market="domestic" />
      </Suspense>
    </>
  );
}
