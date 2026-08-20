import type { NewsArticle } from "@/lib/news-types";
import { isBreakingNews } from "@/lib/news-index-filter";

export type NewsUSIndexFilter =
  | "all"
  | "breaking"
  | "macro"
  | "sp500"
  | "nasdaq"
  | "dow"
  | "NVDA"
  | "AAPL"
  | "MSFT";

export type NewsUSBoard = "sp500" | "nasdaq" | "dow";

export type NewsUSStockFilter = {
  id: Extract<NewsUSIndexFilter, "NVDA" | "AAPL" | "MSFT">;
  label: string;
  terms: readonly string[];
};

export const NEWS_US_STOCK_FILTERS: NewsUSStockFilter[] = [
  { id: "NVDA", label: "NVDA", terms: ["nvda", "nvidia", "엔비디아"] },
  { id: "AAPL", label: "Apple", terms: ["aapl", "apple", "애플"] },
  { id: "MSFT", label: "Microsoft", terms: ["msft", "microsoft", "마이크로소프트"] },
];

export const NEWS_US_INDEX_PILLS: NewsUSIndexFilter[] = [
  "all",
  "breaking",
  "macro",
  "sp500",
  "nasdaq",
  "dow",
  "NVDA",
  "AAPL",
  "MSFT",
];

const MACRO_TERMS = [
  "fed",
  "federal reserve",
  "inflation",
  "cpi",
  "treasury",
  "rate cut",
  "rate hike",
  "jobs report",
  "tariff",
  "trade war",
  "sanctions",
  "opec",
  "oil price",
  "yield",
  "central bank",
  "monetary policy",
  "commerce ministry",
  "finance ministry",
] as const;

const BREAKING_MAX_AGE_MS = 24 * 60 * 60 * 1000;

const SP500_TERMS = ["s&p 500", "s&p500", "sp500", "sp 500", "s&p"] as const;
const NASDAQ_TERMS = ["nasdaq", "나스닥", "nasdaq 100", "nasdaq100"] as const;
const DOW_TERMS = ["dow jones", "dow", "다우", "djia"] as const;

const STOCK_FILTER_BY_ID = Object.fromEntries(
  NEWS_US_STOCK_FILTERS.map((stock) => [stock.id, stock]),
) as Record<NewsUSStockFilter["id"], NewsUSStockFilter>;

function articleText(article: Pick<NewsArticle, "headline" | "summary">): string {
  return `${article.headline} ${article.summary ?? ""}`.toLowerCase();
}

export function isUSBreakingNews(
  article: Pick<NewsArticle, "headline" | "tags" | "publishedAt" | "source">,
  nowMs: number = Date.now(),
): boolean {
  if (article.source?.id === "financial-juice") {
    const published = new Date(article.publishedAt).getTime();
    if (!Number.isNaN(published) && nowMs - published <= BREAKING_MAX_AGE_MS) {
      return true;
    }
  }
  if (isBreakingNews(article, nowMs)) {
    return true;
  }
  const published = new Date(article.publishedAt).getTime();
  if (Number.isNaN(published) || nowMs - published > BREAKING_MAX_AGE_MS) {
    return false;
  }
  const headline = article.headline.trim().toLowerCase();
  return (
    headline.startsWith("breaking:") ||
    headline.startsWith("breaking ") ||
    headline.includes("[breaking]") ||
    headline.includes("breaking news")
  );
}

export function newsUSBoards(
  article: Pick<NewsArticle, "headline" | "summary">,
): NewsUSBoard[] {
  const text = articleText(article);
  const boards: NewsUSBoard[] = [];
  if (SP500_TERMS.some((term) => text.includes(term))) {
    boards.push("sp500");
  }
  if (NASDAQ_TERMS.some((term) => text.includes(term))) {
    boards.push("nasdaq");
  }
  if (DOW_TERMS.some((term) => text.includes(term))) {
    boards.push("dow");
  }
  return boards;
}

function matchesUSStockFilter(
  article: Pick<NewsArticle, "headline" | "summary" | "symbols">,
  filter: NewsUSStockFilter["id"],
): boolean {
  const stock = STOCK_FILTER_BY_ID[filter];
  if (!stock) {
    return false;
  }
  if (article.symbols?.some((sym) => sym.toUpperCase() === filter)) {
    return true;
  }
  const text = articleText(article);
  return stock.terms.some((term) => text.includes(term.toLowerCase()));
}

export function matchesNewsUSIndexFilter(
  // "source" is required by isUSBreakingNews, which this delegates to.
  article: Pick<NewsArticle, "headline" | "summary" | "tags" | "publishedAt" | "symbols" | "source">,
  filter: NewsUSIndexFilter,
): boolean {
  if (filter === "all") {
    return true;
  }
  if (filter === "breaking") {
    return isUSBreakingNews(article);
  }
  if (filter === "macro") {
    const text = articleText(article);
    return MACRO_TERMS.some((term) => text.includes(term));
  }
  if (filter === "sp500" || filter === "nasdaq" || filter === "dow") {
    return newsUSBoards(article).includes(filter);
  }
  return matchesUSStockFilter(article, filter);
}

export const NEWS_US_INDEX_FILTER_LABELS: Record<NewsUSIndexFilter, string> = {
  all: "전체",
  breaking: "속보",
  macro: "매크로",
  sp500: "S&P 500",
  nasdaq: "나스닥",
  dow: "다우",
  NVDA: "NVDA",
  AAPL: "Apple",
  MSFT: "Microsoft",
};

export function usFilterLabel(filter: NewsUSIndexFilter): string {
  return NEWS_US_INDEX_FILTER_LABELS[filter];
}

export function findLatestForUSFilter(
  articles: NewsArticle[],
  filter: NewsUSIndexFilter,
): NewsArticle | null {
  if (filter === "all") {
    return articles[0] ?? null;
  }
  for (const article of articles) {
    if (matchesNewsUSIndexFilter(article, filter)) {
      return article;
    }
  }
  return articles[0] ?? null;
}

export function filterArticlesByUSIndex(
  articles: NewsArticle[],
  filter: NewsUSIndexFilter,
): NewsArticle[] {
  return articles.filter((article) => matchesNewsUSIndexFilter(article, filter));
}

export function sectionTitleForUSBoard(board: NewsUSIndexFilter): string {
  if (board === "all") {
    return "해외 뉴스";
  }
  return `${NEWS_US_INDEX_FILTER_LABELS[board]} 뉴스`;
}
