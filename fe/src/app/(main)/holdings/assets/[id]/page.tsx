import { notFound } from "next/navigation";
import { AssetDetailClient } from "@/components/wealth/asset-detail-client";
import { PageHeading } from "@/components/layout/page-heading";
import { pageMetadata } from "@/lib/seo";

type Props = { params: Promise<{ id: string }> };

export function generateMetadata() {
  return pageMetadata({ title: "자산 상세", description: "자산 평가 근거와 추이", noindex: true });
}

export default async function WealthAssetPage({ params }: Props) {
  const id = Number((await params).id);
  if (!Number.isInteger(id) || id <= 0) notFound();
  return (
    <>
      <PageHeading>자산 상세</PageHeading>
      <AssetDetailClient id={id} />
    </>
  );
}
