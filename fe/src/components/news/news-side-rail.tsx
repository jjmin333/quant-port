"use client";

import { useCallback, useEffect, useState } from "react";
import { Eye } from "lucide-react";
import {
  SectionCard,
  SectionEmpty,
  SectionSkeleton,
} from "@/components/ui/section-card";
import {
  NewsMyStocksCard,
  NewsUpcomingEventsCard,
} from "@/components/news/news-rail-cards";
import { TrendingKeywordsCard } from "@/components/news/trending-keywords-card";
import { useVisibleInterval } from "@/hooks/use-visible-interval";
import { NEWS_REFRESH_MS } from "@/lib/screen-refresh";
import type { QuoteSummary } from "@/lib/quotes-types";
import { fetchDomesticNews, fetchUsNews } from "@/lib/news-api";
import { formatRelativeTime } from "@/lib/format";
import type { NewsArticle } from "@/lib/news-types";

/**
 * 뉴스 화면의 우측 레일.
 *
 * 뉴스 피드는 한 줄에 제목 하나라 폭이 넓어져도 얻는 게 없다 — 오히려 제목 오른쪽이
 * 통째로 빈다. 그 폭을 "이 기사를 읽는 동안 알고 싶은 것"으로 채운다: 지금 지수가
 * 어떤지, 남들은 무엇을 읽는지, 오늘 무슨 단어가 반복되는지.
 *
 * 키워드만 레일이 따로 받는다 — 서버가 시장의 최근 1,000건에서 집계한 결과다
 * (`components/news/trending-keywords-card`, 홈과 같은 부품). 나머지 카드는 화면이
 * 이미 들고 있는 기사·시세를 그대로 쓴다.
 */

type NewsSideRailProps = {
  market: "domestic" | "us";
  /** 현재 화면에 뿌려진 기사 — 「내 종목 뉴스」가 쓴다 */
  articles: NewsArticle[];
  /** 내 보유·관심 종목(비로그인이면 인기 종목 예시) */
  mySymbols: string[];
  mySymbolsPersonal: boolean;
  /** 목록·이슈 스트립과 **공유하는** 시세 — 레일이 따로 부르지 않는다 */
  quotes: Map<string, QuoteSummary>;
  onArticleClick?: (article: NewsArticle) => void;
  onKeywordSelect?: (keyword: string) => void;
};

/**
 * 레일 순서: 오늘의 키워드 → 다가오는 일정 → 많이 본 뉴스 → 내 종목 뉴스.
 * 지수는 중앙 상단 스트립으로 옮겼다.
 *
 * 「출처 분포」와 「최신 투자 리포트」는 뺐다. 출처로 거르는 일은 쓰이지 않았고,
 * 리포트는 레일이 아니라 `/reports`가 있을 자리다 — 레일이 길어질수록 모바일에서는
 * 본문 아래로 쌓이는 세로 길이가 그대로 늘어난다.
 */
export function NewsSideRail({
  market,
  articles,
  mySymbols,
  mySymbolsPersonal,
  quotes,
  onArticleClick,
  onKeywordSelect,
}: NewsSideRailProps) {
  return (
    <>
      {/*
        홈과 **같은 부품**이다 (2026-09-05 사용자 요청 7 — `trending-keywords-card`).
        레일에서는 접힘·각주를 빼고(`variant="rail"`), 누르면 이 화면에서 좁힌다.
      */}
      {/*
        **레일도 목록과 같은 시각을 산다** (2026-09-16 사용자 요청).

        `refreshMs`를 안 주면 이 카드는 마운트 시각에 멈춘다. 옆의 기사 목록이 90초마다
        새로 오는 화면에서 그러면 한 화면에 시각이 둘 섞이고, 그건 홈에서 이미 한 번
        밟은 함정이다(`lib/screen-refresh`의 「한 카드 안에서 두 시각」).
      */}
      <TrendingKeywordsCard
        market={market}
        variant="rail"
        limit={12}
        refreshMs={NEWS_REFRESH_MS}
        onSelect={onKeywordSelect}
      />
      <NewsUpcomingEventsCard
        market={market}
        symbols={mySymbols}
        quotes={quotes}
        personal={mySymbolsPersonal}
      />
      <MostReadCard key={market} market={market} onArticleClick={onArticleClick} />
      <NewsMyStocksCard
        articles={articles}
        symbols={mySymbols}
        quotes={quotes}
        personal={mySymbolsPersonal}
        onArticleClick={onArticleClick}
      />
    </>
  );
}

/* ── 많이 본 뉴스 ─────────────────────────────────────────────────────────── */

function MostReadCard({
  market,
  onArticleClick,
}: {
  market: "domestic" | "us";
  onArticleClick?: (article: NewsArticle) => void;
}) {
  const [items, setItems] = useState<NewsArticle[] | null>(null);

  const load = useCallback(
    () =>
      (market === "domestic" ? fetchDomesticNews : fetchUsNews)({ market, sort: "popular" }),
    [market],
  );

  useEffect(() => {
    let alive = true;
    load()
      .then(({ articles }) => {
        if (alive) setItems(articles.slice(0, 6));
      })
      .catch(() => {
        if (alive) setItems([]);
      });
    return () => {
      alive = false;
    };
  }, [load]);

  /*
    조회수는 **사람이 지금 만드는 값**이라 목록과 같은 90초 주기를 쓴다. 갱신이
    실패하면 직전 순위를 남긴다 — 「집계된 조회 기록이 없습니다」로 떨어뜨리면
    있던 것을 없애는 일이 된다(`lib/screen-refresh`).
  */
  useVisibleInterval(() => {
    load()
      .then(({ articles }) => setItems(articles.slice(0, 6)))
      .catch(() => {});
  }, NEWS_REFRESH_MS);

  return (
    <SectionCard title="많이 본 뉴스">
      {items === null ? (
        <SectionSkeleton rows={6} />
      ) : items.length === 0 ? (
        <SectionEmpty text="집계된 조회 기록이 없습니다" />
      ) : (
        <ol className="min-w-0">
          {items.map((article, i) => (
            <li key={article.id}>
              <button
                type="button"
                onClick={() => onArticleClick?.(article)}
                className="flex w-full min-w-0 gap-2 rounded-md px-1 py-1.5 text-left hover:bg-accent"
              >
                <span className="w-3 shrink-0 pt-px text-cell font-medium text-muted-foreground">
                  {i + 1}
                </span>
                <span className="min-w-0 flex-1">
                  <span className="line-clamp-2 block text-cell leading-snug">
                    {article.headlineKo || article.headline}
                  </span>
                  <span className="mt-1 flex items-center gap-1.5 text-cell text-muted-foreground">
                    <Eye className="size-3" />
                    {article.viewCount.toLocaleString("ko-KR")}
                    {/* 상대시각 — SSR/hydration 불일치 억제 (news-card 주석 참조) */}
                    <span suppressHydrationWarning>· {formatRelativeTime(article.publishedAt)}</span>
                  </span>
                </span>
              </button>
            </li>
          ))}
        </ol>
      )}
    </SectionCard>
  );
}
