import { RealEstateClient } from "@/components/wealth/real-estate-client";
import { PageHeading } from "@/components/layout/page-heading";
import { pageMetadata } from "@/lib/seo";

export function generateMetadata() {
  return pageMetadata({
    title: "부동산 실거래 시세",
    description: "국토부 실거래가로 본 아파트·오피스텔·연립 단지별 추정가·전세 시세와 한국부동산원 지역 지수",
    path: "/real-estate",
  });
}

/** 부동산 시세 (2026-10-01) — 매물(호가)이 아니라 신고된 실거래로. */
export default function RealEstatePage() {
  return (
    <>
      <PageHeading>부동산 실거래 시세</PageHeading>
      <RealEstateClient />
    </>
  );
}
