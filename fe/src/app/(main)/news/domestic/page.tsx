import { NewsClient } from "@/components/news/news-client";
import { fetchNewsListServer } from "@/lib/news-api.server";
import { resolvePageMetadata } from "@/lib/page-meta.server";
import { breadcrumbJsonLd } from "@/lib/seo";
import { JsonLd } from "@/components/seo/json-ld";
import { PageHeading } from "@/components/layout/page-heading";
import { hasNewsFilterParams, type NewsSearchParams } from "@/lib/news-ssr";
import { pageRoute } from "@/lib/page-meta-routes";

// 이름은 라우트 표 한 곳에서 — 사이드바·SEO·제목이 따로 적혀 어긋나지 않게(2026-10-09).
const LABEL = pageRoute("/news/domestic")?.label ?? "실시간 국내 뉴스";

/*
 * 제목·설명은 `lib/page-meta-routes`의 기본값이고, 관리자가 `/admin/seo`에서
 * 덮어쓸 수 있다 (2026-09-15 사용자 요청 2). `const metadata`가 아니라 함수인
 * 이유는 그 덮어쓰기를 **렌더 시점에** 읽어야 하기 때문이다.
 */
export function generateMetadata() {
  return resolvePageMetadata("/news/domestic");
}

export default async function DomesticNewsPage({
  searchParams,
}: {
  searchParams: Promise<NewsSearchParams>;
}) {
  const params = await searchParams;
  const initialArticles = hasNewsFilterParams(params)
    ? []
    : await fetchNewsListServer("domestic");

  return (
    <>
      <JsonLd
        data={breadcrumbJsonLd([{ name: LABEL, path: "/news/domestic" }])}
      />
      <PageHeading>{LABEL}</PageHeading>
      <NewsClient market="domestic" initialArticles={initialArticles} />
    </>
  );
}
