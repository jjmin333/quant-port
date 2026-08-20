"use client";

import { useMemo } from "react";
import { useRouter } from "next/navigation";
import { DashboardNewsMetaLine } from "@/components/dashboard/dashboard-news-meta-line";
import { SectionCard, SectionEmpty, SectionSkeleton } from "@/components/ui/section-card";
import { useMarketIndices } from "@/hooks/use-market-indices";
import { useSymbolQuotes } from "@/hooks/use-news-symbols";
import { isArticleBreakingNews } from "@/lib/news-breaking";
import { allArticleSymbols } from "@/lib/news-derive";
import type { NewsArticle } from "@/lib/news-types";

/**
 * 홈의 뉴스 파생 카드 둘 — **같은 기사 목록 하나**로 만든다.
 *
 * 목록·키워드·속보 수를 각자 부르면 같은 화면의 숫자가 서로 어긋난다(그 사이에 기사가
 * 들어온다). 그래서 기사는 `DashboardOverview`가 한 번만 받아 두 카드에 내려준다 —
 * 뉴스 화면이 이미 쓰는 규칙과 같다.
 *
 * `articles === null`은 아직 받는 중, `[]`는 받았는데 없음이다. 둘을 같은 화면으로
 * 그리면 실패한 것과 비어 있는 것을 구분할 수 없다.
 */

export function LatestNewsCard({
  articles,
  onPick,
  className,
}: {
  articles: NewsArticle[] | null;
  onPick: (article: NewsArticle) => void;
  className?: string;
}) {
  const rows = useMemo(() => articles?.slice(0, 8) ?? null, [articles]);

  /*
   * 이 기사가 **무엇에 관한 것인지**를 제목만으로 알 수 없다. 뉴스 화면의 카드는 이미
   * 아래에 지수·종목 칩을 달고 있는데(`NewsArticleChips`), 홈은 제목·시각만으로는
   * 「엔비디아 기사」와 「코스피 기사」가 같은 모양으로 쌓여 있었다.
   *
   * 홈에서는 **언론사 대신** 그 칩을 제목 아래 한 줄에 둔다 — 칩을 별도 줄로 내리면
   * 옆 브리핑 카드와 높이를 맞추느라 빈칸이 커진다. 시세는 **화면에 보이는 여덟 건**의
   * 종목만 부른다 — 홈은 100건을 받아 두지만 나머지 92건은 칩이 뜰 자리가 없다.
   */
  const chipSymbols = useMemo(() => (rows ? allArticleSymbols(rows) : []), [rows]);
  const quotes = useSymbolQuotes(chipSymbols);
  const { indices } = useMarketIndices();

  return (
    <SectionCard title="오늘의 뉴스" href="/news/domestic" hrefLabel="뉴스" className={className}>
      {rows === null ? (
        <SectionSkeleton rows={8} />
      ) : rows.length === 0 ? (
        <SectionEmpty text="표시할 뉴스가 없습니다" />
      ) : (
        <ul className="min-w-0 space-y-0.5">
          {rows.map((n) => (
            <li key={n.id}>
              {/*
                칩이 링크라 행은 `<button>`이 될 수 없다 — 버튼 안의 링크는 무효한
                마크업이고 브라우저마다 다르게 동작한다. 뉴스 화면의 카드가 이미
                `role="button"`을 쓰는 것과 같은 이유다.
              */}
              <div
                role="button"
                tabIndex={0}
                onClick={() => onPick(n)}
                onKeyDown={(e) => {
                  if (e.key === "Enter" || e.key === " ") {
                    e.preventDefault();
                    onPick(n);
                  }
                }}
                className="block w-full cursor-pointer rounded-md px-1 py-1.5 text-left hover:bg-accent"
              >
                <p className="line-clamp-2 text-xs leading-snug">{n.headlineKo || n.headline}</p>
                <DashboardNewsMetaLine
                  article={n}
                  quotes={quotes}
                  indices={indices}
                  breaking={isArticleBreakingNews(n)}
                />
              </div>
            </li>
          ))}
        </ul>
      )}
    </SectionCard>
  );
}

/**
 * 오늘의 키워드 — **추가 요청 0**.
 *
 * 집계 규칙은 뉴스 화면의 레일 카드와 같은 것을 쓴다: 한 기사가 같은 단어를 여러 번
 * 달아도 1로 세고(태깅이 후한 기사 하나가 「오늘의 키워드」를 혼자 정하지 않게),
 * 두 건 미만은 버린다(한 번 나온 단어는 화제가 아니다).
 *
 * 눌러서 뉴스 화면으로 넘어갈 때 **검색어로 넘긴다** — 홈에서 좁혀 봐야 볼 기사가
 * 여기엔 여덟 건뿐이라, 좁히는 일은 목록이 다 있는 쪽에서 하는 게 맞다.
 */
export function TodayKeywordsCard({ articles }: { articles: NewsArticle[] | null }) {
  const router = useRouter();

  const keywords = useMemo(() => {
    if (!articles) return null;
    const counts = new Map<string, number>();
    for (const article of articles) {
      for (const raw of new Set(article.keywords ?? [])) {
        const kw = raw.trim();
        if (kw.length < 2) continue;
        counts.set(kw, (counts.get(kw) ?? 0) + 1);
      }
    }
    return [...counts.entries()]
      .filter(([, n]) => n >= 2)
      .sort((a, b) => b[1] - a[1] || a[0].localeCompare(b[0]))
      .slice(0, 14);
  }, [articles]);

  const breakingCount = useMemo(
    () => (articles ? articles.filter(isArticleBreakingNews).length : 0),
    [articles],
  );

  return (
    <SectionCard
      title="오늘의 키워드"
      href="/news/domestic"
      hrefLabel="뉴스"
    >
      {keywords === null ? (
        <SectionSkeleton rows={5} />
      ) : keywords.length === 0 ? (
        <SectionEmpty text="반복 등장하는 키워드가 아직 없습니다" />
      ) : (
        <>
          <div className="flex flex-wrap gap-1">
            {keywords.map(([kw, count]) => (
              <button
                key={kw}
                type="button"
                onClick={() => router.push(`/news/domestic?keyword=${encodeURIComponent(kw)}`)}
                title={`${kw} — ${count}건`}
                className="inline-flex max-w-full items-center gap-1 rounded-full border px-2 py-0.5 text-[11px] transition-colors hover:border-foreground/20 hover:bg-accent"
              >
                <span className="truncate">{kw}</span>
                <span className="shrink-0 font-mono text-[10px] text-muted-foreground">
                  {count}
                </span>
              </button>
            ))}
          </div>
          <p className="mt-2 border-t pt-2 text-[10px] leading-relaxed text-muted-foreground">
            최근 기사 {articles?.length ?? 0}건에서 2회 이상 등장한 단어
            {breakingCount > 0 && (
              <>
                {" · "}
                <b className="text-[var(--profit)]">속보 {breakingCount}건</b>
              </>
            )}
          </p>
        </>
      )}
    </SectionCard>
  );
}
