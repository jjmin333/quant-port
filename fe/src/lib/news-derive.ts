import type { NewsArticle } from "@/lib/news-types";
import { isArticleBreakingNews } from "@/lib/news-breaking";

/**
 * 뉴스 화면의 파생 집계.
 *
 * 전부 **이미 받아 둔 기사 목록에서** 계산한다. 같은 화면의 숫자를 서버에 다시 물으면
 * 집계와 목록이 어긋날 수 있고(기사가 그 사이에 들어온다), 요청 하나를 아끼는 것보다
 * 그게 더 나쁘다 — 우측 레일의 「오늘의 키워드」가 이미 쓰던 규칙이다.
 */

/* ── A1: 오늘의 이슈 종목 ─────────────────────────────────────────────────── */

export type SymbolMention = {
  symbol: string;
  /** 이 종목이 언급된 기사 수 */
  count: number;
  /** 그중 속보가 하나라도 있었나 */
  hasBreaking: boolean;
};

/**
 * 언급 상위 종목. 한 기사가 같은 종목을 여러 번 달아도 1로 센다.
 *
 * 종목 태그는 BE가 제목·요약에서 뽑는다. 국내 6자리 코드와 해외 티커가 한 목록에
 * 섞여 올 수 있어(국내 기사가 미국 종목을 언급하는 일이 흔하다) 시장으로 거르지 않는다.
 */
export function topSymbolMentions(articles: NewsArticle[], limit = 6): SymbolMention[] {
  const counts = new Map<string, SymbolMention>();
  for (const article of articles) {
    const breaking = isArticleBreakingNews(article);
    for (const raw of new Set(article.symbols ?? [])) {
      const symbol = raw.trim().toUpperCase();
      if (!symbol) continue;
      const cur = counts.get(symbol);
      if (cur) {
        cur.count += 1;
        cur.hasBreaking = cur.hasBreaking || breaking;
      } else {
        counts.set(symbol, { symbol, count: 1, hasBreaking: breaking });
      }
    }
  }
  return [...counts.values()]
    .sort((a, b) => b.count - a.count || a.symbol.localeCompare(b.symbol))
    .slice(0, limit);
}

/** 화면에 있는 기사에 붙은 모든 심볼 — 시세를 한 번에 받아오기 위한 목록. */
export function allArticleSymbols(articles: NewsArticle[], cap = 60): string[] {
  const seen = new Set<string>();
  for (const article of articles) {
    for (const raw of article.symbols ?? []) {
      const symbol = raw.trim().toUpperCase();
      if (symbol) seen.add(symbol);
      if (seen.size >= cap) return [...seen];
    }
  }
  return [...seen];
}

/* ── A2: 뉴스 흐름 ────────────────────────────────────────────────────────── */

export type NewsFlowBucket = {
  /** 이 칸이 덮는 시간대의 시작 */
  start: Date;
  count: number;
  breaking: number;
};

export const NEWS_FLOW_HOURS = 24;

/**
 * 최근 24시간을 1시간 칸으로 나눈 기사량.
 *
 * 마지막 칸은 "지금이 속한 시간"이라 아직 채워지는 중이다 — 낮게 나오는 게 정상이고,
 * 그래서 화면에서 마지막 칸만 색을 달리해 진행 중임을 표시한다.
 */
export function buildNewsFlow(articles: NewsArticle[], now = new Date()): NewsFlowBucket[] {
  const end = new Date(now);
  end.setMinutes(0, 0, 0);

  const buckets: NewsFlowBucket[] = [];
  const indexByTime = new Map<number, number>();
  for (let i = NEWS_FLOW_HOURS - 1; i >= 0; i -= 1) {
    const start = new Date(end.getTime() - i * 3600_000);
    indexByTime.set(start.getTime(), buckets.length);
    buckets.push({ start, count: 0, breaking: 0 });
  }

  const oldest = buckets[0].start.getTime();
  for (const article of articles) {
    const at = new Date(article.publishedAt).getTime();
    if (!Number.isFinite(at) || at < oldest) continue;
    const slot = new Date(at);
    slot.setMinutes(0, 0, 0);
    const idx = indexByTime.get(slot.getTime());
    if (idx === undefined) continue;
    buckets[idx].count += 1;
    if (isArticleBreakingNews(article)) buckets[idx].breaking += 1;
  }
  return buckets;
}

/** A2의 막대를 눌렀을 때 남길 기사. */
export function articlesInHour(articles: NewsArticle[], start: Date): NewsArticle[] {
  const from = start.getTime();
  const to = from + 3600_000;
  return articles.filter((article) => {
    const at = new Date(article.publishedAt).getTime();
    return Number.isFinite(at) && at >= from && at < to;
  });
}

/* ── C2: 시간 구분 ────────────────────────────────────────────────────────── */

export type NewsTimeGroup = "now" | "today-am" | "today-pm" | "yesterday" | "older";

