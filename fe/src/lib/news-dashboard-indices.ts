import type { NewsArticle } from "@/lib/news-types";
import type { NewsIndexFilter } from "@/lib/news-index-filter";
import type { NewsUSIndexFilter } from "@/lib/news-us-index-filter";

/** Dashboard index IDs — aligned with market-overview GROUPS and index modal (future). */
export const DASHBOARD_INDEX_IDS = [
  "kospi",
  "kosdaq",
  "usd-krw",
  "nasdaq",
  "nasdaq100",
  "nq100",
  "sp500",
  "dow",
  "vix",
  "us10y",
  "sox",
  "btc",
  "gold",
] as const;

export type DashboardIndexId = (typeof DASHBOARD_INDEX_IDS)[number];

const DASHBOARD_INDEX_ID_SET = new Set<string>(DASHBOARD_INDEX_IDS);

export const DASHBOARD_INDEX_TAG_PREFIX = "index:";

/** Text rules aligned with BE `dashboardIndexRules` (news_index_match.go). */
const DASHBOARD_INDEX_TERMS: Partial<Record<DashboardIndexId, readonly string[]>> = {
  // 「서울 증시」·「Seoul stocks」도 코스피를 부르는 말이다(BE 규칙과 함께 고칠 것).
  kospi: ["코스피", "kospi", "서울 증시", "서울증시", "seoul stocks", "seoul shares"],
  kosdaq: ["코스닥", "kosdaq"],
  "usd-krw": ["원달러", "usd/krw", "usd-krw", "usdkrw", "원/달러", "달러-원", "달러환율", "원·달러"],
  sp500: ["s&p 500", "s&p500", "sp500", "sp 500", "s&p"],
  nasdaq: ["nasdaq", "나스닥"],
  nasdaq100: ["nasdaq 100", "nasdaq100", "nq100", "나스닥100", "나스닥 100"],
  nq100: ["nasdaq 100", "nasdaq100", "nq100", "나스닥100", "나스닥 100"],
  dow: ["dow jones", "dow", "djia", "다우"],
  vix: ["vix", "변동성 지수", "volatility index", "fear gauge"],
  us10y: [
    "미 국채 10년",
    "미국채 10년",
    "미 10년물",
    "10-year treasury",
    "10 year treasury",
    "10-year yield",
    "treasury yield",
    "dgs10",
  ],
  sox: ["sox", "phlx semiconductor", "반도체 지수", "chip index"],
  btc: [
    "비트코인",
    "bitcoin",
    "btc",
    "이더리움",
    "ethereum",
    "eth",
    "가상자산",
    "암호화폐",
    "cryptocurrency",
    "crypto market",
    "블록체인",
    "blockchain",
  ],
  gold: ["금값", "금 시세", "gold price", "gold spot", "spot gold", "국제금", "금값이", "금값은", "gold futures", "gold etf", "gold market", "gold rally", "gold falls", "gold rises", "국제 금", "금 가격", "금 시장"],
};

function matchesGoldIndex(text: string): boolean {
  const terms = DASHBOARD_INDEX_TERMS.gold ?? [];
  if (terms.some((term) => text.includes(term.toLowerCase()))) {
    return true;
  }
  let idx = 0;
  while (idx < text.length) {
    const pos = text.indexOf("gold", idx);
    if (pos < 0) break;
    const after = pos + 4;
    if (text.startsWith("man", after)) {
      idx = after;
      continue;
    }
    const before = pos === 0 || !/[a-z0-9]/i.test(text[pos - 1] ?? "");
    const afterOk = after >= text.length || !/[a-z0-9]/i.test(text[after] ?? "");
    if (before && afterOk) return true;
    idx = after;
  }
  return (
    /\b금\b/.test(text) &&
    (text.includes("국제") || text.includes("금값") || text.includes("금 시세") || text.includes("금 가격"))
  );
}

const DASHBOARD_INDEX_LABELS: Record<DashboardIndexId, string> = {
  kospi: "코스피",
  kosdaq: "코스닥",
  "usd-krw": "원/달러",
  nasdaq: "나스닥",
  nasdaq100: "나스닥100",
  nq100: "나스닥100",
  sp500: "S&P 500",
  dow: "다우",
  vix: "VIX",
  us10y: "미 국채 10년",
  sox: "반도체 지수",
  btc: "비트코인",
  gold: "금",
};

export function dashboardIndexLabel(indexId: DashboardIndexId): string {
  return DASHBOARD_INDEX_LABELS[indexId] ?? indexId;
}

/**
 * 판정에 쓰는 본문 — **번역문까지 포함한다.**
 *
 * 해외 기사는 화면에 한국어 번역이 뜨는데 매칭은 영어 원문만 보고 있었다. "Seoul's
 * best day ever"라고 쓴 기사의 번역문이 「서울 증시가 역대 최고일을 기록」인데도
 * 코스피가 안 붙던 이유다 — 사용자가 읽는 글자와 규칙이 보는 글자가 달랐다.
 */
