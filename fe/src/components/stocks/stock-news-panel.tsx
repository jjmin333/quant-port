"use client";

import { useEffect, useMemo, useState } from "react";
import Link from "next/link";
import { Card, CardHeader, CardTitle } from "@/components/ui/card";
import { PageNav } from "@/components/ui/page-nav";
import { NewsCard } from "@/components/news/news-card";
import { useNewsArticleModal } from "@/components/news/news-article-modal-host";
import { fetchStockNews } from "@/lib/news-api";
import type { NewsArticle } from "@/lib/news-types";
import { formatRelativeTime } from "@/lib/format-news";
import { cn } from "@/lib/utils";
import { toUserMessage } from "@/lib/error-message";

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
          setError(toUserMessage(err, "뉴스를 불러오지 못했습니다."));
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

  /*
    # 기사는 **모달로 연다** (2026-09-05 사용자 지적 4)

    여기만 `window.open(link)`으로 새 탭을 띄우고 있었다. 앱의 다른 모든 자리는
    `useNewsArticleModal`을 쓴다 — 그래서 같은 기사가 홈·지수 모달·종목 퀵룩에서는
    번역·원문 셸과 「관련 종목·지수」를 달고 열리는데, **종목 화면의 뉴스 탭에서만**
    앱 밖 언론사 사이트로 튕겨 나갔다. 종목을 보다가 기사를 하나 열면 돌아올 길이
    브라우저 뒤로가기뿐이었다.

    호출부가 정할 일은 없다 — 어느 셸을 쓸지(해외 헤드라인 / 리더 / iframe)와
    조회 기록은 그 훅이 안다(`news-article-modal-host.tsx` 머리말).

    `pool`로 지금 받아 둔 목록을 넘기면 모달 안에 「같은 이슈」 묶음이 함께 선다.
  */
  const newsModal = useNewsArticleModal({ pool: articles });
  const openArticle = newsModal.open;

  if (variant === "compact") {
    return (
      <>
      <section className={cn("flex min-h-0 flex-col", className)}>
        <div className="flex shrink-0 items-center justify-between gap-2 px-1 pb-2">
          <h3 className="text-cell font-medium text-[var(--text-secondary)]">실시간 뉴스</h3>
          <Link href={newsHref} className="text-micro text-[var(--brand)] hover:underline">
            전체 →
          </Link>
        </div>
        <div className="min-h-0 flex-1 overflow-y-auto">
          {loading ? (
            <p className="py-4 text-center text-cell text-[var(--text-muted)]">로딩 중…</p>
          ) : error ? (
            <p className="py-4 text-center text-cell text-[var(--text-muted)]">{error}</p>
          ) : articles.length === 0 ? (
            <p className="py-4 text-center text-cell text-[var(--text-muted)]">관련 뉴스 없음</p>
          ) : (
            <ul className="space-y-0.5">
              {articles.map((article) => (
                <li key={article.id}>
                  <button
                    type="button"
                    onClick={() => openArticle(article)}
                    className="w-full rounded-lg px-2 py-2 text-left transition-colors hover:bg-[var(--surface-elevated)]/60"
                  >
                    <p className="line-clamp-2 text-cell font-medium leading-snug text-[var(--text-primary)]">
                      {article.headline}
                    </p>
                    <p
                      className="mt-1 truncate text-micro text-[var(--text-muted)]"
                      suppressHydrationWarning
                    >
                      {formatRelativeTime(article.publishedAt)} · {article.publisher.name}
                    </p>
                  </button>
                </li>
              ))}
            </ul>
          )}
        </div>
      </section>
      {newsModal.modal}
      </>
    );
  }

  return (
    <>
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
        <Link href={newsHref} className="text-cell text-[var(--brand)] hover:underline">
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
            <PageNav
              page={safePage}
              totalPages={totalPages}
              onChange={setPage}
              label="뉴스 페이지"
              className="mt-3"
            />
          </>
        )}
      </div>
    </Card>
    {newsModal.modal}
    </>
  );
}
