import { AuctionsClient } from "@/components/wealth/auctions-client";
import { PageHeading } from "@/components/layout/page-heading";
import { pageMetadata } from "@/lib/seo";

export function generateMetadata() {
  return pageMetadata({
    title: "부동산 공매",
    description: "캠코 온비드 부동산 공매 물건 — 감정가·최저입찰가·유찰횟수와 낙찰가율 통계",
    path: "/real-estate/auctions",
  });
}

export default function AuctionsPage() {
  return (
    <>
      <PageHeading>부동산 공매</PageHeading>
      <AuctionsClient />
    </>
  );
}
