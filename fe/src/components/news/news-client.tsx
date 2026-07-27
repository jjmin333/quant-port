"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { ChevronDown, RefreshCw } from "lucide-react";
import type { NewsArticle, NewsMarket, NewsSort, NewsSourceId } from "@/lib/news-types";
import {
  getMockNews,
  MOCK_NEWS_ARTICLES,
  NEWS_MARKET_LABELS,
  sourcesForMarket,
} from "@/lib/mock-news";
import { fetchDomesticNews, fetchUsNewsDemo } from "@/lib/news-api";
import { useHeaderSearch } from "@/components/providers/header-search-provider";
import { NewsFeed } from "@/components/news/news-feed";
import { NewsArticleModal } from "@/components/news/news-article-modal";
import { NewsReaderModal } from "@/components/news/news-reader-modal";
import { cn } from "@/lib/utils";

const SORT_LABELS: Record<NewsSort, string> = {
  latest: "전체 · 최신순",
  popular: "전체 · 인기순",
};

type NewsClientProps = {
  market: NewsMarket;
};

export function NewsClient({ market }: NewsClientProps) {
  const { activeQuery, isSearchActive, mode } = useHeaderSearch();
  const marketSources = useMemo(() => sourcesForMarket(market), [market]);
  const newsSearchActive =
    isSearchActive &&
    ((market === "domestic" && mode === "news-domestic") ||
      (market === "us" && mode === "news-us"));
  const [sourceId, setSourceId] = useState<NewsSourceId | "all">("all");
  const [sort, setSort] = useState<NewsSort>("latest");
  const [refreshKey, setRefreshKey] = useState(0);
  const [sourceOpen, setSourceOpen] = useState(false);
  const [sortOpen, setSortOpen] = useState(false);
  const [refreshing, setRefreshing] = useState(false);
  const [articles, setArticles] = useState<NewsArticle[]>([]);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [loading, setLoading] = useState(market === "domestic");
  const [selectedArticle, setSelectedArticle] = useState<NewsArticle | null>(null);
  const [readerArticle, setReaderArticle] = useState<NewsArticle | null>(null);

  const mockArticles = useMemo(
    () => getMockNews({ market, sourceId, sort }),
    [market, sourceId, sort, refreshKey],
  );

  useEffect(() => {
    if (market === "domestic") {
      let cancelled = false;
      setLoading(true);
      setLoadError(null);

      fetchDomesticNews({
        market,
        sourceId,
        sort,
        query: newsSearchActive ? activeQuery : undefined,
      })
        .then((items) => {
          if (!cancelled) {
            setArticles(items);
          }
        })
        .catch((err: unknown) => {
          if (!cancelled) {
            setLoadError(err instanceof Error ? err.message : "뉴스를 불러오지 못했습니다");
            setArticles([]);
          }
        })
        .finally(() => {
          if (!cancelled) {
            setLoading(false);
          }
        });

      return () => {
        cancelled = true;
      };
    }

    if (market === "us") {
      let cancelled = false;
      setLoading(true);
      setLoadError(null);

      fetchUsNewsDemo()
        .then((demo) => {
          if (!cancelled) {
            const items = [demo];
            if (newsSearchActive) {
              const q = activeQuery.toLowerCase();
              const filtered = items.filter(
                (a) =>
                  a.headline.toLowerCase().includes(q) ||
                  a.symbols?.some((s) => s.toLowerCase().includes(q)),
              );
              setArticles(filtered);
            } else {
              setArticles(items);
            }
          }
        })
        .catch((err: unknown) => {
          if (!cancelled) {
            setLoadError(err instanceof Error ? err.message : "Reader API 데모를 불러오지 못했습니다");
            setArticles([]);
          }
        })
        .finally(() => {
          if (!cancelled) {
            setLoading(false);
          }
        });

      return () => {
        cancelled = true;
      };
    }

    setArticles(mockArticles);
    setLoading(false);
    setLoadError(null);
  }, [market, sourceId, sort, refreshKey, mockArticles, activeQuery, newsSearchActive]);

  const displayArticles = market === "domestic" || market === "us" ? articles : mockArticles;

  const sourceLabel =
    sourceId === "all"
      ? "출처"
      : (marketSources.find((s) => s.id === sourceId)?.name ?? "출처");

  const handleRefresh = useCallback(() => {
    setRefreshing(true);
    setRefreshKey((k) => k + 1);
    window.setTimeout(() => setRefreshing(false), 400);
  }, []);

  const handleArticleClick = useCallback((article: NewsArticle) => {
    if (article.readerMode) {
      setReaderArticle(article);
      return;
    }
    setSelectedArticle(article);
  }, []);

  const handleCloseModal = useCallback(() => {
    setSelectedArticle(null);
  }, []);

  const handleCloseReader = useCallback(() => {
    setReaderArticle(null);
  }, []);

  return (
    <>
      <header className="flex shrink-0 items-center justify-between gap-4 border-b border-[var(--border)] bg-[var(--background)] px-4 py-4 sm:px-6">
        <h1 className="text-lg font-semibold text-[var(--text-primary)]">
          {NEWS_MARKET_LABELS[market]}
        </h1>

        <div className="flex items-center gap-2">
          <FilterDropdown
            label={sourceLabel}
            open={sourceOpen}
            onToggle={() => {
              setSourceOpen((v) => !v);
              setSortOpen(false);
            }}
            onClose={() => setSourceOpen(false)}
          >
            <FilterOption
              active={sourceId === "all"}
              onClick={() => {
                setSourceId("all");
                setSourceOpen(false);
              }}
            >
              전체 출처
            </FilterOption>
            {marketSources.map((source) => (
              <FilterOption
                key={source.id}
                active={sourceId === source.id}
                onClick={() => {
                  setSourceId(source.id);
                  setSourceOpen(false);
                }}
              >
                {source.name}
              </FilterOption>
            ))}
          </FilterDropdown>

          <FilterDropdown
            label={SORT_LABELS[sort]}
            open={sortOpen}
            onToggle={() => {
              setSortOpen((v) => !v);
              setSourceOpen(false);
            }}
            onClose={() => setSortOpen(false)}
          >
            {(Object.keys(SORT_LABELS) as NewsSort[]).map((key) => (
              <FilterOption
                key={key}
                active={sort === key}
                onClick={() => {
                  setSort(key);
                  setSortOpen(false);
                }}
              >
                {SORT_LABELS[key]}
              </FilterOption>
            ))}
          </FilterDropdown>

          <button
            type="button"
            aria-label="새로고침"
            onClick={handleRefresh}
            className="flex h-9 w-9 items-center justify-center rounded-full border border-[var(--border)] text-[var(--text-secondary)] transition-colors hover:bg-[var(--surface-elevated)] hover:text-[var(--text-primary)]"
          >
            <RefreshCw className={cn("h-4 w-4", refreshing && "animate-spin")} />
          </button>
        </div>
      </header>

      <main className="flex-1 overflow-y-auto">
        <div className="mx-auto min-w-[var(--dashboard-content-min)] max-w-3xl">
          <div className="border-b border-[var(--border)] px-4 py-2 sm:px-5">
            <p className="text-[11px] text-[var(--text-muted)]">
              {market === "domestic"
                ? loading
                  ? "뉴스 불러오는 중…"
                  : newsSearchActive
                    ? `DB 검색 · ${displayArticles.length}건 · 「${activeQuery}」`
                    : `${displayArticles.length}건 · Naver + RSS (카카오 보완)`
                : market === "us"
                  ? loading
                    ? "Reader API 데모 불러오는 중…"
                    : newsSearchActive
                      ? `검색 · ${displayArticles.length}건 · 「${activeQuery}」`
                      : "1건 · 국내 RSS → Reader API (미국 탭 테스트)"
                  : `Mock 데이터 · ${displayArticles.length}건`}
            </p>
            {loadError && (
              <p className="mt-0.5 text-[11px] text-red-400">{loadError}</p>
            )}
            {market === "us" && !loading && !loadError && (
              <p className="mt-0.5 text-[11px] text-[var(--text-muted)]">
                국내 RSS 수집 기사 1건 · 클릭 시 Reader API로 본문 추출 (미국 뉴스 기능 미구현)
              </p>
            )}
          </div>
          {!loading && displayArticles.length === 0 ? (
            <p className="px-4 py-8 text-center text-sm text-[var(--text-muted)] sm:px-5">
              {market === "domestic"
                ? newsSearchActive
                  ? "검색 결과가 없습니다. 다른 키워드를 입력해 보세요."
                  : "표시할 뉴스가 없습니다. 잠시 후 새로고침해 주세요."
                : "표시할 뉴스가 없습니다."}
            </p>
          ) : (
            <NewsFeed articles={displayArticles} onArticleClick={handleArticleClick} />
          )}
        </div>
      </main>

      <NewsArticleModal article={selectedArticle} onClose={handleCloseModal} />
      <NewsReaderModal article={readerArticle} onClose={handleCloseReader} />
    </>
  );
}

