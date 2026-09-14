"use client";

import type { NewsArticle } from "@/lib/news-types";
import type { QuoteSummary, MarketIndex } from "@/lib/quotes-types";
import { NewsCard } from "@/components/news/news-card";
import { newsTimeGroup, newsTimeGroupLabel, type NewsTimeGroup } from "@/lib/news-derive";

type NewsFeedProps = {
  articles: NewsArticle[];
  emptyMessage?: string;
  onArticleClick?: (article: NewsArticle) => void;
  /** C1 — 종목·지수 칩 시세. 목록 전체가 한 배치 요청을 공유한다. */
  quotes?: Map<string, QuoteSummary>;
  /** 그 배치가 아직 돌아오지 않았다 — 칩이 종목코드를 대신 그리지 않게. */
  quotesLoading?: boolean;
  indices?: MarketIndex[];
  /** C2 — 시간 구분선. 검색 결과처럼 시간순이 아닌 목록에서는 끈다. */
  grouped?: boolean;
};

export function NewsFeed({
  articles,
  emptyMessage = "표시할 뉴스가 없습니다.",
  onArticleClick,
  quotes,
  quotesLoading,
  indices,
  grouped = false,
}: NewsFeedProps) {
  if (articles.length === 0) {
    return (
      <div className="px-6 py-16 text-center text-sm text-[var(--text-muted)]">
        {emptyMessage}
      </div>
    );
  }

  if (!grouped) {
    return (
      <div className="divide-y divide-[var(--border)]">
        {articles.map((article) => (
          <NewsCard
            key={article.id}
            article={article}
            onClick={onArticleClick}
            quotes={quotes}
            quotesLoading={quotesLoading}
            indices={indices}
          />
        ))}
      </div>
    );
  }

  /*
   * 시간 묶음은 **목록이 이미 최신순**이라는 전제 위에서만 성립한다. 한 번 훑으며
   * 묶음이 바뀌는 지점에 머리글을 끼우면 되고, 그래서 정렬을 다시 하지 않는다 —
   * 인기순처럼 다른 정렬에서는 호출부가 grouped를 끈다.
   */
  const now = new Date();
  const sections: Array<{ group: NewsTimeGroup; items: NewsArticle[] }> = [];
  for (const article of articles) {
    const group = newsTimeGroup(article, now);
    const last = sections[sections.length - 1];
    if (last && last.group === group) last.items.push(article);
    else sections.push({ group, items: [article] });
  }

  return (
    <div>
      {sections.map((section, i) => (
        <section key={`${section.group}-${i}`}>
          <div className="flex items-center gap-2 pb-0.5 pt-3 first:pt-1">
            <span className="shrink-0 text-cell font-bold text-[var(--text-muted)]">
              {newsTimeGroupLabel(section.group)}
            </span>
            <span className="h-px flex-1 bg-[var(--border)]" />
            <span className="shrink-0 text-micro text-[var(--text-muted)]">
              {section.items.length}건
            </span>
          </div>
          <div className="divide-y divide-[var(--border)]">
            {section.items.map((article) => (
              <NewsCard
                key={article.id}
                article={article}
                onClick={onArticleClick}
                quotes={quotes}
            quotesLoading={quotesLoading}
            indices={indices}
              />
            ))}
          </div>
        </section>
      ))}
    </div>
  );
}
