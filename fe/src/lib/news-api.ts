import type { ApiResponse } from "@/lib/api-types";
import type {
  NewsArticle,
  NewsListFilter,
  NewsPreviewMode,
  NewsReaderContent,
  NewsSourceId,
} from "@/lib/news-types";
import { NEWS_SOURCES } from "@/lib/mock-news";
import { publisherById, resolvePublisherFromUrl } from "@/lib/news-publishers";
import { parseNewsSearchSymbol } from "@/lib/header-search-config";
import { getApiBaseUrl, mergeFetchInit, parseJson } from "@/lib/api-client";
import { cachedGet } from "@/lib/fetch-cache";

type NewsArticleDTO = {
  id: string;
  market: "domestic" | "us";
  headline: string;
  headlineKo?: string;
  summary?: string;
  summaryKo?: string;
  sourceId: NewsSourceId;
  publisherId?: string;
  publisherName?: string;
  tags: string[];
  symbols?: string[];
  keywords?: string[];
  publishedAt: string;
  viewCount: number;
  url?: string;
  originalUrl?: string;
  portalUrl?: string;
  imageUrl?: string;
  previewMode?: NewsPreviewMode;
  embedPolicy?: "ok" | "blocked" | "unknown";
  readerMode?: boolean;
  highlighted?: boolean;
};

type NewsListResponse = ApiResponse<NewsArticleDTO[]> & {
  meta?: { count?: number; totalCount?: number; updatedAt?: string };
};

type DashboardNewsResponse = ApiResponse<NewsArticleDTO[]> & {
  meta?: { count?: number; fullCount?: number; updatedAt?: string };
};

export type DomesticNewsResult = {
  articles: NewsArticle[];
  totalCount: number;
};

export type UsNewsResult = DomesticNewsResult;

/**
 * 홈 대시보드용 기사 묶음.
 *
 * `articles`는 앞 `fullCount`건만 **온전한 기사**이고, 그 뒤는 제목·태그·키워드·
 * 발행시각만 남긴 요약본이다. 홈이 실제로 그리는 건 여덟 줄인데 「오늘의 키워드」와
 * 속보 개수는 100건 전체를 세야 해서, 세지 못할 만큼 줄일 수도 없고 100건을 통째로
 * 받을 필요도 없다.
 *
 * 그래서 **모달을 여는 대상은 앞 `fullCount`건으로 제한해야 한다** — 뒤쪽 항목에는
 * 본문 링크가 없어 창이 비어 뜬다.
 */
export type DashboardNewsResult = {
  articles: NewsArticle[];
  fullCount: number;
};

function formatPublisherDisplayName(raw: string): string {
  const trimmed = raw.trim();
  if (!trimmed) return "기타";
  // Already Korean
  if (/[\uAC00-\uD7A3]/.test(trimmed)) return trimmed;
  return trimmed;
}

function mapArticle(dto: NewsArticleDTO): NewsArticle {
  const source =
    NEWS_SOURCES.find((s) => s.id === dto.sourceId && s.market === dto.market) ??
    NEWS_SOURCES.find((s) => s.id === dto.sourceId) ??
    NEWS_SOURCES.find((s) => s.id === "rss")!;
  const link = dto.originalUrl?.trim() || dto.url?.trim();
  const publisher = dto.publisherId
    ? publisherById(dto.publisherId, dto.publisherName)
    : resolvePublisherFromUrl(link, dto.publisherName);
  if (dto.publisherName && publisher.id === "unknown") {
    publisher.name = formatPublisherDisplayName(dto.publisherName);
  }
  return {
    id: dto.id,
    market: dto.market,
    headline: dto.headline,
    headlineKo: dto.headlineKo,
    summary: dto.summary,
    summaryKo: dto.summaryKo,
    publisher,
    source,
    tags: dto.tags ?? [],
    symbols: dto.symbols,
    keywords: dto.keywords?.filter(Boolean),
    publishedAt: dto.publishedAt,
    viewCount: dto.viewCount ?? 0,
    url: link,
    originalUrl: dto.originalUrl ?? link,
    portalUrl: dto.portalUrl,
    imageUrl: dto.imageUrl,
    previewMode: dto.previewMode,
    embedPolicy: dto.embedPolicy ?? "unknown",
    readerMode: dto.readerMode,
    highlighted: dto.highlighted,
  };
}

export async function fetchUsNews(filter: NewsListFilter): Promise<UsNewsResult> {
  const params = new URLSearchParams();
  params.set("sort", filter.sort);
  if (filter.sourceId && filter.sourceId !== "all") {
    params.set("source", filter.sourceId);
  }
  if (filter.symbol?.trim()) {
    params.set("symbol", filter.symbol.trim());
  }
  const query = filter.query?.trim();
  if (query) {
    params.set("q", query);
    const symbol = parseNewsSearchSymbol(query, "us");
    if (symbol) {
      params.set("symbol", symbol);
    }
  }

  const res = await fetch(`${getApiBaseUrl()}/api/v1/news/us?${params}`, mergeFetchInit({
    cache: "no-store",
  }));

  if (!res.ok) {
    const text = await res.text();
    throw new Error(text.slice(0, 200) || "Failed to load US news");
  }

  const body = await parseJson<NewsListResponse>(res);
  return {
    articles: (body.data ?? []).map(mapArticle),
    totalCount: body.meta?.totalCount ?? body.data?.length ?? 0,
  };
}