function articleText(article: NewsArticleTextFields): string {
  return [article.headline, article.headlineKo, article.summary, article.summaryKo]
    .filter(Boolean)
    .join(" ")
    .toLowerCase();
}

type NewsArticleTextFields = Pick<
  NewsArticle,
  "headline" | "summary" | "tags" | "headlineKo" | "summaryKo"
>;

/** Extract dashboard index IDs from ingest tags (`index:kospi`, …). */
export function indexIdsFromTags(tags: readonly string[] | undefined): DashboardIndexId[] {
  if (!tags?.length) {
    return [];
  }
  const out: DashboardIndexId[] = [];
  for (const tag of tags) {
    if (!tag.startsWith(DASHBOARD_INDEX_TAG_PREFIX)) {
      continue;
    }
    const id = tag.slice(DASHBOARD_INDEX_TAG_PREFIX.length);
    if (DASHBOARD_INDEX_ID_SET.has(id)) {
      out.push(id as DashboardIndexId);
    }
  }
  return out;
}

export function isDashboardIndexTag(tag: string): boolean {
  if (!tag.startsWith(DASHBOARD_INDEX_TAG_PREFIX)) {
    return false;
  }
  return DASHBOARD_INDEX_ID_SET.has(tag.slice(DASHBOARD_INDEX_TAG_PREFIX.length));
}

export function articleMatchesDashboardIndex(
  article: Pick<NewsArticle, "headline" | "summary" | "tags" | "headlineKo" | "summaryKo">,
  indexId: DashboardIndexId,
): boolean {
  const tag = `${DASHBOARD_INDEX_TAG_PREFIX}${indexId}`;
  if (article.tags?.includes(tag)) {
    return true;
  }
  const terms = DASHBOARD_INDEX_TERMS[indexId];
  const text = articleText(article);
  if (indexId === "gold") {
    return matchesGoldIndex(text);
  }
  if (!terms?.length) {
    return false;
  }
  return terms.some((term) => text.includes(term.toLowerCase()));
}

/**
 * `nq100`은 `nasdaq100`과 **같은 말**을 매칭한다(둘 다 "나스닥100"). 기사에서 지수를
 * 뽑을 때 둘 다 통과시키면 같은 지수 카드가 두 장 뜬다 — 목록에서는 하나만 본다.
 * (지수 시세 응답에서는 `nq100`이 선물이라 따로 존재하므로 카탈로그 자체는 못 줄인다.)
 */
const ARTICLE_INDEX_IDS: readonly DashboardIndexId[] = DASHBOARD_INDEX_IDS.filter(
  (id) => id !== "nq100",
);

/**
 * 기사 하나에 걸린 지수들.
 *
 * 기사 모달이 종목 카드 옆에 지수 카드를 세우는 데 쓴다 — "아마존 상승, 애플 하락,
 * 서울 증시 사상 최고" 같은 제목은 종목만큼이나 지수 이야기인데, 지금까지 모달은
 * 종목만 보여주고 코스피는 어디에도 없었다.
 *
 * 판정 규칙은 목록 필터(`articleMatchesDashboardIndex`)와 **같은 것**을 쓴다. 카드에
 * 뜬 지수와 그 지수로 좁힌 목록이 어긋나면 둘 중 하나는 거짓말이 된다.
 */
export function articleDashboardIndexIds(
  article: Pick<NewsArticle, "headline" | "summary" | "tags" | "headlineKo" | "summaryKo">,
  limit = 3,
): DashboardIndexId[] {
  const out: DashboardIndexId[] = [];
  for (const id of ARTICLE_INDEX_IDS) {
    if (out.length >= limit) break;
    if (articleMatchesDashboardIndex(article, id)) out.push(id);
  }
  return out;
}

export function countArticlesByDashboardIndex(
  articles: NewsArticle[],
  indexId: DashboardIndexId,
): number {
  let count = 0;
  for (const article of articles) {
    if (articleMatchesDashboardIndex(article, indexId)) {
      count += 1;
    }
  }
  return count;
}

/** Pulse strip index card → pill filter when the market has a matching tab. */
export function dashboardIndexToPillFilter(
  market: "domestic" | "us",
  indexId: string,
): NewsIndexFilter | NewsUSIndexFilter | null {
  if (market === "domestic") {
    if (indexId === "kospi" || indexId === "kosdaq") {
      return indexId;
    }
    return null;
  }
  if (indexId === "sp500" || indexId === "nasdaq" || indexId === "dow") {
    return indexId;
  }
  return null;
}

export function pillFilterToDashboardIndex(
  market: "domestic" | "us",
  filter: NewsIndexFilter | NewsUSIndexFilter,
): DashboardIndexId | null {
  if (market === "domestic" && (filter === "kospi" || filter === "kosdaq")) {
    return filter;
  }
  if (market === "us" && (filter === "sp500" || filter === "nasdaq" || filter === "dow")) {
    return filter;
  }
  return null;
}
