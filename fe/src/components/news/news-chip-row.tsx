"use client";

import { useLayoutEffect, useRef, useState } from "react";
import {
  NewsArticleChips,
  articleChipTotalCount,
} from "@/components/news/news-article-chips";
import { measureMetaChipLayout, metaPlusCandidates } from "@/lib/dashboard-news-meta-fit";
import type { NewsArticle } from "@/lib/news-types";
import type { MarketIndex, QuoteSummary } from "@/lib/quotes-types";
import { cn } from "@/lib/utils";

/**
 * 기사 카드 아래의 **한 줄짜리** 종목·지수 칩 줄.
 *
 * # 왜 생겼나 (2026-09-11 사용자 요청 4)
 *
 * 모바일에서 이 줄이 **두 줄이 되면 카드가 깨졌다.** 실측(375px, 국내 뉴스 목록 83행):
 * 15행의 칩이 두 줄로 넘어갔고 그중 14행은 잘렸다 — 글 칸이 `max-h-24`(96px)인데
 * 내용이 101px이라 `overflow-hidden`이 둘째 줄을 **5px 남기고 가로로 베어** 냈다.
 * 카드마다 베이는 높이가 달라 목록이 들쭉날쭉해 보이던 것이 그 증상이다.
 *
 * # 왜 `flex-nowrap` 하나로 안 되나
 *
 * 줄바꿈만 막으면 flex가 칩들을 **눌러서** 한 줄에 밀어 넣는다. 실측하면 「코…」·「니…」
 * ·「삼성…」이 되고 마지막 칩은 반쯤 잘린다 — 그건 이미 한 번 고친 버그다
 * (`dashboard-news-meta-fit` 머리글의 2026-09-05 사용자 지적 2: 「「코…」는 그냥
 * 고장으로 읽힌다」).
 *
 * 그래서 홈 메타 줄이 쓰는 것과 **같은 계산**을 쓴다: 숨은 측정 줄에 칩을 그려 실제
 * 폭을 재고, 한 줄에 들어가는 개수만 그린 뒤 나머지는 「+N」으로 말한다. 하나도 못
 * 들어가면 0개 + 「+N」이다 — 뭉개진 이름보다 그게 더 많은 것을 말한다.
 *
 * 홈 메타 줄과 다른 점은 하나다: 이 줄에는 **시각이 함께 서지 않는다**(목록 카드는
 * 시각이 윗줄에 있다). 그래서 측정 줄에 `data-meta-time`을 두지 않고, 계산은 그 폭을
 * 0으로 받는다.
 */

/** 한 줄에 그릴 수 있는 칩의 상한. 홈 메타 줄과 같은 값이다. */
const CHIP_CAP = 3;

export function NewsChipRow({
  article,
  quotes,
  quotesLoading = false,
  indices,
  className,
}: {
  article: NewsArticle;
  quotes: Map<string, QuoteSummary>;
  quotesLoading?: boolean;
  indices: MarketIndex[];
  className?: string;
}) {
  const rowRef = useRef<HTMLDivElement>(null);
  const measureRef = useRef<HTMLDivElement>(null);
  const totalCount = articleChipTotalCount(article, indices);
  const fullMax = Math.min(CHIP_CAP, totalCount);
  const [displayMax, setDisplayMax] = useState(fullMax);

  useLayoutEffect(() => {
    const row = rowRef.current;
    const measure = measureRef.current;
    if (!row || !measure) return;

    const applyFit = () => {
      const next =
        fullMax === 0 ? 0 : measureMetaChipLayout(row, measure, false, totalCount);
      setDisplayMax((current) => (current === next ? current : next));
    };
    // 칩 폭은 시세가 붙으면서 바뀐다(「삼성전자」 → 「삼성전자 -2.61%」). 마이크로태스크로
    // 미루면 그 렌더가 끝난 뒤의 폭을 잰다.
    const scheduleFit = () => queueMicrotask(applyFit);

    scheduleFit();
    const observer = new ResizeObserver(scheduleFit);
    observer.observe(row);
    observer.observe(measure);
    return () => observer.disconnect();
  }, [article.id, fullMax, totalCount, quotes, indices, quotesLoading]);

  if (totalCount === 0) return null;

  /*
    `overflow-hidden`은 **측정 줄을 위한 것**이다.

    측정 줄은 `absolute`라 부모의 레이아웃 높이에는 영향을 주지 않는데, 부모보다
    몇 px 높으면 조상의 `scrollHeight`에는 그만큼 더해진다. 이 부품이 사는 자리가
    `max-h-24 overflow-hidden` 글 칸이라(news-card), 그 4px이 「칸이 넘쳤다」는 신호를
    만든다 — 눈에 보이는 것은 아무것도 잘리지 않지만, 이 버그를 찾을 때 쓴 신호가
    바로 그것이라 계기를 흐리지 않게 여기서 가둔다.
  */
  return (
    <div
      ref={rowRef}
      className={cn("relative flex min-w-0 items-center overflow-hidden", className)}
    >
      <div
        ref={measureRef}
        aria-hidden
        className="pointer-events-none invisible absolute left-0 top-0 flex max-w-none flex-nowrap items-center gap-1"
      >
        {fullMax > 0 && (
          <NewsArticleChips
            article={article}
            quotes={quotes}
            quotesLoading={quotesLoading}
            indices={indices}
            max={fullMax}
            chipClassName="max-w-[7rem] shrink-0"
            className="flex-nowrap"
          />
        )}
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

      <NewsArticleChips
        article={article}
        quotes={quotes}
        quotesLoading={quotesLoading}
        indices={indices}
        max={displayMax}
        overflowHint={displayMax < totalCount}
        chipClassName="max-w-[7rem] shrink-0"
        className="flex-nowrap"
      />
    </div>
  );
}
