import { Suspense } from "react";
import { notFound } from "next/navigation";
import { ResearchDetailClient } from "@/components/research/research-detail-client";

type Props = { params: Promise<{ id: string }> };

export default async function ReportDetailPage({ params }: Props) {
  const { id } = await params;
  const reportId = Number(id);
  if (!Number.isInteger(reportId) || reportId <= 0) notFound();

  return (
    <Suspense>
      <ResearchDetailClient id={reportId} />
    </Suspense>
  );
}
