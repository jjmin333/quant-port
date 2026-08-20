"use client";

import { NEWS_BREAKING_LABEL } from "@/lib/news-breaking";
import { cn } from "@/lib/utils";

type NewsBreakingBadgeProps = {
  className?: string;
};

export function NewsBreakingBadge({ className }: NewsBreakingBadgeProps) {
  return (
    <span
      className={cn(
        "news-badge news-badge--red inline-flex shrink-0 items-center px-2 py-0.5 text-[10px] font-semibold",
        className,
      )}
    >
      {NEWS_BREAKING_LABEL}
    </span>
  );
}
