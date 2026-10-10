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
          /*
            한 칩의 상한은 **좁은 화면에서 더 짧다** (2026-09-06 사용자 지적 1).

            9rem(144px)은 375px 화면의 38%다. 키워드 하나가 그만큼 늘어나면 남는
            자리에 두 번째 키워드도, 시각도 들어가지 못한다. 데스크톱에서는 그
            상한이 문제가 된 적이 없으므로 `sm` 위로는 그대로 둔다.
          */
          className="inline-flex max-w-[7rem] truncate rounded-full border border-violet-500/25 bg-violet-500/10 px-2 py-0.5 text-micro font-medium text-violet-700 dark:text-violet-300 sm:max-w-[9rem]"
          title={kw}
        >
          {kw}
        </span>
      ))}
      {rest > 0 && (
        <span className="text-micro text-[var(--text-muted)]">+{rest}</span>
      )}
    </div>
  );
}
