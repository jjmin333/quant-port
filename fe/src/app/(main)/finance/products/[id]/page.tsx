import { notFound } from "next/navigation";
import { ProductDetailClient } from "@/components/finance/product-detail-client";
import { PageHeading } from "@/components/layout/page-heading";
import { pageMetadata } from "@/lib/seo";

type Props = { params: Promise<{ id: string }> };

// 상품 상세는 색인하지 않는다 — id는 우리 표의 번호라 공시가 바뀌면 사라질 수 있다.
export function generateMetadata() {
  return pageMetadata({ title: "금융상품 상세", description: "금융감독원 공시 기준 금융상품 상세와 계산", noindex: true });
}

export default async function FinanceProductPage({ params }: Props) {
  const id = Number((await params).id);
  if (!Number.isInteger(id) || id <= 0) notFound();
  return (
    <>
      <PageHeading>금융상품 상세</PageHeading>
      <ProductDetailClient id={id} />
    </>
  );
}
