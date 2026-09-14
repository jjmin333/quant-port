"use client";

import { useEffect, useMemo, useState } from "react";
import { useRouter } from "next/navigation";
import {
  ChipCollapseRow,
  ChipCollapseToggle,
  useChipCollapse,
} from "@/components/ui/chip-collapse";
import { SectionCard, SectionEmpty, SectionSkeleton } from "@/components/ui/section-card";
import { fetchTopKeywords, type NewsTopKeywords } from "@/lib/news-api";
import { isArticleBreakingNews } from "@/lib/news-breaking";
import type { NewsArticle } from "@/lib/news-types";

/*
「오늘의 키워드」 — **한 벌** (2026-09-05 사용자 요청 7).

# 무엇이 두 벌이었나

같은 이름·같은 엔드포인트(`/news/{market}/keywords`)·같은 빈 문구를 쓰는 카드가
홈(`dashboard-news-cards`의 `TodayKeywordsCard`)과 뉴스 레일(`news-side-rail`의
`TrendingKeywordsCard`)에 각각 있었다. 다른 것은 셋뿐이었다:

	홈   : 「전체 보기」 접힘 · 각주(모수·속보 수) · 누르면 뉴스 화면으로 검색어를 싣고 나감
	레일 : 접힘 없음 · 각주 없음 · 누르면 **그 화면에서** 목록을 좁힘

셋 다 「이 카드가 어디에 서 있나」의 결과지 카드가 하는 일의 차이가 아니다. 그래서
한 부품으로 모으고 그 셋만 prop으로 가른다.

**의도적인 차이는 남긴다**(사용자 지시). 레일에서 누르면 그 자리에서 좁히는 것은
목록이 바로 옆에 있기 때문이고, 홈에서 나가는 것은 홈에 볼 기사가 열넷뿐이기
때문이다 — 그 판단은 화면이 하고, 이 카드는 `onSelect`를 받는지로만 안다.

# 집계는 서버가 한다

예전 홈은 받아 온 기사 100건을 브라우저에서 셌다. 100건은 국내 기준 반나절도 못
덮어서 「오늘의 키워드」가 사실상 「최근 두 시간의 키워드」였다. 지금은 시장의 최근
1,000건이라는 **고정된 모수**를 서버가 세고, 건너오는 것은 열몇 줄뿐이다.

목록을 필터링하거나 더 불러올 때 키워드가 함께 흔들리지 않는 것도 그 덕이다 —
언론사 하나로 좁히면 「오늘의 키워드」가 그 언론사의 키워드가 되던 시절이 있었다.
*/

type TrendingKeywordsCardProps = {
  market: "domestic" | "us";
  /** 받아 올 키워드 수. 같은 값이면 두 화면이 한 요청을 나눠 쓴다(`cachedGet`은 URL 키). */
  limit?: number;
  /**
   * 칩을 눌렀을 때. **넘기면 그 화면에서 좁힌다**(뉴스 레일), 안 넘기면 뉴스 화면으로
   * 검색어를 싣고 나간다(홈).
   */
  onSelect?: (keyword: string) => void;
  /**
   * 「전체 보기」 접힘과 각주를 붙인다 — 홈처럼 카드가 넓고 혼자 설 때.
   *
   * 레일에서는 붙이지 않는다. 폭이 좁아 칩이 어차피 몇 줄로 접히고, 각주 한 줄이
   * 레일 전체를 그만큼 아래로 민다.
   */
  variant?: "rail" | "full";
  /**
   * 각주의 속보 수를 세는 모수 — 이 화면이 **이미 들고 있는** 기사.
   *
   * 키워드 모수(서버의 1,000건)와 다르므로 문구를 나눠 적는다. 여기서 다시 부르지
   * 않는 이유는 부르는 쪽이 그 목록을 이미 화면에 그리고 있기 때문이다.
   */
  articles?: NewsArticle[] | null;
};

