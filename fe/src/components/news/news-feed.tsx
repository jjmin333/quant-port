"use client";

import type { NewsArticle } from "@/lib/news-types";
import { NewsCard } from "@/components/news/news-card";

type NewsFeedProps = {
  articles: NewsArticle[];
  emptyMessage?: string;
  onArticleClick?: (article: NewsArticle) => void;
};

export function NewsFeed({
  articles,
  emptyMessage = "표시할 뉴스가 없습니다.",
  onArticleClick,
}: NewsFeedProps) {
  if (articles.length === 0) {
    return (
      <div className="px-6 py-16 text-center text-sm text-[var(--text-muted)]">
        {emptyMessage}
      </div>
    );
  }

  return (
    <div className="divide-y divide-[var(--border)]">
      {articles.map((article) => (
        <NewsCard key={article.id} article={article} onClick={onArticleClick} />
      ))}
    </div>
  );
}
