"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { DashboardNewsMetaLine } from "@/components/dashboard/dashboard-news-meta-line";
import { MiniToggle } from "@/components/ui/mini-toggle";
import { useOpenMarket } from "@/hooks/use-open-market";
import { SectionCard, SectionEmpty, SectionSkeleton } from "@/components/ui/section-card";
import { useMarketIndices } from "@/hooks/use-market-indices";
import { useSymbolQuotes } from "@/hooks/use-news-symbols";
import { useVisibleInterval } from "@/hooks/use-visible-interval";
import { SCREEN_REFRESH_MS } from "@/lib/screen-refresh";
import { fetchDashboardNews } from "@/lib/news-api";
import { isArticleBreakingNews } from "@/lib/news-breaking";
import { allArticleSymbols } from "@/lib/news-derive";
import type { NewsArticle, NewsMarket, NewsTopic } from "@/lib/news-types";
import { NEWS_TOPIC_CHIPS } from "@/lib/news-types";
import { cn } from "@/lib/utils";

/** 뉴스 쪽 국내/해외 이름. `useOpenMarket`이 열린 쪽을 골라 이 값을 돌려준다. */
const NEWS_MARKET_PAIR = { domestic: "domestic", us: "us" } as const satisfies {
  domestic: NewsMarket;
  us: NewsMarket;
};

/**
 * 홈의 **실시간 뉴스** 카드.
 *
 * 「오늘의 키워드」는 2026-09-05에 여기서 나갔다 — 뉴스 레일에 같은 이름·같은
 * 엔드포인트의 카드가 따로 있어서 한 부품으로 모았다(사용자 요청 7,
 * `components/news/trending-keywords-card`). 홈은 그 부품을 그대로 쓴다.
 *
 * 이 카드는 **자기 시장 토글**을 가진다. 국내/해외를 부모가 한 번에 받으면 옆
 * 브리핑과 시장이 어긋나도 묶여 버린다.
 *
 * `articles === null`은 아직 받는 중, `[]`는 받았는데 없음이다. 둘을 같은 화면으로
 * 그리면 실패한 것과 비어 있는 것을 구분할 수 없다.
 */

/**
 * 홈 뉴스 카드가 **받는** 기사 수 = 온전한 본문으로 받을 기사 수.
 *
 * 여덟이던 것을 열넷으로 늘렸다. PC에서 이 카드는 왼쪽 브리핑 카드와 한 행에 서는데,
 * 브리핑이 문단·핵심·지켜볼 것까지 펼치느라 더 길어서 **오른쪽 아래가 비어 있었다**
 * (실측 1440px: 브리핑 535px · 뉴스 478px → 57px). 줄을 늘리고 넘치는 만큼은 카드
 * 안에서 스크롤시키면, 남는 높이가 빈칸이 아니라 **기사 몇 줄**이 된다.
 *
 * 서버에도 이 수만큼 «온전한» 기사를 달라고 한다 — 뒤쪽 요약본은 본문·링크가 없어
 * 눌러도 빈 모달이 뜬다(`fetchDashboardNews`의 `full`).
 *
 * 홈의 다른 카드도 같은 목록을 쓰므로 `export`한다 — 같은 화면에서 `full`만 다른
 * 두 요청이 나가면 같은 100건짜리 목록을 두 번 받게 된다(실측 2026-09-03: 홈 한
 * 번에 `/news/domestic/dashboard`가 `full=8`·`full=14`로 두 번). 캐시는 URL로
 * 맞추므로 **수가 같아야** 한 번으로 합쳐진다.
 *
 * **화면에 그리는 줄 수는 이것과 다르다** — 아래 `newsRowVisibilityClass` 참조.
 */
export const DASHBOARD_NEWS_ROWS = 14;

/**
 * **화면이 좁아지면 줄 수도 줄인다** — 폰 5 · 태블릿 8 · PC 14
 * (2026-09-05 사용자 요청 2, 단계는 사용자가 정했다).
 *
 * PC에서 열넷인 이유는 옆 브리핑 카드의 높이를 채우기 위해서다(위 주석). 그런데
 * 좁은 화면에서는 카드가 위아래로 쌓여 **맞출 상대가 없다** — 열넷이 그대로 남아
 * 홈 첫 화면이 뉴스 목록 하나로 채워졌다. 반대로 태블릿에서 다섯은 남는 높이를
 * 도로 비운다. 그래서 두 단이 아니라 세 단이다.
 *
 * **받는 개수는 그대로 열넷**이고 CSS로 감춘다. 이유가 둘이다:
 *
 *  - 요청 URL이 화면 폭마다 달라지면 캐시가 갈라진다. 홈의 다른 카드가 같은 목록을
 *    쓰는데(`TodayKeywordsCard`의 속보 수, 기사 모달의 「같은 이슈」 묶기), 수가
 *    어긋나는 순간 같은 100건짜리 목록을 두 번 받는다 — 위 `DASHBOARD_NEWS_ROWS`
 *    머리글이 적어 둔 그 실측이다.
 *  - 폭을 재서 개수를 정하면 서버 렌더와 첫 클라이언트 렌더가 어긋나 하이드레이션이
 *    깨진다. 그걸 피하려면 첫 프레임을 비워야 하는데, 줄 몇 개 때문에 치를 값이 아니다.
 */
