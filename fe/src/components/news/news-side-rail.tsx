"use client";

import { useEffect, useMemo, useState } from "react";
import Link from "next/link";
import { Eye } from "lucide-react";
import {
  SectionCard,
  SectionEmpty,
  SectionSkeleton,
} from "@/components/ui/section-card";
import { DirectionBadge } from "@/components/research/research-shared";
import {
  NewsMyStocksCard,
  NewsSourceBreakdownCard,
  NewsUpcomingEventsCard,
} from "@/components/news/news-rail-cards";
import type { QuoteSummary } from "@/lib/quotes-types";
import { fetchDomesticNews, fetchUsNews } from "@/lib/news-api";
import { fetchResearchCallsClient } from "@/lib/research-api";
import { formatRelativeTime } from "@/lib/format";
import type { NewsArticle } from "@/lib/news-types";
import type { ResearchCall } from "@/lib/research-types";

/**
 * 뉴스 화면의 우측 레일.
 *
 * 뉴스 피드는 한 줄에 제목 하나라 폭이 넓어져도 얻는 게 없다 — 오히려 제목 오른쪽이
 * 통째로 빈다. 그 폭을 "이 기사를 읽는 동안 알고 싶은 것"으로 채운다: 지금 지수가
 * 어떤지, 남들은 무엇을 읽는지, 오늘 무슨 단어가 반복되는지.
 *
 * 키워드는 새로 불러오지 않고 **화면에 이미 있는 기사에서 센다** — 같은 화면의 데이터를
 * 다시 요청하면 집계와 목록이 어긋날 수 있고, 요청 하나를 아끼는 것보다 그게 더 나쁘다.
 */

type NewsSideRailProps = {
  market: "domestic" | "us";
  /** 현재 화면에 뿌려진 기사 — 키워드 집계의 원본 */
  articles: NewsArticle[];
  /** 내 보유·관심 종목(비로그인이면 인기 종목 예시) */
  mySymbols: string[];
  mySymbolsPersonal: boolean;
  /** 목록·이슈 스트립과 **공유하는** 시세 — 레일이 따로 부르지 않는다 */
  quotes: Map<string, QuoteSummary>;
  activePublisherId: string | null;
  onPublisherSelect: (publisherId: string | null) => void;
  onArticleClick?: (article: NewsArticle) => void;
  onKeywordSelect?: (keyword: string) => void;
};

/**
 * 레일 순서: 오늘의 키워드 → 다가오는 일정 → 많이 본 뉴스 → 내 종목 뉴스 → 출처 분포 →
 * 최신 투자 리포트. 지수는 중앙 상단 스트립으로 옮겼다.
 */
export function NewsSideRail({
  market,
  articles,
  mySymbols,
  mySymbolsPersonal,
  quotes,
  activePublisherId,
  onPublisherSelect,
  onArticleClick,
  onKeywordSelect,
}: NewsSideRailProps) {
  return (
    <>
      <TrendingKeywordsCard articles={articles} onSelect={onKeywordSelect} />
      <NewsUpcomingEventsCard
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
      <NewsSourceBreakdownCard
        articles={articles}
        activePublisherId={activePublisherId}
        onSelect={onPublisherSelect}
      />
      <RelatedReportsCard />
    </>
  );
}

/* ── 오늘의 키워드 ─────────────────────────────────────────────────────────── */

function TrendingKeywordsCard({
  articles,
  onSelect,
}: {
  articles: NewsArticle[];
  onSelect?: (keyword: string) => void;
}) {
  const keywords = useMemo(() => {
    const counts = new Map<string, number>();
    for (const article of articles) {
      // 한 기사가 같은 단어를 여러 번 달아도 1로 센다 — 태깅이 후한 기사 하나가
      // "오늘의 키워드"를 혼자 정하지 않게.
      for (const raw of new Set(article.keywords ?? [])) {
        const kw = raw.trim();
        if (kw.length < 2) continue;
        counts.set(kw, (counts.get(kw) ?? 0) + 1);
      }
    }
    return [...counts.entries()]
      .filter(([, n]) => n >= 2)
      .sort((a, b) => b[1] - a[1])
      .slice(0, 12);
  }, [articles]);

  return (
    <SectionCard title="오늘의 키워드">
      {keywords.length === 0 ? (
        <SectionEmpty text="반복 등장하는 키워드가 아직 없습니다" />
      ) : (
        <div className="flex flex-wrap gap-1">
          {keywords.map(([kw, count]) => (
            <button
              key={kw}
              type="button"
              onClick={() => onSelect?.(kw)}
              className="inline-flex max-w-full items-center gap-1 rounded-full border px-2 py-0.5 text-[11px] hover:bg-accent"
              title={`${kw} — ${count}건`}
            >
              <span className="truncate">{kw}</span>
              <span className="shrink-0 font-mono text-[10px] text-muted-foreground">{count}</span>
            </button>
          ))}
        </div>
      )}
    </SectionCard>
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

  useEffect(() => {
    let alive = true;
    const fetchNews = market === "domestic" ? fetchDomesticNews : fetchUsNews;
    fetchNews({ market, sort: "popular" })
      .then(({ articles }) => {
        if (alive) setItems(articles.slice(0, 6));
      })
      .catch(() => {
        if (alive) setItems([]);
      });
    return () => {
      alive = false;
    };
  }, [market]);

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
                <span className="w-3 shrink-0 pt-px text-[11px] font-medium text-muted-foreground">
                  {i + 1}
                </span>
                <span className="min-w-0 flex-1">
                  <span className="line-clamp-2 block text-xs leading-snug">
                    {article.headlineKo || article.headline}
                  </span>
                  <span className="mt-1 flex items-center gap-1.5 text-[11px] text-muted-foreground">
                    <Eye className="size-3" />
                    {article.viewCount.toLocaleString("ko-KR")}
                    <span>· {formatRelativeTime(article.publishedAt)}</span>
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

/* ── 최신 리포트 ──────────────────────────────────────────────────────────── */

function RelatedReportsCard() {
  const [calls, setCalls] = useState<ResearchCall[] | null>(null);

  useEffect(() => {
    let alive = true;
    fetchResearchCallsClient({ limit: 3 })
      .then((list) => {
        if (alive) setCalls(list);
      })
      .catch(() => {
        if (alive) setCalls([]);
      });
    return () => {
      alive = false;
    };
  }, []);

  return (
    <SectionCard title="최신 투자 리포트" href="/reports" hrefLabel="리포트">
      {calls === null ? (
        <SectionSkeleton rows={3} />
      ) : calls.length === 0 ? (
        <SectionEmpty text="아직 발행된 리포트가 없습니다" />
      ) : (
        <ul className="min-w-0 space-y-0.5">
          {calls.map((call) => (
            <li key={call.id}>
              <Link
                href={`/reports/${call.id}`}
                className="block rounded-md px-1 py-1.5 hover:bg-accent"
              >
                <span className="flex items-center gap-1">
                  <DirectionBadge direction={call.direction} />
                  <span className="truncate text-[11px] text-muted-foreground">
                    {call.symbolName || call.symbol}
                  </span>
                </span>
                <span className="mt-1 line-clamp-2 block text-xs leading-snug">{call.title}</span>
              </Link>
            </li>
          ))}
        </ul>
      )}
    </SectionCard>
  );
}
