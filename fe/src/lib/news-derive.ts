import type { NewsArticle } from "@/lib/news-types";

/**
 * 뉴스 화면의 파생 집계.
 *
 * 여기 남은 것은 **화면에 떠 있는 기사만 보면 되는** 계산이다 — 시세를 받을 심볼
 * 목록, 시간 구분선, 같은 이슈 묶기.
 *
 * 「오늘의 키워드」와 「많이 언급된 종목」은 여기 있다가 서버로 갔다. 목록을 세면 그
 * 목록이 덮는 시간만 세게 되는데, 국내 최신 100건이 덮는 시간이 34분이었다(실측
 * 2026-09-01). 고정된 모수가 필요한 집계는 `news-api.ts`의 fetch 두 개가 맡는다.
 */

/**
 * 화면에 있는 기사에 붙은 모든 심볼 — 시세를 한 번에 받아오기 위한 목록.
 *
 * **cap은 서버 상한(`/quotes/batch`의 `limit` = 100)에서 역산한 값이다.** 호출부는
 * 여기에 「내 종목」과 「많이 언급된 종목」을 더해서 보내므로, 90이면 그 둘이
 * 붙어도 서버 상한 안에 든다(상한을 넘기면 서버가 `truncated`로 알려 주지만,
 * 애초에 넘기지 않는 편이 낫다).
 *
 * 60이던 것을 올렸다. 실측(2026-09-01 마감 후) 국내 뉴스 100건의 서로 다른 심볼이
 * 67개라, 60에서 잘린 7개가 **요청되지도 않은 채** 기사 하단에 종목코드로 남아
 * 있었다 — 「시세를 못 받아서」가 아니라 「묻지 않아서」였다. 배치가 캐시·DB만
 * 읽게 된 뒤로는(quotes_batch.go) 심볼을 늘려도 상류 호출이 늘지 않는다.
 */
export function allArticleSymbols(articles: NewsArticle[], cap = 90): string[] {
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
