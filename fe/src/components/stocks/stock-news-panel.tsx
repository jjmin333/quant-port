"use client";

import { useEffect, useMemo, useState } from "react";
import Link from "next/link";
import { ChevronLeft, ChevronRight } from "lucide-react";
import { Card, CardHeader, CardTitle } from "@/components/ui/card";
import { NewsCard } from "@/components/news/news-card";
import { fetchStockNews } from "@/lib/news-api";
import type { NewsArticle } from "@/lib/news-types";
import { formatRelativeTime } from "@/lib/format-news";
import { cn } from "@/lib/utils";

type StockNewsPanelProps = {
  symbol: string;
  name: string;
  market: "domestic" | "us";
  /** Sidebar: fewer items, no thumbnails */
  variant?: "default" | "compact";
  /** How many articles to fetch in total (paginated client-side in the default variant). */
  limit?: number;
  /** Items per page — default variant only. */
  pageSize?: number;
  className?: string;
};

/** Numbered page window (max 5 numbers) around the current page, à la 1 … 4 5 6 … 12. */
function pageWindow(current: number, total: number): (number | "ellipsis")[] {
  if (total <= 7) return Array.from({ length: total }, (_, i) => i + 1);
  const pages = new Set<number>([1, total, current, current - 1, current + 1]);
  const sorted = Array.from(pages)
    .filter((p) => p >= 1 && p <= total)
    .sort((a, b) => a - b);
  const out: (number | "ellipsis")[] = [];
  sorted.forEach((p, i) => {
    if (i > 0 && p - sorted[i - 1] > 1) out.push("ellipsis");
    out.push(p);
  });
  return out;
}

