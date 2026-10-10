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

/** 승계 배지의 「AI 17:00」 — 문장이 쓰인 시각만 뽑는다. */
/**
 * 승계 배지의 시각 — 물려받은 문장이 **다른 날** 것이면 날을 붙인다.
 *
 * 「AI 21:00」은 새벽 1시에 보면 오늘 21시(아직 오지 않은 시각)로 읽힌다. 503이 몰리는
 * 22:00~03:00에 자정을 넘긴 승계가 흔하다. 하루 전이면 「어제 21:00」, 그보다 오래면
 * 「9/24 21:00」. 날은 서울 달력으로 센다.
 */
export function formatCarryClock(carriedFrom: string, generatedAt?: string): string {
  const clock = formatBriefingClock(carriedFrom);
  if (!clock) return "";
  const from = new Date(carriedFrom);
  const at = generatedAt ? new Date(generatedAt) : new Date();
  if (Number.isNaN(at.getTime())) return clock;
  const day = (d: Date) => d.toLocaleDateString("en-CA", { timeZone: "Asia/Seoul" });
  const fromDay = day(from);
  const atDay = day(at);
  if (fromDay === atDay) return clock;
  const diffDays = Math.round(
    (Date.parse(`${atDay}T00:00:00+09:00`) - Date.parse(`${fromDay}T00:00:00+09:00`)) / 86_400_000,
  );
  if (diffDays === 1) return `어제 ${clock}`;
  const [, m, d] = fromDay.split("-");
  return `${Number(m)}/${Number(d)} ${clock}`;
}

export function formatBriefingClock(iso: string): string {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) {
    return "";
  }
  return d.toLocaleTimeString("ko-KR", {
    timeZone: "Asia/Seoul",
    hour: "2-digit",
    minute: "2-digit",
    hour12: false,
  });
}