const NEWS_ROWS_PHONE = 5;
const NEWS_ROWS_TABLET = 8;

/**
 * `i`번째 줄이 어느 폭부터 보이는가.
 *
 * `<li>`의 기본 display가 `list-item`이라 되살릴 때도 그것으로 되돌린다 — `block`으로
 * 되돌리면 그 줄만 목록 항목이 아니게 되고, 스크린 리더가 세는 개수가 달라진다.
 */
function newsRowVisibilityClass(index: number): string | undefined {
  if (index < NEWS_ROWS_PHONE) return undefined;
  if (index < NEWS_ROWS_TABLET) return "hidden md:list-item";
  return "hidden lg:list-item";
}

export function LatestNewsCard({
  onPick,
  onPoolChange,
  fillRow = false,
  className,
}: {
  onPick: (article: NewsArticle) => void;
  /** 모달 「같은 이슈」 묶기에 쓸, 지금 화면에 있는 온전한 기사. */
  onPoolChange?: (articles: NewsArticle[]) => void;
  /**
   * 옆 브리핑 카드의 높이를 **채우는가**(`lib/dashboard-news-layout`).
   *
   * 채울 때만 목록이 카드 안에서 스크롤된다. 혼자 서는 행에서까지 스크롤시키면
   * 전폭으로 늘어난 카드가 도로 352px에 갇힌다 — 「PC에서 다섯 줄」의 원인이었다.
   */
  fillRow?: boolean;
  className?: string;
}) {
  // 기본값은 **지금 열려 있는 시장** (2026-09-11 사용자 요청 9).
  const [market, setMarket] = useOpenMarket(NEWS_MARKET_PAIR);
  return (
    <LatestNewsCardBody
      key={market}
      market={market}
      onMarketChange={setMarket}
      onPick={onPick}
      onPoolChange={onPoolChange}
      fillRow={fillRow}
      className={className}
    />
  );
}

