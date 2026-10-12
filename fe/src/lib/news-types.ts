import type { NewsPublisher } from "@/lib/news-publishers";

export type NewsMarket = "domestic" | "us";

export type NewsSourceId =
  | "naver"
  | "kakao"
  | "rss"
  | "finnhub"
  | "financial-news"
  | "yonhap"
  | "save-pick"
  | "reuters"
  | "financial-juice"
  | "bloomberg"
  | "google_news"
  | "tokenpost"
  | "coinreaders"
  | "blockmedia";

/** Ingest channel (Naver API, RSS poller, etc.) */
export type NewsSource = {
  id: NewsSourceId;
  name: string;
  market: NewsMarket;
  badgeClass: string;
};

export type NewsSort = "latest" | "popular";

export type NewsEmbedPolicy = "ok" | "blocked" | "unknown";

export type NewsPreviewMode = "reader" | "iframe" | "summary";

export type NewsArticle = {
  id: string;
  market: NewsMarket;
  headline: string;
  headlineKo?: string;
  summary?: string;
  summaryKo?: string;
  /** Original media outlet */
  publisher: NewsPublisher;
  /** How quant-port collected the article */
  source: NewsSource;
  tags: string[];
  symbols?: string[];
  keywords?: string[];
  publishedAt: string;
  viewCount: number;
  /** Canonical article URL (original publisher) */
  url?: string;
  originalUrl?: string;
  portalUrl?: string;
  imageUrl?: string;
  previewMode?: NewsPreviewMode;
  /** Whether iframe preview is likely to work */
  embedPolicy?: NewsEmbedPolicy;
  /** Open via Reader API instead of iframe */
  readerMode?: boolean;
  highlighted?: boolean;
  /** 국내 카테고리(2026-10-09). 없으면 주식. */
  topic?: NewsTopic;
  /** 제목에서 뽑은 자산 칩(지역·금리) — 누르면 자산 미리보기 모달. */
  assetTags?: NewsAssetTag[];
};

export type NewsTopic = "stock" | "real_estate" | "rates" | "deposit" | "loan";

export type NewsAssetTag = { kind: "region" | "sido" | "rate" | "apt" | "product"; key: string; name: string };

/**
 * 목록 필터 값 — 기사 topic 하나 또는 묶음 `finance`(금리·예적금·대출, 2026-10-12 사용자 요청).
 * 묶음은 BE가 `topic IN (rates, deposit, loan)`으로 푼다(`model.NewsTopicFilterMembers`).
 */
export type NewsTopicFilter = "" | NewsTopic | "finance";

/** 국내 뉴스 카테고리 칩(2026-10-09 결정 A, 10-12에 금리·예적금·대출을 한 칩으로). `""`은 전체. 해외에는 칩이 없다. */
export const NEWS_TOPIC_CHIPS: { id: NewsTopicFilter; label: string }[] = [
  { id: "", label: "전체" },
  { id: "stock", label: "주식" },
  { id: "real_estate", label: "부동산" },
  { id: "finance", label: "금리·예금·대출" },
];

/** 옛 칩 주소(`?topic=rates` 등)는 묶음 칩으로. 모르는 값은 null. */
export function parseNewsTopicFilter(t: string | null): NewsTopicFilter | null {
  if (t === "rates" || t === "deposit" || t === "loan") return "finance";
  return NEWS_TOPIC_CHIPS.some((c) => c.id === t) ? (t as NewsTopicFilter) : null;
}

const NEWS_TOPIC_LABELS: Record<NewsTopic, string | null> = {
  stock: null,
  real_estate: "부동산",
  rates: "금리",
  deposit: "예·적금",
  loan: "대출",
};

/** 기사 카드의 주제 배지 — 기사 topic은 셋으로 남아 있어 배지는 그대로 「금리」·「대출」처럼 나뉜다. */
export function newsTopicLabel(topic?: NewsTopic): string | null {
  return topic ? NEWS_TOPIC_LABELS[topic] ?? null : null;
}

export type NewsListFilter = {
  market: NewsMarket;
  sourceId?: NewsSourceId | "all";
  sort: NewsSort;
  query?: string;
  symbol?: string;
  /** 국내만. 비우면 전체. */
  topic?: NewsTopicFilter;
};

export type NewsReaderContent = {
  url: string;
  title: string;
  originalTitle?: string;
  byline?: string;
  excerpt?: string;
  originalExcerpt?: string;
  siteName?: string;
  imageUrl?: string;
  textContent: string;
  contentHtml?: string;
  publishedAt?: string;
  fetchedAt: string;
  translated?: boolean;
};

/** User-facing link — original publisher URL preferred. */
export function newsArticleLink(article: Pick<NewsArticle, "url" | "originalUrl">): string | undefined {
  const link = article.originalUrl?.trim() || article.url?.trim();
  return link && link !== "#" ? link : undefined;
}

export function newsUsesReader(article: NewsArticle): boolean {
  return Boolean(article.readerMode || article.previewMode === "reader");
}
