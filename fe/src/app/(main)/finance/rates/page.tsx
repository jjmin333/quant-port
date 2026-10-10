import { RatesClient } from "@/components/wealth/rates-client";
import { PageHeading } from "@/components/layout/page-heading";
import { pageMetadata } from "@/lib/seo";

export function generateMetadata() {
  return pageMetadata({
    title: "금리",
    description: "기준금리·국고채·회사채·CD와 예금·주담대 평균금리, 국고채 수익률 곡선(한국은행 ECOS)",
    path: "/finance/rates",
  });
}

export default function RatesPage() {
  return (
    <>
      <PageHeading>금리 동향</PageHeading>
      <RatesClient />
    </>
  );
}
