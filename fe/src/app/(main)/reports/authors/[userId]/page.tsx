import { Suspense } from "react";
import { notFound } from "next/navigation";
import { ResearchAuthorClient } from "@/components/research/research-author-client";

type Props = { params: Promise<{ userId: string }> };

export default async function ReportAuthorPage({ params }: Props) {
  const { userId } = await params;
  const id = Number(userId);
  if (!Number.isInteger(id) || id <= 0) notFound();

  return (
    <Suspense>
      <ResearchAuthorClient userId={id} />
    </Suspense>
  );
}
