import { Suspense } from "react";
import { notFound } from "next/navigation";
import { ResearchAuthorClient } from "@/components/research/research-author-client";
import { pageMetadata } from "@/lib/seo";

type Props = { params: Promise<{ userId: string }> };

/*
 * 작성자 프로필은 색인하지 않는다 — 리포트 본문은 `/reports/<id>`가 이미 들고
 * 있어서, 프로필까지 색인하면 같은 글 목록이 두 주소로 잡힌다.
 */
export const metadata = pageMetadata({
  title: "작성자",
  description: "이 작성자가 발행한 리포트 목록입니다.",
  noindex: true,
});

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