export async function fetchDomesticNews(filter: NewsListFilter): Promise<DomesticNewsResult> {
  const params = new URLSearchParams();
  params.set("sort", filter.sort);
  if (filter.sourceId && filter.sourceId !== "all") {
    params.set("source", filter.sourceId);
  }
  if (filter.symbol?.trim()) {
    params.set("symbol", filter.symbol.trim());
  }
  const query = filter.query?.trim();
  if (query) {
    params.set("q", query);
    const symbol = parseNewsSearchSymbol(query, "domestic");
    if (symbol) {
      params.set("symbol", symbol);
    }
  }

  const res = await fetch(`${getApiBaseUrl()}/api/v1/news/domestic?${params}`, mergeFetchInit({
    cache: "no-store",
  }));

  if (!res.ok) {
    const text = await res.text();
    throw new Error(text.slice(0, 200) || "Failed to load news");
  }

  const body = await parseJson<NewsListResponse>(res);
  return {
    articles: (body.data ?? []).map(mapArticle),
    totalCount: body.meta?.totalCount ?? body.data?.length ?? 0,
  };
}

export async function fetchDashboardNews(
  market: "domestic" | "us" = "domestic",
): Promise<DashboardNewsResult> {
  const path = `/api/v1/news/${market}/dashboard`;
  // 서버도 이 목록을 15초 캐시한다. 여기 캐시는 그 위에서 **왕복 자체를** 없앤다 —
  // 홈을 떠났다 돌아오는 30초 안에는 회선을 아예 쓰지 않는다.
  return cachedGet(path, async () => {
    const res = await fetch(
      `${getApiBaseUrl()}${path}`,
      mergeFetchInit({ cache: "no-store" }),
    );
    if (!res.ok) {
      const text = await res.text();
      throw new Error(text.slice(0, 200) || "Failed to load news");
    }
    const body = await parseJson<DashboardNewsResponse>(res);
    return {
      articles: (body.data ?? []).map(mapArticle),
      fullCount: body.meta?.fullCount ?? body.data?.length ?? 0,
    };
  });
}

function articleMatchesStock(
  article: NewsArticle,
  symbol: string,
  name: string,
): boolean {
  const sym = symbol.trim().toUpperCase();
  if (!sym) return false;

  if (article.symbols?.some((s) => s.trim().toUpperCase() === sym)) {
    return true;
  }

  const haystack = `${article.headline} ${article.summary ?? ""}`.toLowerCase();
  if (haystack.includes(sym.toLowerCase())) {
    return true;
  }

  const trimmedName = name.trim();
  if (trimmedName.length >= 2 && haystack.includes(trimmedName.toLowerCase())) {
    return true;
  }

  return false;
}

/** Stock detail — domestic symbol match or US ticker/name keyword match in headlines. */
export async function fetchStockNews(params: {
  symbol: string;
  name: string;
  market: "domestic" | "us";
  limit?: number;
}): Promise<NewsArticle[]> {
  const symbol = params.symbol.trim();
  const name = params.name.trim();
  const limit = params.limit ?? 12;

  if (params.market === "domestic") {
    const { articles } = await fetchDomesticNews({
      market: "domestic",
      sort: "latest",
      symbol,
    });
    return articles.slice(0, limit);
  }

  if (params.market === "us") {
    const { articles } = await fetchUsNews({
      market: "us",
      sort: "latest",
      symbol,
    });
    return articles.slice(0, limit);
  }

  return [];
}

type UsDemoDTO = NewsArticleDTO & { readerMode?: boolean };

export async function fetchUsNewsDemo(): Promise<NewsArticle> {
  const res = await fetch(`${getApiBaseUrl()}/api/v1/news/us/demo`, mergeFetchInit({
    cache: "no-store",
  }));
  if (!res.ok) {
    const text = await res.text();
    throw new Error(text.slice(0, 200) || "Failed to load US demo article");
  }
  const body = await parseJson<ApiResponse<UsDemoDTO>>(res);
  return mapArticle(body.data);
}

export async function fetchNewsReader(
  url: string,
  opts?: { market?: NewsArticle["market"]; articleId?: string },
): Promise<NewsReaderContent> {
  const params = new URLSearchParams({ url });
  if (opts?.market) {
    params.set("market", opts.market);
  }
  if (opts?.articleId) {
    params.set("articleId", opts.articleId);
  }
  const res = await fetch(`${getApiBaseUrl()}/api/v1/news/reader?${params}`, mergeFetchInit({
    cache: "no-store",
  }));
  if (!res.ok) {
    const body = await parseJson<{ error?: string }>(res).catch(() => ({ error: res.statusText }));
    throw new Error(body.error || "Failed to extract article");
  }
  const body = await parseJson<ApiResponse<NewsReaderContent>>(res);
  return body.data;
}

const readerPrefetch = new Map<string, Promise<NewsReaderContent>>();

function readerPrefetchKey(url: string, market?: NewsArticle["market"], articleId?: string): string {
  return `${market ?? ""}:${articleId ?? ""}:${url.trim()}`;
}

export function prefetchNewsReader(
  url: string,
  opts?: { market?: NewsArticle["market"]; articleId?: string },
): void {
  const trimmed = url.trim();
  const key = readerPrefetchKey(trimmed, opts?.market, opts?.articleId);
  if (!trimmed || trimmed === "#" || readerPrefetch.has(key)) {
    return;
  }
  const promise = fetchNewsReader(trimmed, opts).catch(() => {
    readerPrefetch.delete(key);
    throw new Error("prefetch failed");
  });
  readerPrefetch.set(key, promise);
}

export async function recordNewsView(id: string): Promise<void> {
  const params = new URLSearchParams({ id });
  await fetch(`${getApiBaseUrl()}/api/v1/news/view?${params}`, mergeFetchInit({
    method: "POST",
    cache: "no-store",
  })).catch(() => {
    // non-blocking
  });
}
