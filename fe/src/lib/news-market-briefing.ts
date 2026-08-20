import type { NewsArticle, NewsMarket } from "@/lib/news-types";
import { newsDisplayHeadline, newsDisplaySummary } from "@/lib/news-display";
import {
  filterArticlesByIndex,
  filterLabel,
  NEWS_INDEX_FILTER_LABELS,
  type NewsIndexFilter,
} from "@/lib/news-index-filter";
import {
  filterArticlesByUSIndex,
  type NewsUSIndexFilter,
  sectionTitleForUSBoard,
  usFilterLabel,
} from "@/lib/news-us-index-filter";

export type NewsMarketBriefing = {
  headline: string;
  summary: string;
  highlights: string[];
  board: NewsIndexFilter | NewsUSIndexFilter;
  generatedAt: string;
  source: "rules";
};

function briefingPool(
  articles: NewsArticle[],
  board: NewsIndexFilter | NewsUSIndexFilter,
  market: NewsMarket,
): NewsArticle[] {
  if (board === "all") {
    return articles;
  }
  const matched =
    market === "us"
      ? filterArticlesByUSIndex(articles, board as NewsUSIndexFilter)
      : filterArticlesByIndex(articles, board as NewsIndexFilter);
  return matched.length > 0 ? matched : articles;
}

function trimSentence(text: string, maxLen: number): string {
  const t = text.replace(/\s+/g, " ").trim();
  if (t.length <= maxLen) {
    return t;
  }
  return `${t.slice(0, maxLen - 1).trim()}…`;
}

/** Rule-based market briefing from ingested headlines (no LLM). */
export function buildNewsMarketBriefing(
  articles: NewsArticle[],
  board: NewsIndexFilter | NewsUSIndexFilter,
  market: NewsMarket = "domestic",
): NewsMarketBriefing | null {
  if (articles.length === 0) {
    return null;
  }

  const pool = briefingPool(articles, board, market);
  const top = pool.slice(0, 6);
  const lead = top[0];
  if (!lead) {
    return null;
  }

  const snippets = top
    .map((a) => newsDisplaySummary(a)?.trim() || newsDisplayHeadline(a).trim())
    .filter(Boolean)
    .slice(0, 4);

  const summary =
    snippets.length >= 2
      ? trimSentence(snippets.join(" "), 420)
      : trimSentence(newsDisplaySummary(lead)?.trim() || newsDisplayHeadline(lead), 420);

  return {
    headline: newsDisplayHeadline(lead),
    summary,
    highlights: top.slice(0, 4).map((a) => newsDisplayHeadline(a)),
    board,
    generatedAt: new Date().toISOString(),
    source: "rules",
  };
}

export function briefingBoardLabel(
  board: NewsIndexFilter | NewsUSIndexFilter,
  market: NewsMarket = "domestic",
): string {
  if (market === "us") {
    return usFilterLabel(board as NewsUSIndexFilter);
  }
  return filterLabel(board as NewsIndexFilter);
}

export function sectionTitleForBoard(
  board: NewsIndexFilter | NewsUSIndexFilter,
  market: NewsMarket = "domestic",
): string {
  if (market === "us") {
    return sectionTitleForUSBoard(board as NewsUSIndexFilter);
  }
  if (board === "all") {
    return "국내 뉴스";
  }
  return `${NEWS_INDEX_FILTER_LABELS[board as NewsIndexFilter]} 뉴스`;
}

export function formatBriefingTimestamp(iso: string): string {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) {
    return iso;
  }
  const date = d.toLocaleDateString("ko-KR", {
    timeZone: "Asia/Seoul",
    year: "numeric",
    month: "numeric",
    day: "numeric",
  });
  const weekday = d.toLocaleDateString("ko-KR", { timeZone: "Asia/Seoul", weekday: "short" });
  const time = d.toLocaleTimeString("ko-KR", {
    timeZone: "Asia/Seoul",
    hour: "2-digit",
    minute: "2-digit",
    hour12: false,
  });
  return `${date} (${weekday}) ${time}`;
}
