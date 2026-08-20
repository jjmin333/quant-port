"use client";

import type { NewsArticle } from "@/lib/news-types";
import { newsArticleLink, newsUsesReader } from "@/lib/news-types";
import { newsDisplayHeadline } from "@/lib/news-display";
import { newsCardSummary } from "@/lib/news-card-summary";
import { prefetchNewsReader } from "@/lib/news-api";
import { formatRelativeTime } from "@/lib/format-news";
import { isArticleBreakingNews } from "@/lib/news-breaking";
import { NewsCardThumbnail } from "@/components/news/news-card-thumbnail";
import { NewsBreakingBadge } from "@/components/news/news-breaking-badge";
import { NewsKeywordBadges } from "@/components/news/news-keyword-badges";
import { NewsArticleChips } from "@/components/news/news-article-chips";
import type { QuoteSummary } from "@/lib/quotes-types";
import type { MarketIndex } from "@/lib/quotes-types";
import { articleDashboardIndexIds } from "@/lib/news-dashboard-indices";
import { cn } from "@/lib/utils";

type NewsCardProps = {
  article: NewsArticle;
  onClick?: (article: NewsArticle) => void;
  /** C1 — 종목·지수 칩 시세. 목록 전체가 한 배치 요청을 공유한다. */
  quotes?: Map<string, QuoteSummary>;
  indices?: MarketIndex[];
};

export function NewsCard({ article, onClick, quotes, indices }: NewsCardProps) {
  const link = newsArticleLink(article);
  const clickable = Boolean(onClick || link);
  const headline = newsDisplayHeadline(article);
  const summary = newsCardSummary(article);
  const isBreaking = isArticleBreakingNews(article);
  const isUs = article.market === "us";
  const hasChips = Boolean(
    quotes &&
      indices &&
      ((article.symbols?.length ?? 0) > 0 || articleDashboardIndexIds(article).length > 0),
  );

  const handlePrefetch = () => {
    if (!isUs && newsUsesReader(article) && link) {
      prefetchNewsReader(link, { market: article.market, articleId: article.id });
    }
  };

  return (
    <article
      role={clickable ? "button" : undefined}
      tabIndex={clickable ? 0 : undefined}
      onClick={() => onClick?.(article)}
      onMouseEnter={handlePrefetch}
      onFocus={handlePrefetch}
      onKeyDown={(e) => {
        if (clickable && (e.key === "Enter" || e.key === " ")) {
          e.preventDefault();
          onClick?.(article);
        }
      }}
      className={cn(
        "min-h-[100px] border-b border-[var(--border)] py-2.5 transition-colors last:border-b-0",
        article.highlighted
          ? "bg-violet-500/[0.04] hover:bg-violet-500/[0.07]"
          : "hover:bg-[var(--surface-elevated)]/40",
        clickable && "cursor-pointer",
      )}
    >
      <div className="grid min-h-20 grid-cols-[5rem_minmax(0,1fr)] items-center gap-3">
        <NewsCardThumbnail imageUrl={article.imageUrl} compact />

        <div className="flex max-h-20 min-w-0 flex-col justify-center gap-1 overflow-hidden">
          <div className="flex min-w-0 items-center gap-1">
            <div className="flex min-w-0 flex-1 items-center gap-1 overflow-hidden">
              <span
                className={cn(
                  "inline-flex max-w-[5.5rem] shrink-0 truncate rounded-md border px-1.5 py-px text-[9px] font-semibold",
                  article.publisher.badgeClass,
                )}
              >
                {article.publisher.name}
              </span>
              {isBreaking && <NewsBreakingBadge className="px-1.5 py-px text-[9px]" />}
              <NewsKeywordBadges
                keywords={article.keywords}
                max={2}
                className="min-w-0 flex-1 gap-0.5 overflow-hidden [&_span]:max-w-[4.5rem] [&_span]:px-1.5 [&_span]:py-px [&_span]:text-[9px]"
              />
            </div>
            <time
              dateTime={article.publishedAt}
              className="shrink-0 text-[10px] text-[var(--text-muted)]"
            >
              {formatRelativeTime(article.publishedAt)}
            </time>
          </div>

          <h2 className="line-clamp-2 text-[13px] font-bold leading-snug text-[var(--text-primary)]">
            {headline}
          </h2>

          {hasChips ? (
            /*
             * 종목이 걸린 기사는 요약 대신 종목 칩 줄을 쓴다. 둘 다 넣으면 행 높이가
             * 기사마다 달라져 목록이 들쭉날쭉해진다 — 시세가 요약보다 값이 크다.
             */
            <NewsArticleChips article={article} quotes={quotes!} indices={indices!} />
          ) : (
            summary && (
              <p className="line-clamp-2 text-[11px] leading-snug text-[var(--text-secondary)]">
                {summary}
              </p>
            )
          )}
        </div>
      </div>
    </article>
  );
}
