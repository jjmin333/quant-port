import type { NewsArticle } from "@/lib/news-types";
import { isBreakingNews } from "@/lib/news-index-filter";
import { isUSBreakingNews } from "@/lib/news-us-index-filter";

export const NEWS_BREAKING_LABEL = "속보";

export function isArticleBreakingNews(
  article: NewsArticle,
  nowMs: number = Date.now(),
): boolean {
  if (article.market === "us") {
    return isUSBreakingNews(article, nowMs);
  }
  return isBreakingNews(article, nowMs);
}
