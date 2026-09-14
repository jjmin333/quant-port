"use client";

import { useMemo } from "react";
import type { NewsArticle } from "@/lib/news-types";
import { relatedArticles } from "@/lib/news-derive";
import { newsDisplayHeadline } from "@/lib/news-display";
import { formatRelativeTime } from "@/lib/format-news";
import { cn } from "@/lib/utils";

/**
 * D2 — 같은 사건을 다룬 다른 기사.
 *
 * 해외 뉴스는 이미 `dedupeNewsArticles`로 중복을 **버리고** 있었다. 버리는 대신
 * 접어서 보여주면 같은 사건에 매체별 논조가 어떻게 다른지를 볼 수 있다.
 *
 * 묶음은 제목 토큰의 유사도로 만든다 — 본문을 받아오지 않으므로 추가 요청이 없다.
 * 대신 정확도가 완벽하지 않아 「관련 있어 보이는 기사」라고 적는다. 확신을 표시하면
 * 틀렸을 때 더 나쁘다.
 */
export function NewsIssueCluster({
  article,
  pool,
  onArticleClick,
  className,
}: {
  article: NewsArticle;
  pool: NewsArticle[];
  onArticleClick?: (article: NewsArticle) => void;
  className?: string;
}) {
  const related = useMemo(() => relatedArticles(article, pool), [article, pool]);
  if (related.length === 0) return null;

  return (
    <div className={cn("rounded-xl border border-[var(--border)] bg-[var(--surface)] p-2.5", className)}>
      <div className="mb-1 flex items-baseline justify-between gap-2">
        <p className="text-cell font-semibold text-[var(--text-muted)]">
          관련 있어 보이는 기사 {related.length}건
        </p>
        <span className="text-micro text-[var(--text-muted)]">매체별 논조 비교</span>
      </div>
      <ul className="min-w-0">
        {related.map((item) => (
          <li key={item.id}>
            <button
              type="button"
              onClick={() => onArticleClick?.(item)}
              className="flex w-full min-w-0 gap-2 border-t border-[var(--border)]/60 py-1.5 text-left first:border-t-0 hover:bg-[var(--surface-elevated)]"
            >
              <span
                className={cn(
                  "mt-px inline-flex max-w-[5.5rem] shrink-0 truncate rounded-md border px-1.5 py-px text-micro font-semibold",
                  item.publisher.badgeClass,
                )}
              >
                {item.publisher.name}
              </span>
              <span className="min-w-0 flex-1">
                <span className="line-clamp-2 block text-cell leading-snug text-[var(--text-primary)]">
                  {newsDisplayHeadline(item)}
                </span>
                <span
                  className="mt-0.5 block text-micro text-[var(--text-muted)]"
                  suppressHydrationWarning
                >
                  {formatRelativeTime(item.publishedAt)}
                </span>
              </span>
            </button>
          </li>
        ))}
      </ul>
    </div>
  );
}