function FilterDropdown({
  label,
  open,
  onToggle,
  onClose,
  children,
}: {
  label: string;
  open: boolean;
  onToggle: () => void;
  onClose: () => void;
  children: React.ReactNode;
}) {
  return (
    <div className="relative">
      <button
        type="button"
        onClick={onToggle}
        className="inline-flex h-9 items-center gap-1 rounded-lg border border-[var(--border)] bg-[var(--surface)] px-3 text-xs text-[var(--text-secondary)] hover:text-[var(--text-primary)]"
      >
        <span className="max-w-[7rem] truncate sm:max-w-none">{label}</span>
        <ChevronDown className={cn("h-3.5 w-3.5 transition-transform", open && "rotate-180")} />
      </button>
      {open && (
        <>
          <button
            type="button"
            aria-label="닫기"
            className="fixed inset-0 z-10 cursor-default"
            onClick={onClose}
          />
          <div className="absolute right-0 z-20 mt-1 min-w-[9rem] overflow-hidden rounded-lg border border-[var(--border)] bg-[var(--surface-elevated)] py-1 shadow-lg">
            {children}
          </div>
        </>
      )}
    </div>
  );
}

function FilterOption({
  active,
  onClick,
  children,
}: {
  active: boolean;
  onClick: () => void;
  children: React.ReactNode;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={cn(
        "block w-full px-3 py-2 text-left text-xs transition-colors",
        active
          ? "bg-[var(--accent-muted)] text-[var(--accent)]"
          : "text-[var(--text-secondary)] hover:bg-[var(--surface)] hover:text-[var(--text-primary)]",
      )}
    >
      {children}
    </button>
  );
}
