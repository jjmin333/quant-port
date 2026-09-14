"use client";

import { useLayoutEffect, useRef, useState } from "react";
import {
  NewsArticleChips,
  articleChipTotalCount,
} from "@/components/news/news-article-chips";
import { formatRelativeTime } from "@/lib/format";
import { measureMetaChipLayout, metaPlusCandidates } from "@/lib/dashboard-news-meta-fit";
import type { NewsArticle } from "@/lib/news-types";
import type { MarketIndex, QuoteSummary } from "@/lib/quotes-types";

const META_CHIP_CAP = 3;

/**
 * 홈 뉴스 행 메타 줄 — 칩 바로 옆에 시간, 가로 폭이 부족할 때만 +N.
 *
 * # 칩은 **줄어들지 않는다** (2026-09-05 사용자 지적 2)
 *
 * 칩에 `shrink-0`을 준다. 없으면 폭이 모자랄 때 flex가 칩들을 눌러 「코…」처럼
 * 한 글자로 만든다 — 그게 사용자가 「오류처럼 느껴진다」고 한 그 모양이다.
 * 잘려도 되는 것은 **이름 안쪽**(`max-w-[7rem]` + `truncate`, 「LG에너지…」)이지
 * 칩 자체가 아니다.
 *
 * `shrink-0`을 주면 계산이 틀렸을 때 칩이 부모 밖으로 나가는데, 그래서 아래 측정이
 * 「+N」의 폭까지 세도록 함께 고쳤다(`dashboard-news-meta-fit`의 머리글).
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
        fullMax === 0 ? 0 : measureMetaChipLayout(row, measure, breaking, totalCount);
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
  }, [article.id, breaking, fullMax, totalCount, timeLabel, quotes, indices]);

  return (
    <div
      ref={rowRef}
      className="relative mt-1 flex min-w-0 flex-wrap items-center gap-1.5 text-cell text-muted-foreground"
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
            chipClassName="max-w-[7rem] shrink-0"
            className="flex-nowrap"
          />
        )}
        <span data-meta-time className="shrink-0" suppressHydrationWarning>
          {timeLabel}
        </span>
        {/*
          「+N」의 폭도 함께 잰다. 예전에는 `fullMax - 1`까지만 그려서, 칩 상한(3)을
          다 쓰는데 총 개수가 그보다 많은 흔한 경우의 「+8」을 **잴 방법이 없었다** —
          그래서 그 폭이 계산에서 통째로 빠졌다.
        */}
        {metaPlusCandidates(totalCount, fullMax).map((hidden) => (
          <span
            key={hidden}
            data-meta-plus={hidden}
            className="shrink-0 text-micro text-[var(--text-muted)]"
          >
            +{hidden}
          </span>
        ))}
      </div>

      {breaking && (
        <span
          data-meta-breaking
          className="shrink-0 rounded bg-[var(--profit)]/12 px-1 text-micro font-bold text-[var(--profit)]"
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
            chipClassName="max-w-[7rem] shrink-0"
            className="flex-nowrap"
          />
        )}
        {/* 상대시각 — SSR/hydration 불일치 억제 (news-card 주석 참조) */}
        <span className="shrink-0" suppressHydrationWarning>{timeLabel}</span>
      </span>
    </div>
  );
}
