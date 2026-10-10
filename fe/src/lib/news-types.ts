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

/** 국내 뉴스 카테고리 칩(2026-10-09 결정 A — 6칩). `""`은 전체. 해외에는 칩이 없다. */
export const NEWS_TOPIC_CHIPS: { id: "" | NewsTopic; label: string }[] = [
  { id: "", label: "전체" },
  { id: "stock", label: "주식" },
  { id: "real_estate", label: "부동산" },
  { id: "rates", label: "금리" },
  { id: "deposit", label: "예·적금" },
  { id: "loan", label: "대출" },
];

export function newsTopicLabel(topic?: NewsTopic): string | null {
  if (!topic || topic === "stock") return null;
  return NEWS_TOPIC_CHIPS.find((c) => c.id === topic)?.label ?? null;
}

export type NewsListFilter = {
  market: NewsMarket;
  sourceId?: NewsSourceId | "all";
  sort: NewsSort;
  query?: string;
  symbol?: string;
  /** 국내만. 비우면 전체. */
  topic?: NewsTopic | "";
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
