import { NewsClient } from "@/components/news/news-client";
import { fetchNewsListServer } from "@/lib/news-api.server";
import { breadcrumbJsonLd, pageMetadata } from "@/lib/seo";
import { JsonLd } from "@/components/seo/json-ld";
import { hasNewsFilterParams, type NewsSearchParams } from "@/lib/news-ssr";

export const metadata = pageMetadata({
  title: "국내 주식 뉴스",
  description:
    "국내 증시 뉴스를 종목별로 묶어 보여줍니다. 기사에 언급된 종목과 키워드가 함께 붙어, 내가 보는 종목의 소식만 골라 읽을 수 있습니다.",
  path: "/news/domestic",
});

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
        data={breadcrumbJsonLd([{ name: "국내 주식 뉴스", path: "/news/domestic" }])}
      />
      <NewsClient market="domestic" initialArticles={initialArticles} />
    </>
  );
}