export function StockNewsPanel({
  symbol,
  name,
  market,
  variant = "default",
  limit,
  pageSize = 8,
  className,
}: StockNewsPanelProps) {
  const [articles, setArticles] = useState<NewsArticle[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [page, setPage] = useState(1);

  const itemLimit = limit ?? (variant === "compact" ? 5 : 60);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    setError(null);
    setPage(1);

    fetchStockNews({ symbol, name, market, limit: itemLimit })
      .then((items) => {
        if (!cancelled) setArticles(items);
      })
      .catch((err) => {
        if (!cancelled) {
          setArticles([]);
          setError(err instanceof Error ? err.message : "뉴스를 불러오지 못했습니다.");
        }
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });

    return () => {
      cancelled = true;
    };
  }, [symbol, name, market, itemLimit]);

  const totalPages = Math.max(1, Math.ceil(articles.length / pageSize));
  // Guard against `page` outliving a shrunken result set (e.g. a refetch that
  // returns fewer articles while the user was on a later page).
  const safePage = Math.min(page, totalPages);
  const pageArticles = useMemo(
    () => articles.slice((safePage - 1) * pageSize, safePage * pageSize),
    [articles, safePage, pageSize],
  );

  const newsHref = market === "us" ? "/news/us" : "/news/domestic";

  const openArticle = (article: NewsArticle) => {
    const link = article.originalUrl?.trim() || article.url?.trim();
    if (link) window.open(link, "_blank", "noopener,noreferrer");
  };

  if (variant === "compact") {
    return (
      <section className={cn("flex min-h-0 flex-col", className)}>
        <div className="flex shrink-0 items-center justify-between gap-2 px-1 pb-2">
          <h3 className="text-xs font-medium text-[var(--text-secondary)]">실시간 뉴스</h3>
          <Link href={newsHref} className="text-[10px] text-[var(--brand)] hover:underline">
            전체 →
          </Link>
        </div>
        <div className="min-h-0 flex-1 overflow-y-auto">
          {loading ? (
            <p className="py-4 text-center text-xs text-[var(--text-muted)]">로딩 중…</p>
          ) : error ? (
            <p className="py-4 text-center text-xs text-[var(--text-muted)]">{error}</p>
          ) : articles.length === 0 ? (
            <p className="py-4 text-center text-xs text-[var(--text-muted)]">관련 뉴스 없음</p>
          ) : (
            <ul className="space-y-0.5">
              {articles.map((article) => (
                <li key={article.id}>
                  <button
                    type="button"
                    onClick={() => openArticle(article)}
                    className="w-full rounded-lg px-2 py-2 text-left transition-colors hover:bg-[var(--surface-elevated)]/60"
                  >
                    <p className="line-clamp-2 text-xs font-medium leading-snug text-[var(--text-primary)]">
                      {article.headline}
                    </p>
                    <p className="mt-1 truncate text-[10px] text-[var(--text-muted)]">
                      {formatRelativeTime(article.publishedAt)} · {article.publisher.name}
                    </p>
                  </button>
                </li>
              ))}
            </ul>
          )}
        </div>
      </section>
    );
  }

  return (
    <Card className={className}>
      <CardHeader className="flex flex-row items-center justify-between gap-2">
        <CardTitle>
          실시간 뉴스
          {!loading && articles.length > 0 && (
            <span className="ml-1.5 font-normal text-[var(--text-muted)]">
              {articles.length.toLocaleString("ko-KR")}건
            </span>
          )}
        </CardTitle>
        <Link href={newsHref} className="text-xs text-[var(--brand)] hover:underline">
          전체 뉴스 →
        </Link>
      </CardHeader>
      <div className="px-4 pb-4">
        {loading ? (
          <p className="py-8 text-center text-sm text-[var(--text-muted)]">뉴스 로딩 중…</p>
        ) : error ? (
          <p className="py-8 text-center text-sm text-[var(--text-muted)]">{error}</p>
        ) : articles.length === 0 ? (
          <p className="py-8 text-center text-sm text-[var(--text-muted)]">
            「{name}」 관련 최신 뉴스가 없습니다.
          </p>
        ) : (
          <>
            <div className="divide-y divide-[var(--border)]">
              {pageArticles.map((article) => (
                <NewsCard key={article.id} article={article} onClick={openArticle} />
              ))}
            </div>
            {totalPages > 1 && (
              <nav
                aria-label="뉴스 페이지"
                className="mt-3 flex flex-wrap items-center justify-center gap-1 border-t border-[var(--border)] pt-3"
              >
                <button
                  type="button"
                  onClick={() => setPage((p) => Math.max(1, p - 1))}
                  disabled={safePage <= 1}
                  aria-label="이전 페이지"
                  className="flex h-7 w-7 items-center justify-center rounded-md text-[var(--text-secondary)] hover:bg-[var(--surface-elevated)] disabled:pointer-events-none disabled:opacity-30"
                >
                  <ChevronLeft className="h-3.5 w-3.5" />
                </button>
                {pageWindow(safePage, totalPages).map((p, i) =>
                  p === "ellipsis" ? (
                    <span
                      key={`ellipsis-${i}`}
                      className="flex h-7 w-7 items-center justify-center text-xs text-[var(--text-muted)]"
                    >
                      …
                    </span>
                  ) : (
                    <button
                      key={p}
                      type="button"
                      onClick={() => setPage(p)}
                      aria-current={p === safePage ? "page" : undefined}
                      className={cn(
                        "flex h-7 min-w-7 items-center justify-center rounded-md px-1.5 text-xs font-medium tabular-nums",
                        p === safePage
                          ? "bg-[var(--brand)] text-[var(--brand-foreground)]"
                          : "text-[var(--text-secondary)] hover:bg-[var(--surface-elevated)]",
                      )}
                    >
                      {p}
                    </button>
                  ),
                )}
                <button
                  type="button"
                  onClick={() => setPage((p) => Math.min(totalPages, p + 1))}
                  disabled={safePage >= totalPages}
                  aria-label="다음 페이지"
                  className="flex h-7 w-7 items-center justify-center rounded-md text-[var(--text-secondary)] hover:bg-[var(--surface-elevated)] disabled:pointer-events-none disabled:opacity-30"
                >
                  <ChevronRight className="h-3.5 w-3.5" />
                </button>
              </nav>
            )}
          </>
        )}
      </div>
    </Card>
  );
}
