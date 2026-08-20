import type { NewsArticle } from "@/lib/news-types";

function normalizeHeadlineKey(text: string): string {
  return text
    .replace(/^financialjuice:\s*/i, "")
    .replace(/\s+[-–—|]\s+[^-–—|]{2,80}$/i, "")
    .replace(/\.\.\.$/u, "")
    .replace(/…$/u, "")
    .trim()
    .toLowerCase()
    .replace(/\s+/g, " ");
}

function articleKeys(article: NewsArticle): string[] {
  const keys = new Set<string>();
  const en = normalizeHeadlineKey(article.headline);
  const ko = normalizeHeadlineKey(article.headlineKo ?? "");
  if (en) keys.add(`en:${en}`);
  if (ko) keys.add(`ko:${ko}`);
  const url = (article.originalUrl ?? article.url ?? "").trim().toLowerCase();
  if (url.includes("financialjuice.com")) {
    keys.add(`url:${url.split("?")[0]?.replace(/\/$/, "") ?? url}`);
  }
  if (keys.size === 0) keys.add(`id:${article.id}`);
  return [...keys];
}

function sourcePriority(sourceId: string | undefined): number {
  switch (sourceId) {
    case "financial-juice":
      return 4;
    case "reuters":
      return 3;
    case "finnhub":
      return 2;
    case "google_news":
      return 1;
    default:
      return 0;
  }
}

function preferArticle(candidate: NewsArticle, current: NewsArticle): boolean {
  const candidatePriority = sourcePriority(candidate.source?.id);
  const currentPriority = sourcePriority(current.source?.id);
  if (candidatePriority !== currentPriority) {
    return candidatePriority > currentPriority;
  }
  if (candidate.headlineKo?.trim() && !current.headlineKo?.trim()) {
    return true;
  }
  if (!candidate.headlineKo?.trim() && current.headlineKo?.trim()) {
    return false;
  }
  return new Date(candidate.publishedAt).getTime() >= new Date(current.publishedAt).getTime();
}

function articlesOverlap(a: NewsArticle, b: NewsArticle): boolean {
  const keysB = new Set(articleKeys(b));
  return articleKeys(a).some((key) => keysB.has(key));
}

export function dedupeNewsArticles(articles: NewsArticle[]): NewsArticle[] {
  if (articles.length < 2) {
    return articles;
  }
  const groups: NewsArticle[][] = [];
  for (const article of articles) {
    let merged = false;
    for (const group of groups) {
      if (articlesOverlap(article, group[0]!)) {
        group.push(article);
        merged = true;
        break;
      }
    }
    if (!merged) {
      groups.push([article]);
    }
  }
  return groups.map((group) =>
    group.reduce((best, candidate) => (preferArticle(candidate, best) ? candidate : best)),
  );
}
