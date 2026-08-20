import type { NewsArticle } from "@/lib/news-types";

export function newsDisplayHeadline(article: Pick<NewsArticle, "market" | "headline" | "headlineKo">): string {
  if (article.market === "us") {
    const ko = article.headlineKo?.trim();
    if (ko) return ko;
  }
  return article.headline;
}

export function newsDisplaySummary(
  article: Pick<NewsArticle, "market" | "summary" | "summaryKo">,
): string | undefined {
  if (article.market === "us") {
    const ko = article.summaryKo?.trim();
    if (ko) return ko;
  }
  return article.summary;
}

export function newsOriginalHeadline(
  article: Pick<NewsArticle, "market" | "headline" | "headlineKo">,
): string | undefined {
  if (article.market !== "us") return undefined;
  const ko = article.headlineKo?.trim();
  const en = article.headline.trim();
  if (!ko || ko === en) return undefined;
  return en;
}
