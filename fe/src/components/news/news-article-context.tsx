"use client";

import { useMemo } from "react";
import { NewsRelatedEntityPanel } from "@/components/news/news-related-stock-panel";
import { NewsIssueCluster } from "@/components/news/news-issue-cluster";
import { articleDashboardIndexIds } from "@/lib/news-dashboard-indices";
import { relatedArticles } from "@/lib/news-derive";
import type { NewsArticle } from "@/lib/news-types";
import type { QuoteSummary } from "@/lib/quotes-types";

/**
 * 기사 모달 세 개(국내 리더 · 국내 iframe · 해외 헤드라인)가 **함께 쓰는** 아래 블록.
 *
 * 셋은 본문을 보여주는 방식만 다르고, 본문 다음에 필요한 것은 똑같다 —
 * 이 기사가 어느 종목·지수 이야기인지(D1), 같은 사건을 누가 또 썼는지(D2). 모달마다
 * 따로 만들면 셋이 곧 어긋난다.
 *
 * **관심·메모·스탬프는 종목 카드(`StockMiniCard`)에만 붙는다.** 예전 모달 하단 한 줄은
 * 종목이 여럿이어도 **첫 번째 종목에만** 작용했다. 종목·지수가 없는 거시 기사(금리 등)에는
 * 액션을 두지 않는다 — 메모·스탬프는 종목 단위 기능이고, 기사 링크만 남기는 버튼은
 * 실제로 저장되지 않았다.
 *
 * iframe 모달의 우측 칸은 본문 폭을 줄인다. 관련 종목·지수도, 같은 이슈 묶음도 없으면
 * 칸 자체를 그리지 않는다 — 빈 검은 세로 띠만 남는다.
 */
export function articleModalContextHasContent(
  article: NewsArticle,
  pool: NewsArticle[],
): boolean {
  const symbols = (article.symbols ?? []).map((s) => s.trim().toUpperCase()).filter(Boolean);
  if (symbols.length > 0 || articleDashboardIndexIds(article).length > 0) {
    return true;
  }
  return relatedArticles(article, pool).length > 0;
}

export function NewsArticleContext({
  article,
  pool,
  quotes,
  onArticleClick,
  onNavigate,
}: {
  article: NewsArticle;
  /** 같은 이슈 묶기의 후보 — 지금 화면에 있는 기사들 */
  pool: NewsArticle[];
  quotes: Map<string, QuoteSummary>;
  onArticleClick?: (article: NewsArticle) => void;
  /** 카드에서 다른 화면으로 떠날 때 모달을 닫는다. */
  onNavigate?: () => void;
}) {
  const symbols = useMemo(
    () => (article.symbols ?? []).map((s) => s.trim().toUpperCase()).filter(Boolean),
    [article.symbols],
  );

  const hasEntities = symbols.length > 0 || articleDashboardIndexIds(article).length > 0;
  const hasContent = useMemo(
    () => articleModalContextHasContent(article, pool),
    [article, pool],
  );

  if (!hasContent) {
    return null;
  }

  return (
    <div className="space-y-2.5">
      {hasEntities && (
        <div>
          <p className="mb-1.5 text-[11px] font-semibold text-[var(--text-muted)]">
            관련 종목 · 지수
          </p>
          <NewsRelatedEntityPanel
            article={article}
            symbols={symbols}
            quotes={quotes}
            onNavigate={onNavigate}
          />
        </div>
      )}

      <NewsIssueCluster article={article} pool={pool} onArticleClick={onArticleClick} />
    </div>
  );
}
