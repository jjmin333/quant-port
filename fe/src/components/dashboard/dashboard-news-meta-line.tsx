"use client";

import { useLayoutEffect, useRef, useState } from "react";
import {
  NewsArticleChips,
  articleChipTotalCount,
} from "@/components/news/news-article-chips";
import { formatRelativeTime } from "@/lib/format";
import { measureMetaChipLayout } from "@/lib/dashboard-news-meta-fit";
import type { NewsArticle } from "@/lib/news-types";
import type { MarketIndex, QuoteSummary } from "@/lib/quotes-types";

const META_CHIP_CAP = 3;

/**
 * 홈 뉴스 행 메타 줄 — 칩 바로 옆에 시간, 가로 폭이 부족할 때만 +N.
 */
export function DashboardNewsMetaLine({
  article,
  quotes,
  indices,
  breaking,
}: {
  article: NewsArticle;
  quotes: Map<string, QuoteSummary>;
  indices: MarketIndex[];
  breaking: boolean;
}) {
  const rowRef = useRef<HTMLDivElement>(null);
  const measureRef = useRef<HTMLDivElement>(null);
  const totalCount = articleChipTotalCount(article, indices);
  const fullMax = Math.min(META_CHIP_CAP, totalCount);
  const [displayMax, setDisplayMax] = useState(fullMax);
  const timeLabel = formatRelativeTime(article.publishedAt);

  useLayoutEffect(() => {
    const row = rowRef.current;
    const measure = measureRef.current;
    if (!row || !measure) return;

    const applyFit = () => {
      const next =
        fullMax === 0 ? 0 : measureMetaChipLayout(row, measure, breaking);
      setDisplayMax((current) => (current === next ? current : next));
    };

    const scheduleFit = () => {
      queueMicrotask(applyFit);
    };

    scheduleFit();
    const observer = new ResizeObserver(scheduleFit);
    observer.observe(row);
    observer.observe(measure);
    return () => observer.disconnect();
  }, [article.id, breaking, fullMax, timeLabel, quotes, indices]);

  return (
    <div
      ref={rowRef}
      className="relative mt-1 flex min-w-0 flex-wrap items-center gap-1.5 text-[11px] text-muted-foreground"
    >
      <div
        ref={measureRef}
        aria-hidden
        className="pointer-events-none invisible absolute left-0 top-0 flex max-w-none flex-nowrap items-center gap-1.5"
      >
        {fullMax > 0 && (
          <NewsArticleChips
            article={article}
            quotes={quotes}
            indices={indices}
            max={fullMax}
            chipClassName="max-w-[7rem]"
            className="flex-nowrap"
          />
        )}
        <span data-meta-time className="shrink-0">
          {timeLabel}
        </span>
        {fullMax > 1 &&
          Array.from({ length: fullMax - 1 }, (_, i) => i + 1).map((hidden) => (
            <span
              key={hidden}
              data-meta-plus={hidden}
              className="shrink-0 text-[10px] text-[var(--text-muted)]"
            >
              +{hidden}
            </span>
          ))}
      </div>

      {breaking && (
        <span
          data-meta-breaking
          className="shrink-0 rounded bg-[var(--profit)]/12 px-1 text-[10px] font-bold text-[var(--profit)]"
        >
          속보
        </span>
      )}
      <span className="inline-flex min-w-0 flex-nowrap items-center gap-1.5">
        {totalCount > 0 && (
          <NewsArticleChips
            article={article}
            quotes={quotes}
            indices={indices}
            max={displayMax}
            overflowHint={displayMax < totalCount}
            chipClassName="max-w-[7rem]"
            className="flex-nowrap"
          />
        )}
        <span className="shrink-0">{timeLabel}</span>
      </span>
    </div>
  );
}