const TIME_GROUP_LABELS: Record<NewsTimeGroup, string> = {
  now: "지금 · 1시간 내",
  "today-am": "오늘 오전",
  "today-pm": "오늘 오후",
  yesterday: "어제",
  older: "그 이전",
};

export function newsTimeGroupLabel(group: NewsTimeGroup): string {
  return TIME_GROUP_LABELS[group];
}

/**
 * 기사 하나가 속하는 시간 묶음.
 *
 * 「오전/오후」를 나누는 이유는 장이 그렇게 나뉘기 때문이다 — 개장 직후 쏟아진 기사와
 * 마감 뒤 정리 기사는 성격이 다르고, 168건을 한 덩어리로 흘리면 어디까지 봤는지를 잃는다.
 */
export function newsTimeGroup(article: NewsArticle, now = new Date()): NewsTimeGroup {
  const at = new Date(article.publishedAt);
  if (Number.isNaN(at.getTime())) return "older";

  if (now.getTime() - at.getTime() < 3600_000) return "now";

  const sameDay =
    at.getFullYear() === now.getFullYear() &&
    at.getMonth() === now.getMonth() &&
    at.getDate() === now.getDate();
  if (sameDay) return at.getHours() < 12 ? "today-am" : "today-pm";

  const yesterday = new Date(now);
  yesterday.setDate(yesterday.getDate() - 1);
  const isYesterday =
    at.getFullYear() === yesterday.getFullYear() &&
    at.getMonth() === yesterday.getMonth() &&
    at.getDate() === yesterday.getDate();
  return isYesterday ? "yesterday" : "older";
}

/* ── E3: 출처 분포 ────────────────────────────────────────────────────────── */

export type PublisherCount = { id: string; name: string; count: number };

export function publisherBreakdown(articles: NewsArticle[], limit = 6): PublisherCount[] {
  const counts = new Map<string, PublisherCount>();
  for (const article of articles) {
    const id = article.publisher.id;
    const cur = counts.get(id);
    if (cur) cur.count += 1;
    else counts.set(id, { id, name: article.publisher.name, count: 1 });
  }
  return [...counts.values()].sort((a, b) => b.count - a.count).slice(0, limit);
}

/* ── D2: 같은 이슈 묶기 ───────────────────────────────────────────────────── */

/**
 * 제목을 비교용으로 정규화 — 매체마다 다는 머리표([속보]·[단독])와 기호를 걷어낸다.
 */
function titleTokens(text: string): Set<string> {
  const cleaned = text
    .replace(/\[[^\]]*\]/g, " ")
    .replace(/【[^】]*】/g, " ")
    .replace(/[^\p{L}\p{N}\s]/gu, " ")
    .toLowerCase();
  return new Set(cleaned.split(/\s+/).filter((token) => token.length >= 2));
}

function jaccard(a: Set<string>, b: Set<string>): number {
  if (a.size === 0 || b.size === 0) return 0;
  let shared = 0;
  for (const token of a) if (b.has(token)) shared += 1;
  return shared / (a.size + b.size - shared);
}

/**
 * 한 기사와 **같은 사건을 다룬 듯한** 다른 기사들.
 *
 * 제목 토큰의 자카드 유사도만 본다 — 본문을 받아오지 않으므로 추가 요청이 없다.
 * 정확도가 완벽하지 않아 화면에서는 「관련 있어 보이는 기사」로 적는다: 확신을 표시하면
 * 틀렸을 때 더 나쁘다.
 */
export function relatedArticles(
  article: NewsArticle,
  pool: NewsArticle[],
  {
    limit = 5,
    threshold = 0.34,
    windowMs = 12 * 3600_000,
  }: { limit?: number; threshold?: number; windowMs?: number } = {},
): NewsArticle[] {
  const base = titleTokens(article.headlineKo || article.headline);
  if (base.size < 3) return [];
  const baseAt = new Date(article.publishedAt).getTime();

  const scored: Array<{ article: NewsArticle; score: number }> = [];
  for (const other of pool) {
    if (other.id === article.id) continue;
    // 같은 사건을 다룬 기사는 시간적으로도 붙어 있다. 이 창이 없으면 "업비트, X
    // 입출금 중단"과 3일 전 "빗썸, Y 입출금 중단"처럼 **문장 틀만 같은** 기사가
    // 묶인다 — 제목 유사도만으로는 구분되지 않는 종류의 오탐이다.
    const otherAt = new Date(other.publishedAt).getTime();
    if (Number.isFinite(baseAt) && Number.isFinite(otherAt)) {
      if (Math.abs(otherAt - baseAt) > windowMs) continue;
    }
    const score = jaccard(base, titleTokens(other.headlineKo || other.headline));
    if (score >= threshold) scored.push({ article: other, score });
  }
  return scored
    .sort((a, b) => b.score - a.score)
    .slice(0, limit)
    .map((entry) => entry.article);
}
