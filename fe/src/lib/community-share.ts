import { COMMUNITY_LIMITS } from "@/lib/community-types";
import type { NewsArticle } from "@/lib/news-types";

/**
 * 뉴스 → 커뮤니티 공유 진입 주소 (2026-09-27 사용자 요청 4 「기사를 리포스트하듯」).
 * 제목은 글쓰기 화면의 표시용이다 — 서버는 기사 id로 원본을 다시 읽어 제목·출처를 저장한다.
 * 기사에 태그된 종목은 미리 붙여 둔다(최대 3개).
 */
export function communityShareHref(article: Pick<NewsArticle, "id" | "headline" | "headlineKo" | "symbols">): string {
  const q = new URLSearchParams({ article: article.id, title: article.headlineKo || article.headline });
  if (article.symbols?.length) q.set("symbols", article.symbols.slice(0, COMMUNITY_LIMITS.symbols).join(","));
  return `/community/new?${q.toString()}`;
}
