"use client";

import { cn } from "@/lib/utils";

type NewsKeywordBadgesProps = {
  keywords?: string[];
  className?: string;
  max?: number;
};

export function NewsKeywordBadges({ keywords, className, max = 4 }: NewsKeywordBadgesProps) {
  const items = (keywords ?? []).filter((kw) => kw.trim()).slice(0, max);
  const rest = (keywords?.length ?? 0) - items.length;
  if (items.length === 0) return null;

  return (
    <div className={cn("flex flex-wrap items-center gap-1", className)}>
      {items.map((kw) => (
        <span
          key={kw}
          className="inline-flex max-w-[9rem] truncate rounded-full border border-violet-500/25 bg-violet-500/10 px-2 py-0.5 text-[10px] font-medium text-violet-300"
          title={kw}
        >
          {kw}
        </span>
      ))}
      {rest > 0 && (
        <span className="text-[10px] text-[var(--text-muted)]">+{rest}</span>
      )}
    </div>
  );
}
