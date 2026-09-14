"use client";

import { Megaphone } from "lucide-react";
import type { NewsArticle } from "@/lib/news-types";
import { newsDisplayHeadline } from "@/lib/news-display";
import { cn } from "@/lib/utils";

type NewsBreakingTickerProps = {
  article: NewsArticle | null;
  badgeLabel: string;
  className?: string;
};

export function NewsBreakingTicker({ article, badgeLabel, className }: NewsBreakingTickerProps) {
  if (!article) {
    return null;
  }

  return (
    <div
      className={cn(
        "flex items-center gap-2.5 rounded-xl border border-[var(--border)] bg-[var(--surface)]/80 px-3.5 py-2.5",
        className,
      )}
    >
      <Megaphone className="h-4 w-4 shrink-0 text-[var(--brand)]" aria-hidden />
      <span className="shrink-0 rounded-md bg-[var(--brand-muted)] px-2 py-0.5 text-cell font-semibold text-[var(--brand)]">
        {badgeLabel}
      </span>
      <p className="min-w-0 truncate text-note font-medium leading-snug text-[var(--text-primary)]">
        {newsDisplayHeadline(article)}
      </p>
    </div>
  );
}
