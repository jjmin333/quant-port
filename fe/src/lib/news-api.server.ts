import { fetchDomesticNews, fetchUsNews } from "@/lib/news-api";
import type { NewsArticle } from "@/lib/news-types";

/**
 * 뉴스 목록 첫 화면을 서버에서 미리 받아 온다.
 *
 * 목적은 `research-api.server.ts`와 같다 — 크롤러가 받는 HTML에 **기사 제목이
 * 실리게** 하는 것. 지금까지 `/news/domestic`의 본문은 사이드바 1,827자와
 * 「불러오는 중…」이 전부였다.
 *
 * **기본 정렬·필터일 때만 쓴다.** 사용자가 `?sort=popular` 같은 걸 달고 오면
 * 서버가 미리 받은 목록은 틀린 목록이라, 부르는 쪽(page.tsx)이 아예 건너뛴다.
 * 크롤러는 언제나 맨 주소로 오므로 이 조건이 크롤러를 놓치지 않는다.
 */
const REVALIDATE = 300;

export async function fetchNewsListServer(
  market: "domestic" | "us",
): Promise<NewsArticle[]> {
  try {
    const fetchNews = market === "domestic" ? fetchDomesticNews : fetchUsNews;
    const { articles } = await fetchNews(
      { market, sourceId: "all", sort: "latest" },
      { next: { revalidate: REVALIDATE } },
    );
    return articles;
  } catch {
    // SEO용 선반입이 화면을 막지 않는다 — 클라이언트가 어차피 다시 받는다.
    return [];
  }
}
