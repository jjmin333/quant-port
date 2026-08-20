import type { NewsArticle } from "@/lib/news-types";

export type NewsIndexFilter =
  | "all"
  | "breaking"
  | "kospi"
  | "kosdaq"
  | "005930"
  | "000660"
  | "005380";

export type NewsIndexBoard = "kospi" | "kosdaq";

export type NewsStockFilter = {
  id: Extract<NewsIndexFilter, "005930" | "000660" | "005380">;
  label: string;
  terms: readonly string[];
};

export const NEWS_STOCK_FILTERS: NewsStockFilter[] = [
  { id: "005930", label: "삼성전자", terms: ["삼성전자", "005930"] },
  { id: "000660", label: "SK하이닉스", terms: ["sk하이닉스", "하이닉스", "000660"] },
  { id: "005380", label: "현대차", terms: ["현대차", "현대자동차", "005380"] },
];

/** Pill order in the news page tab bar. */
export const NEWS_INDEX_PILLS: NewsIndexFilter[] = [
  "all",
  "breaking",
  "kospi",
  "kosdaq",
  "005930",
  "000660",
  "005380",
];

const BREAKING_MAX_AGE_MS = 24 * 60 * 60 * 1000;

const KOSPI_TERMS = ["코스피", "kospi"] as const;
const KOSDAQ_TERMS = ["코스닥", "kosdaq"] as const;

const STOCK_FILTER_BY_ID = Object.fromEntries(
  NEWS_STOCK_FILTERS.map((stock) => [stock.id, stock]),
) as Record<NewsStockFilter["id"], NewsStockFilter>;

function articleText(article: Pick<NewsArticle, "headline" | "summary">): string {
  return `${article.headline} ${article.summary ?? ""}`.toLowerCase();
}

export function isBreakingNews(
  article: Pick<NewsArticle, "headline" | "tags" | "publishedAt">,
  nowMs: number = Date.now(),
): boolean {
  const published = new Date(article.publishedAt).getTime();
  if (Number.isNaN(published) || nowMs - published > BREAKING_MAX_AGE_MS) {
    return false;
  }
  if (article.tags?.includes("속보")) {
    return true;
  }
  const headline = article.headline.trim();
  return headline.includes("[속보]") || headline.includes("【속보】") || headline.startsWith("속보");
}

export function newsIndexBoards(
  article: Pick<NewsArticle, "headline" | "summary">,
): NewsIndexBoard[] {
  const text = articleText(article);
  const boards: NewsIndexBoard[] = [];
  if (KOSPI_TERMS.some((term) => text.includes(term))) {
    boards.push("kospi");
  }
  if (KOSDAQ_TERMS.some((term) => text.includes(term))) {
    boards.push("kosdaq");
  }
  return boards;
}

function matchesStockFilter(
  article: Pick<NewsArticle, "headline" | "summary" | "symbols">,
  filter: NewsStockFilter["id"],
): boolean {
  const stock = STOCK_FILTER_BY_ID[filter];
  if (!stock) {
    return false;
  }
  if (article.symbols?.some((sym) => sym === filter)) {
    return true;
  }
  const text = articleText(article);
  return stock.terms.some((term) => text.includes(term.toLowerCase()));
}

export function isStockIndexFilter(filter: NewsIndexFilter): filter is NewsStockFilter["id"] {
  return filter in STOCK_FILTER_BY_ID;
}

export function matchesNewsIndexFilter(
  article: Pick<NewsArticle, "headline" | "summary" | "tags" | "publishedAt" | "symbols">,
  filter: NewsIndexFilter,
): boolean {
  if (filter === "all") {
    return true;
  }
  if (filter === "breaking") {
    return isBreakingNews(article);
  }
  if (filter === "kospi" || filter === "kosdaq") {
    return newsIndexBoards(article).includes(filter);
  }
  return matchesStockFilter(article, filter);
}

export const NEWS_INDEX_FILTER_LABELS: Record<NewsIndexFilter, string> = {
  all: "전체",
  breaking: "속보",
  kospi: "코스피",
  kosdaq: "코스닥",
  "005930": "삼성전자",
  "000660": "SK하이닉스",
  "005380": "현대차",
};

export function filterLabel(filter: NewsIndexFilter): string {
  return NEWS_INDEX_FILTER_LABELS[filter];
}

export function findLatestBreakingNews(articles: NewsArticle[]): NewsArticle | null {
  for (const article of articles) {
    if (isBreakingNews(article)) {
      return article;
    }
  }
  return null;
}

export function findLatestForFilter(
  articles: NewsArticle[],
  filter: NewsIndexFilter,
): NewsArticle | null {
  if (filter === "all") {
    return articles[0] ?? null;
  }
  for (const article of articles) {
    if (matchesNewsIndexFilter(article, filter)) {
      return article;
    }
  }
  return articles[0] ?? null;
}

export function filterArticlesByIndex(
  articles: NewsArticle[],
  filter: NewsIndexFilter,
): NewsArticle[] {
  return articles.filter((article) => matchesNewsIndexFilter(article, filter));
}