export function TrendingKeywordsCard({
  market,
  limit = 14,
  onSelect,
  variant = "full",
  articles = null,
}: TrendingKeywordsCardProps) {
  const router = useRouter();
  /*
    **어느 시장의 결과인지 함께 들고 있는다.** 시장이 바뀔 때 이펙트에서 동기로
    비우면 렌더가 한 번 더 돌고(`react-hooks/set-state-in-effect`), 그러지 않으면
    새 응답이 오기 전까지 앞 시장의 키워드가 그대로 서 있다. 키가 안 맞으면
    「아직 받는 중」으로 읽는다 — 저장소의 다른 카드(`useSymbolQuotesState`)와 같은 패턴이다.
  */
  const [loaded, setLoaded] = useState<{ key: string; top: NewsTopKeywords } | null>(null);
  const key = `${market}:${limit}`;

  useEffect(() => {
    let alive = true;
    fetchTopKeywords(market, limit)
      .then((res) => {
        if (alive) setLoaded({ key: `${market}:${limit}`, top: res });
      })
      .catch(() => {
        if (alive) {
          setLoaded({
            key: `${market}:${limit}`,
            top: { keywords: [], articleCount: 0, minMentions: 2 },
          });
        }
      });
    return () => {
      alive = false;
    };
  }, [market, limit]);

  const top = loaded && loaded.key === key ? loaded.top : null;
  const keywords = top ? top.keywords.map((k) => [k.keyword, k.count] as const) : null;
  const chips = useChipCollapse(keywords?.length ?? 0);
  const full = variant === "full";

  const breakingCount = useMemo(
    () => (articles ? articles.filter(isArticleBreakingNews).length : 0),
    [articles],
  );

  const pick = (keyword: string) => {
    if (onSelect) {
      onSelect(keyword);
      return;
    }
    router.push(`/news/${market}?keyword=${encodeURIComponent(keyword)}`);
  };

  const chipButtons = keywords?.map(([kw, count]) => (
    <button
      key={kw}
      type="button"
      onClick={() => pick(kw)}
      title={`${kw} — ${count}건`}
      className="inline-flex max-w-full items-center gap-1 rounded-full border px-2 py-0.5 text-cell transition-colors hover:border-foreground/20 hover:bg-accent"
    >
      <span className="truncate">{kw}</span>
      <span className="shrink-0 font-num text-micro text-muted-foreground">{count}</span>
    </button>
  ));

  return (
    <SectionCard
      /*
        「전체 보기」는 **제목 바로 오른쪽**이다 — 칩 줄 아래가 아니라
        (2026-09-03 결정, `docs/UI_SPEC.md`의 접힘 규약). 헤더 오른쪽 끝의
        「뉴스 →」와는 하는 일이 다르므로 자리도 다르게 둔다.
      */
      title={
        full ? (
          <span className="flex min-w-0 items-baseline gap-1.5">
            오늘의 키워드
            <ChipCollapseToggle state={chips} className="font-normal" />
          </span>
        ) : (
          "오늘의 키워드"
        )
      }
      href={full ? `/news/${market}` : undefined}
      hrefLabel={full ? "뉴스" : undefined}
    >
      {keywords === null ? (
        <SectionSkeleton rows={full ? 5 : 4} />
      ) : keywords.length === 0 ? (
        <SectionEmpty text="반복 등장하는 키워드가 아직 없습니다" />
      ) : full ? (
        <>
          <ChipCollapseRow state={chips} className="gap-1">
            {chipButtons}
          </ChipCollapseRow>
          <p className="mt-2 border-t pt-2 text-micro leading-relaxed text-muted-foreground">
            최근 기사 {(top?.articleCount ?? 0).toLocaleString("ko-KR")}건에서{" "}
            {top?.minMentions ?? 2}회 이상 등장한 단어
            {breakingCount > 0 && (
              <>
                {/* 속보는 이 화면이 들고 있는 목록에서 센 값이라 모수가 다르다. */}
                {" · "}
                <b className="text-[var(--profit)]">
                  최근 {articles?.length ?? 0}건 중 속보 {breakingCount}건
                </b>
              </>
            )}
          </p>
        </>
      ) : (
        <div className="flex flex-wrap gap-1">{chipButtons}</div>
      )}
    </SectionCard>
  );
}
