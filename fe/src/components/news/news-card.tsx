"use client";

import { Eye } from "lucide-react";
import type { NewsArticle } from "@/lib/news-types";
import { formatRelativeTime, formatViewCount } from "@/lib/format-news";
import { cn } from "@/lib/utils";

type NewsCardProps = {
  article: NewsArticle;
  onClick?: (article: NewsArticle) => void;
};

export function NewsCard({ article, onClick }: NewsCardProps) {
  const clickable = Boolean(onClick || article.url);

  return (
    <article
      role={clickable ? "button" : undefined}
      tabIndex={clickable ? 0 : undefined}
      onClick={() => onClick?.(article)}
      onKeyDown={(e) => {
        if (clickable && (e.key === "Enter" || e.key === " ")) {
          e.preventDefault();
          onClick?.(article);
        }
      }}
      className={cn(
        "border-b border-[var(--border)] px-4 py-4 transition-colors sm:px-5",
        article.highlighted
          ? "bg-violet-500/[0.06] hover:bg-violet-500/[0.09]"
          : "hover:bg-[var(--surface-elevated)]/60",
        clickable && "cursor-pointer",
      )}
    >
      <div className="flex items-start justify-between gap-3">
        <div className="flex min-w-0 flex-wrap items-center gap-2">
          <span
            className={cn(
              "inline-flex shrink-0 items-center rounded-md border px-2 py-0.5 text-[11px] font-semibold",
              article.publisher.badgeClass,
            )}
          >
            {article.publisher.name}
          </span>
          {article.tags.map((tag) => (
            <span
              key={tag}
              className="text-[11px] font-medium text-[var(--text-muted)]"
            >
              {tag}
            </span>
          ))}
        </div>
        <time
          dateTime={article.publishedAt}
          className="shrink-0 whitespace-nowrap text-[11px] text-[var(--text-muted)]"
        >
          {formatRelativeTime(article.publishedAt)}
        </time>
      </div>

      <h2 className="mt-2.5 text-[15px] font-semibold leading-snug text-[var(--text-primary)]">
        {article.headline}
      </h2>

      <div className="mt-3 flex justify-end">
        <span className="inline-flex items-center gap-1 text-[11px] text-[var(--text-muted)]">
          <Eye className="h-3.5 w-3.5" aria-hidden />
          <span className="font-mono tabular-nums">{formatViewCount(article.viewCount)}</span>
        </span>
      </div>
    </article>
  );
}
