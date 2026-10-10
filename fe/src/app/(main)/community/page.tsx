import { Suspense } from "react";
import { CommunityListClient } from "@/components/community/community-list-client";
import { JsonLd } from "@/components/seo/json-ld";
import { fetchCommunityPostsServer } from "@/lib/community-api.server";
import { resolvePageMetadata } from "@/lib/page-meta.server";
import { breadcrumbJsonLd } from "@/lib/seo";

/*
 * 제목·설명은 `lib/page-meta-routes`의 기본값이고, 관리자가 `/admin/seo`에서 덮어쓸 수 있다.
 */
export function generateMetadata() {
  return resolvePageMetadata("/community");
}

type Props = { searchParams: Promise<{ symbol?: string }> };

/**
 * 커뮤니티 목록 (2026-09-27 사용자 요청 4). **첫 페이지를 서버가 HTML에 싣는다** — 검색 색인
 * 허용(결정)이고, 크롤러가 글 주소(`/community/<id>`)를 찾아가는 길이 이 목록이다.
 */
export default async function CommunityPage({ searchParams }: Props) {
  const { symbol } = await searchParams;
  const initialPage = await fetchCommunityPostsServer(symbol?.toUpperCase());
  return (
    <>
      <JsonLd data={breadcrumbJsonLd([{ name: "커뮤니티", path: "/community" }])} />
      <Suspense>
        <CommunityListClient initialPage={initialPage} initialSymbol={symbol?.toUpperCase()} />
      </Suspense>
    </>
  );
}
