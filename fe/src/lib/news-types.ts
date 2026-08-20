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
};

export type NewsListFilter = {
  market: NewsMarket;
  sourceId?: NewsSourceId | "all";
  sort: NewsSort;
  query?: string;
  symbol?: string;
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
