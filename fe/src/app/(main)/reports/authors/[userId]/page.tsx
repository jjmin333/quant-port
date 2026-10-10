import { Suspense } from "react";
import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { ResearchAuthorClient } from "@/components/research/research-author-client";
import { resolvePageMetadata } from "@/lib/page-meta.server";
import { fetchResearchAuthorServer } from "@/lib/research-api.server";
import { pageMetadata } from "@/lib/seo";

type Props = { params: Promise<{ userId: string }> };

function parseUserId(raw: string): number | null {
  const id = Number(raw);
  return Number.isInteger(id) && id > 0 ? id : null;
}

/*
 * 작성자 프로필은 **공개 리포트가 있을 때만** 색인한다 (2026-10-09 사용자 결정 — 검색 색인 개선 D).
 * 예전에는 전부 noindex였다: 클라이언트만 그려 제목이 모두 「작성자」였고, 그런 주소는 서로
 * 똑같은 빈 문서다. 이제 서버가 이름·리포트 목록을 미리 받아 그리므로 작성자마다 다른 문서가 되고,
 * 리포트끼리 이어 주는 허브가 된다. 공개 리포트가 0이면 내용이 없으니 그대로 noindex.
 *
 * 관리자 덮어쓰기(`/admin/seo`)는 공개 리포트가 없는 경우의 기본값으로 남는다.
 */
export async function generateMetadata({ params }: Props): Promise<Metadata> {
  const id = parseUserId((await params).userId);
  const profile = id ? await fetchResearchAuthorServer(id) : null;
  if (!profile?.authorName || profile.calls.length === 0) {
    return resolvePageMetadata("/reports/authors/[userId]");
  }
  return pageMetadata({
    title: `${profile.authorName}의 리포트`,
    description: `${profile.authorName}이(가) 발행한 공개 투자 리포트 ${profile.calls.length}건과 적중 기록입니다.`,
    path: `/reports/authors/${id}`,
  });
}

export default async function ReportAuthorPage({ params }: Props) {
  const id = parseUserId((await params).userId);
  if (!id) notFound();
  // generateMetadata와 같은 주소·같은 revalidate라 Next가 한 번만 부른다.
  const initial = await fetchResearchAuthorServer(id);

  return (
    <Suspense>
      <ResearchAuthorClient userId={id} initial={initial} />
    </Suspense>
  );
}
