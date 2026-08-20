import type { NewsArticle } from "@/lib/news-types";
import { newsDisplayHeadline, newsDisplaySummary, newsOriginalHeadline } from "@/lib/news-display";

export function newsCardSummary(
  article: Pick<NewsArticle, "market" | "headline" | "headlineKo" | "summary" | "summaryKo">,
): string | undefined {
  const headline = newsDisplayHeadline(article).trim();
  const summary = newsDisplaySummary(article)?.trim();
  const original = newsOriginalHeadline(article)?.trim();
  if (!summary) {
    return undefined;
  }
  if (summary === headline) {
    return undefined;
  }
  if (original && summary === original) {
    return undefined;
  }
  return summary;
}