function LatestNewsCardBody({
  market,
  onMarketChange,
  onPick,
  onPoolChange,
  fillRow,
  className,
}: {
  market: NewsMarket | null;
  onMarketChange: (m: NewsMarket) => void;
  onPick: (article: NewsArticle) => void;
  onPoolChange?: (articles: NewsArticle[]) => void;
  fillRow?: boolean;
  className?: string;
}) {
  const [articles, setArticles] = useState<NewsArticle[] | null>(null);
  /** 국내 카테고리 칩(2026-10-09 요청 4). 해외 탭에서는 칩을 숨기고 이 값도 쓰지 않는다. */
  const [topic, setTopic] = useState<NewsTopic | "">("");
  const activeTopic = market === "domestic" ? topic : "";

  /** 언마운트 뒤 도착한 응답을 버린다 — 요청을 거는 자리가 첫 적재와 주기 둘이다. */
  const aliveRef = useRef(true);
  /** 한 번이라도 받아 본 적이 있는가 — 첫 적재의 실패와 갱신의 실패를 가른다. */
  const loadedRef = useRef(false);
  useEffect(() => {
    aliveRef.current = true;
    return () => {
      aliveRef.current = false;
    };
  }, []);

  const load = useCallback(() => {
    if (market == null) return; // 어느 시장을 받을지 아직 모른다
    fetchDashboardNews(market, DASHBOARD_NEWS_ROWS, activeTopic)
      .then((res) => {
        if (!aliveRef.current) return;
        // 뒤쪽 요약본은 칩·모달에 쓰면 빈 창이 뜬다. 본문이 온전한 앞부분만 둔다.
        const full = res.articles.slice(0, res.fullCount);
        loadedRef.current = true;
        setArticles(full);
        onPoolChange?.(full);
      })
      .catch(() => {
        // 갱신이 실패하면 **보고 있던 기사를 지우지 않는다**. 첫 적재의 실패만
        // 「표시할 뉴스가 없습니다」로 떨어진다(`lib/screen-refresh`).
        if (!aliveRef.current || loadedRef.current) return;
        setArticles([]);
        onPoolChange?.([]);
      });
    // `onPoolChange`는 부모 setState라 렌더마다 같은 참조다.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [market, activeTopic]);

  useEffect(() => {
    load();
  }, [load]);

  /*
    **5분마다 다시 받는다** (2026-09-16 사용자 요청). 「실시간 뉴스」라고 적힌 카드가
    홈을 열어 둔 동안에는 마운트 시각의 기사 열넷을 그대로 들고 있었다 — 이름이 참이
    되려면 목록도 따라와야 한다. 서버 캐시가 15초, 클라이언트 캐시가 30초라 이 주기는
    실제로 왕복한다.
  */
  useVisibleInterval(load, SCREEN_REFRESH_MS, market != null);

  const rows = useMemo(() => articles?.slice(0, DASHBOARD_NEWS_ROWS) ?? null, [articles]);

  /*
   * 이 기사가 **무엇에 관한 것인지**를 제목만으로 알 수 없다. 뉴스 화면의 카드는 이미
   * 아래에 지수·종목 칩을 달고 있는데(`NewsArticleChips`), 홈은 제목·시각만으로는
   * 「엔비디아 기사」와 「코스피 기사」가 같은 모양으로 쌓여 있었다.
   *
   * 홈에서는 **언론사 대신** 그 칩을 제목 아래 한 줄에 둔다 — 칩을 별도 줄로 내리면
   * 옆 브리핑 카드와 높이를 맞추느라 빈칸이 커진다. 시세는 **이 카드가 들고 있는
   * 열넷**의 종목만 부른다 — 홈은 100건을 받아 두지만 나머지 86건은 칩이 뜰 자리가
   * 없다. 좁은 화면에서 감춘 줄까지 세는 이유는, 감춤이 CSS라 그 줄의 칩도 DOM에
   * 있고 요청은 어차피 배치 하나이기 때문이다(`useSymbolQuotes`).
   */
  const chipSymbols = useMemo(() => (rows ? allArticleSymbols(rows) : []), [rows]);
  const quotes = useSymbolQuotes(chipSymbols);
  const { indices } = useMarketIndices();

  return (
    <SectionCard
      title="실시간 뉴스"
      href={market === "us" ? "/news/us" : `/news/domestic${activeTopic ? `?topic=${activeTopic}` : ""}`}
      hrefLabel="뉴스"
      className={className}
      /*
        PC에서 **옆에 카드가 설 때만** 그 높이를 채운다. 목록이 남는 높이보다 길면
        카드 안에서 스크롤되므로, 브리핑이 짧은 날에도 이 카드가 행을 혼자 늘리지
        않는다. `min-h-0`이 없으면 flex 자식이 내용 높이 아래로 줄지 못해 스크롤이
        생기지 않는다. 모바일은 위아래로 쌓이므로 맞출 상대가 없다 — 자연 높이 그대로다.

        브리핑이 없는 날에는 이 클래스가 아예 붙지 않는다(`fillRow`). 붙여 두면 옆이
        빈 채로 칸의 바닥값 352px에 갇혀 열넷 중 다섯 줄만 보였다(2026-09-05).
      */
      bodyClassName={fillRow ? "lg:min-h-0 lg:flex-1 lg:overflow-y-auto" : undefined}
      action={
        <MiniToggle
          options={[
            { id: "domestic" as const, label: "국내" },
            { id: "us" as const, label: "해외" },
          ]}
          value={market}
          onChange={onMarketChange}
        />
      }
    >
      {market === "domestic" && (
        <div className="-mx-1 mb-1.5 flex min-w-0 gap-1 overflow-x-auto px-1 pb-0.5" role="group" aria-label="뉴스 카테고리">
          {NEWS_TOPIC_CHIPS.map((c) => (
            <button
              key={c.id || "all"}
              type="button"
              aria-pressed={topic === c.id}
              onClick={() => {
                setArticles(null);
                setTopic(c.id);
              }}
              className={cn(
                "shrink-0 rounded-full border px-2 py-0.5 text-micro transition-colors",
                topic === c.id
                  ? "border-[var(--brand)] bg-[var(--brand)] font-semibold text-white"
                  : "border-[var(--border)] text-[var(--text-secondary)] hover:text-[var(--text-primary)]",
              )}
            >
              {c.label}
            </button>
          ))}
        </div>
      )}
      {rows === null ? (
        <SectionSkeleton rows={8} />
      ) : rows.length === 0 ? (
        <SectionEmpty text="표시할 뉴스가 없습니다" />
      ) : (
        <ul className="min-w-0 space-y-0.5">
          {rows.map((n, i) => (
            <li key={n.id} className={newsRowVisibilityClass(i)}>
              {/*
                칩이 링크라 **제목만** 버튼이다 (2026-09-06).

                예전에는 제목과 메타 줄을 통째로 감싼 `div`가 `role="button"`이었다.
                「버튼 안의 링크는 무효」라서 `<button>`을 피한 것인데, `role="button"`
                이어도 같은 위반이다(axe `nested-interactive`) — 태그만 바뀌었을 뿐
                보조기술에는 똑같이 「버튼 안에 링크가 있다」로 보인다.

                그래서 칩을 클릭 영역 **밖으로** 낸다. 기사를 여는 것은 제목이고,
                칩은 그 옆에 나란한 링크다. 호버 강조는 바깥 껍데기가 계속 들고 있어
                한 덩어리로 보인다.
              */}
              <div className="rounded-md px-1 py-1.5 hover:bg-accent">
                <button
                  type="button"
                  onClick={() => onPick(n)}
                  className="block w-full cursor-pointer bg-transparent p-0 text-left focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-[var(--brand)]"
                >
                  <p className="line-clamp-2 text-cell leading-snug">{n.headlineKo || n.headline}</p>
                </button>
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
