import type { ApiResponse } from "@/lib/portfolio-types";
import type { NewsArticle, NewsListFilter, NewsReaderContent, NewsSourceId } from "@/lib/news-types";
import { NEWS_SOURCES } from "@/lib/mock-news";
import { publisherById, resolvePublisherFromUrl } from "@/lib/news-publishers";
import { parseNewsSearchSymbol } from "@/lib/header-search-config";
import { getApiBaseUrl, mergeFetchInit, parseJson } from "@/lib/api-client";

type NewsArticleDTO = {
  id: string;
  market: "domestic" | "us";
  headline: string;
  summary?: string;
  sourceId: NewsSourceId;
  publisherId?: string;
  publisherName?: string;
  tags: string[];
  symbols?: string[];
  publishedAt: string;
  viewCount: number;
  url?: string;
  embedPolicy?: "ok" | "blocked" | "unknown";
  readerMode?: boolean;
  highlighted?: boolean;
};

type NewsListResponse = ApiResponse<NewsArticleDTO[]> & {
  meta?: { count?: number; updatedAt?: string };
};

function mapArticle(dto: NewsArticleDTO): NewsArticle {
  const source =
    NEWS_SOURCES.find((s) => s.id === dto.sourceId) ??
    NEWS_SOURCES.find((s) => s.id === "rss")!;
  const publisher = dto.publisherId
    ? publisherById(dto.publisherId, dto.publisherName)
    : resolvePublisherFromUrl(dto.url, dto.publisherName);
  if (dto.publisherName && publisher.id === "unknown") {
    publisher.name = dto.publisherName;
  }
  return {
    id: dto.id,
    market: dto.market,
    headline: dto.headline,
    summary: dto.summary,
    publisher,
    source,
    tags: dto.tags ?? [],
    symbols: dto.symbols,
    publishedAt: dto.publishedAt,
    viewCount: dto.viewCount ?? 0,
    url: dto.url,
    embedPolicy: dto.embedPolicy ?? "unknown",
    readerMode: dto.readerMode,
    highlighted: dto.highlighted,
  };
}

export async function fetchDomesticNews(filter: NewsListFilter): Promise<NewsArticle[]> {
  const params = new URLSearchParams();
  params.set("sort", filter.sort);
  if (filter.sourceId && filter.sourceId !== "all") {
    params.set("source", filter.sourceId);
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
  return (body.data ?? []).map(mapArticle);
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

export async function fetchNewsReader(url: string): Promise<NewsReaderContent> {
  const params = new URLSearchParams({ url });
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
