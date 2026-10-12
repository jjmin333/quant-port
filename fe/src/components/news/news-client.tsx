"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useSearchParams } from "next/navigation";
import { Check, RefreshCw, X } from "lucide-react";
import type { NewsArticle, NewsMarket, NewsSort, NewsSourceId, NewsTopicFilter } from "@/lib/news-types";
import { NEWS_TOPIC_CHIPS, parseNewsTopicFilter } from "@/lib/news-types";
import { getMockNews } from "@/lib/mock-news";
import {
  fetchDomesticNews,
  fetchTopSymbols,
  fetchUsNews,
  type NewsSymbolCount,
} from "@/lib/news-api";
import { fetchNewsBriefing, type NewsMarketBriefing } from "@/lib/news-briefing-api";
/*
 * 지수 필터 라이브러리(news-index-filter · news-us-index-filter ·
 * news-dashboard-indices)는 **여기서 더 이상 쓰지 않는다.** 이 화면의 코스피·코스닥·
 * 환율 pill과 지수 카드가 사라졌기 때문이다. 파일 자체는 지운 것이 아니라 —
 * 뉴스 모달의 「관련 지수」 카드와 홈이 계속 쓴다.
 */
import { NEWS_BREAKING_LABEL } from "@/lib/news-breaking";
import { selectBreakingArticles } from "@/lib/news-breaking-filter";
import { dedupeNewsArticles } from "@/lib/news-dedupe";
import { allArticleSymbols } from "@/lib/news-derive";
import { useMySymbols, useSymbolQuotesState } from "@/hooks/use-news-symbols";
import { useMarketIndices } from "@/hooks/use-market-indices";
import { useVisibleInterval } from "@/hooks/use-visible-interval";
import { NEWS_REFRESH_MS } from "@/lib/screen-refresh";
import { useHeaderSearch } from "@/components/providers/header-search-provider";
import { PageScrollShell } from "@/components/layout/page-content-shell";
import { PageColumns } from "@/components/layout/page-columns";
import { NewsSideRail } from "@/components/news/news-side-rail";
import { NewsBreakingTicker } from "@/components/news/news-breaking-ticker";
import { NewsFeed } from "@/components/news/news-feed";
import { NewsBriefingCard } from "@/components/news/news-briefing-card";
import { NewsIssueStrip } from "@/components/news/news-issue-strip";
import { useNewsArticleModal } from "@/components/news/news-article-modal-host";
import { NewsSampleSymbolsBanner } from "@/components/news/news-login-gate";
import { NewsSortRow } from "@/components/news/news-sort-row";
import { cn } from "@/lib/utils";
import { toUserMessage } from "@/lib/error-message";
import { useUrlParamState } from "@/hooks/use-url-param-state";
import { LoadErrorState } from "@/components/common/load-error-state";

type LiveNewsMarket = Extract<NewsMarket, "domestic" | "us">;

/*
 * 화면 문구. `subtitle`과 `emptyBreaking`은 각각 제목 블록·속보 pill과 함께
 * 사라져서 남은 것은 하나뿐이다 — 종목 카드를 눌렀는데 기사가 없을 때의 안내.
 */
const PAGE_COPY: Record<LiveNewsMarket, { emptyFilter: string }> = {
  domestic: {
    emptyFilter: "관련 기사가 없습니다. 위 카드를 한 번 더 누르면 전체 목록으로 돌아갑니다.",
  },
  us: {
    emptyFilter: "관련 기사가 없습니다. 위 카드를 한 번 더 누르면 전체 목록으로 돌아갑니다.",
  },
};

function isLiveNewsMarket(market: NewsMarket): market is LiveNewsMarket {
  return market === "domestic" || market === "us";
}

type NewsClientProps = {
  market: NewsMarket;
  /**
   * 서버가 미리 받아 둔 기사 목록(기본 정렬 기준). 크롤러가 받는 첫 HTML에
   * 기사 제목이 실리게 하는 것이 목적이다 — 특히 자바스크립트를 사실상 돌리지
   * 않는 네이버 Yeti에게는 이것이 유일하게 보이는 내용이다.
   *
   * 주소에 정렬·검색 파라미터가 붙어 있으면 부르는 쪽이 비워서 넘긴다(그때는
   * 서버가 미리 받은 목록이 틀린 목록이다).
   */
  initialArticles?: NewsArticle[];
};

export function NewsClient({ market, initialArticles = [] }: NewsClientProps) {
  const { activeQuery, isSearchActive, mode } = useHeaderSearch();
  const searchParams = useSearchParams();
  const newsSearchActive =
    isSearchActive &&
    ((market === "domestic" && mode === "news-domestic") ||
      (market === "us" && mode === "news-us"));

  const [sourceId, setSourceId] = useState<NewsSourceId | "all">("all");
  /** 국내 카테고리(2026-10-09 요청 4). `?topic=`으로 들어오면 그 칩부터(홈 카드에서 「더 보기」). */
  const [topic, setTopic] = useState<NewsTopicFilter>(() =>
    market === "domestic" ? parseNewsTopicFilter(searchParams.get("topic")) ?? "" : "",
  );
  /*
   * 목록을 좁히는 길이 **한 벌**이 됐다.
   *
   * 예전에는 세 벌이었다 — 상단 카드(지수·종목), 그 아래 pill 줄(전체·속보·코스피·
   * 코스닥 + 오늘 많이 언급된 종목 셋), 그리고 헤더 검색. 앞의 둘은 **같은 종목이
   * 같은 화면에 두 번** 서는 구조였다: SK하이닉스가 카드로도 pill로도 있어, 어느
   * 쪽을 눌러야 하는지 알 수 없었다(첨부 화면 실측).
   *
   * 지금 종목·이슈로 좁히는 일은 **상단 카드만** 한다. 여기 남은 것은 성격이 다른
   * 것 하나 — 「내 종목」은 집계가 아니라 로그인한 사람의 목록이라 카드가 될 수 없다.
   */
  const [myOnly, setMyOnly] = useState(false);
  /*
   * 「속보만」 (2026-09-05 사용자 요청 5).
   *
   * 정렬 옆에 서지만 **정렬이 아니다** — 목록을 좁힌다. 그래서 모양도 최신순·인기순의
   * 얇은 칩이 아니라 「내 종목」과 같은 pill을 쓴다.
   *
   * 판정은 목록·카드·티커가 이미 함께 쓰는 규칙 하나다(`isArticleBreakingNews`):
   * 제목의 「[속보]」나 `속보` 태그 **그리고** 최근 것. 여기서 조건을 새로 쓰면
   * 카드에는 속보 배지가 붙었는데 필터에는 안 걸리는 기사가 생긴다.
   */
  const [breakingOnly, setBreakingOnly] = useState(false);
  /*
   * `?keyword=`로 들어오면 그 키워드로 좁힌 채 연다.
   *
   * 홈의 「오늘의 키워드」가 이 링크를 쓴다. 홈에는 기사가 여덟 건뿐이라 거기서 좁혀 봐야
   * 볼 것이 없고, 좁히는 일은 목록이 다 있는 이 화면이 해야 한다. 초기값으로만 읽는다 —
   * 이후 칩을 끄고 켜는 것은 화면 안의 상태지 주소가 따라다닐 일이 아니다.
   */
  const [keywordFilter, setKeywordFilter] = useState<string | null>(
    () => searchParams.get("keyword")?.trim() || null,
  );
  const [symbolFilter, setSymbolFilter] = useState<string | null>(null);
  const [sampleBannerHidden, setSampleBannerHidden] = useState(false);
  /*
    정렬은 주소에 싣는다 (§C-2) — 새로고침·뒤로가기에도 유지되고, 「인기순」으로 본
    목록을 그대로 공유할 수 있다.

    나머지 필터(출처·키워드·종목)는 싣지 않는다. 전부 주소에 붙이면 링크가 상태를
    설명하기는커녕 읽을 수 없게 되고, 그것들은 한 번 보고 마는 탐색용이라 돌아올
    값이 아니다. 돌아올 만한 값(카테고리·정렬)만 남긴다.
  */
  const [sort, setSort] = useUrlParamState<NewsSort>(
    "sort",
    "latest",
    (v): v is NewsSort => v === "latest" || v === "popular",
  );
  const [refreshKey, setRefreshKey] = useState(0);
  const [refreshing, setRefreshing] = useState(false);
  const [articles, setArticles] = useState<NewsArticle[]>(initialArticles);
  const [totalCount, setTotalCount] = useState(initialArticles.length);
  const [loadError, setLoadError] = useState<string | null>(null);
  // 서버가 이미 기사를 줬으면 「불러오는 중」으로 시작하지 않는다.
  const [loading, setLoading] = useState(isLiveNewsMarket(market) && initialArticles.length === 0);
  /** 서버가 준 목록으로 시작했는가 — 아래 첫 fetch가 스피너를 건너뛸지 가른다. */
  const hydratedFromServer = useRef(initialArticles.length > 0);
  /**
   * **주기 갱신인 회차의 `refreshKey`** (2026-09-16 사용자 요청).
   *
   * 새로고침 단추가 올린 회차와 90초 시계가 올린 회차는 같은 이펙트 셋을 타지만 요구가
   * 반대다 — 사람이 누른 쪽은 「지금 다시 받는 중」이 보여야 하고, 시계가 올린 쪽은
   * **아무 일도 없었던 것처럼** 지나가야 한다. 90초마다 목록이 스켈레톤으로 지워졌다
   * 돌아오면 읽던 자리를 잃고, 사용자는 자기가 뭘 눌렀다고 읽는다.
   *
   * 불리언이 아니라 **회차 번호**를 드는 이유는 이 표식을 읽는 이펙트가 셋(목록·
   * 브리핑·언급 종목)이기 때문이다. 먼저 도는 이펙트가 불리언을 내려 버리면 나머지
   * 둘은 그 회차가 조용한 회차였다는 것을 알 수 없다.
   *
   * 상태가 아니라 ref인 이유는 렌더에 쓰이지 않아서다 — 이펙트만 읽는다.
   */
  const silentRefreshKey = useRef<number | null>(null);
  const [briefing, setBriefing] = useState<NewsMarketBriefing | null>(null);
  /*
   * 「많이 언급된 종목」 집계. **읽는 곳은 한참 아래지만 선언은 여기 있어야 한다.**
   *
   * 예전에는 이 상태가 자기 `useEffect` 바로 위(그러니까 다른 `useEffect` 세 개
   * 뒤)에 있었다. 컴포넌트 안에서 `useState`가 이펙트보다 뒤에 오는 유일한
   * 자리였고, 그 파일을 편집할 때마다 Fast Refresh가 훅 목록을 갈아끼우면서
   * **`Cannot access 'mentions' before initialization`** 을 냈다 — 오류는 엉뚱하게
   * 위쪽 이펙트의 의존성 배열(241행)을 가리켜서 소스만 봐서는 원인이 안 보인다.
   * 콜드 로드에서는 재현되지 않고 HMR을 몇 번 거친 탭에서만 나온다.
   *
   * 무엇을 받아 오는지는 아래 `fetchTopSymbols` 이펙트의 주석에 적어 두었다.
   */
  const [mentions, setMentions] = useState<NewsSymbolCount[]>([]);

  const liveMarket: LiveNewsMarket = market === "us" ? "us" : "domestic";
  const mySymbols = useMySymbols(liveMarket);

  const mockArticles = useMemo(
    () => getMockNews({ market, sourceId, sort }),
    [market, sourceId, sort],
  );

  const pageCopy = isLiveNewsMarket(market) ? PAGE_COPY[market] : PAGE_COPY.domestic;

  /*
   * 시장을 바꾸면 필터를 전부 되돌린다 — 국내에서 걸어 둔 「반도체」가 해외 목록에
   * 그대로 남아 있으면 빈 화면의 이유를 알 수 없다.
   *
   * **마운트에서는 돌지 않는다.** 첫 실행까지 초기화하면 `?keyword=`로 들어온 값을
   * 화면이 뜨자마자 스스로 지운다.
   */
  const prevMarket = useRef(market);
  useEffect(() => {
    if (prevMarket.current === market) return;
    prevMarket.current = market;
    setMyOnly(false);
    setSourceId("all");
    setKeywordFilter(null);
    setSymbolFilter(null);
  }, [market]);

  useEffect(() => {
    if (!isLiveNewsMarket(market)) {
      return;
    }

    let cancelled = false;
    /*
     * 서버가 준 목록을 들고 시작한 **첫 실행에서는 화면을 비우지 않는다.**
     * 그냥 `setLoading(true)`를 하면 방금 서버가 찍어 준 기사들이 스피너로
     * 한 번 지워졌다가 돌아온다 — 미리 그린 보람이 없다. 두 번째 실행부터는
     * (정렬을 바꿨거나 새로고침을 눌렀거나) 평소대로 로딩을 보여 준다.
     */
    const silent = silentRefreshKey.current === refreshKey;
    const skipSpinner = hydratedFromServer.current || silent;
    hydratedFromServer.current = false;
    queueMicrotask(() => {
      if (!cancelled && !skipSpinner) {
        setLoading(true);
        setLoadError(null);
      }
    });

    const fetchNews = market === "domestic" ? fetchDomesticNews : fetchUsNews;
    fetchNews({
      market,
      sourceId,
      sort,
      topic: market === "domestic" ? topic : "",
      query: newsSearchActive ? activeQuery : undefined,
    })
      .then(({ articles: items, totalCount: count }) => {
        if (!cancelled) {
          setArticles(items);
          setTotalCount(count);
        }
      })
      .catch((err: unknown) => {
        if (cancelled) return;
        /*
          **갱신의 실패는 화면을 비우지 않는다.** 첫 적재의 실패는 「못 불러왔다」가
          맞지만, 90초짜리 시계가 올린 회차가 한 번 실패했다고 읽고 있던 기사 예순
          건을 지우면 낡은 목록을 고치러 와서 목록을 없애는 일이 된다 —
          다음 회차가 90초 뒤에 다시 채운다(`lib/screen-refresh`).
        */
        if (silent) return;
        setLoadError(toUserMessage(err, "뉴스를 불러오지 못했습니다"));
        setArticles([]);
        setTotalCount(0);
      })
      .finally(() => {
        if (!cancelled) {
          setLoading(false);
        }
      });

    return () => {
      cancelled = true;
    };
  }, [market, sourceId, sort, topic, refreshKey, activeQuery, newsSearchActive]);

  /*
   * 브리핑은 **저장된 것을 읽기만** 한다 — 생성은 서버 배치의 몫이다(화면이 열릴 때
   * 만들면 하루 LLM 비용이 방문자 수에 비례한다). 그래서 실패해도 조용히 넘어가고,
   * 아직 생성된 적 없으면 카드를 아예 그리지 않는다.
   */
  useEffect(() => {
    if (!isLiveNewsMarket(market)) return;
    let cancelled = false;
    fetchNewsBriefing(market)
      .then((data) => {
        if (!cancelled) setBriefing(data);
      })
      .catch(() => {
        // 실패한 갱신은 직전 글을 남긴다 — 위 목록과 같은 규칙(`lib/screen-refresh`).
        if (!cancelled && silentRefreshKey.current !== refreshKey) setBriefing(null);
      });
    return () => {
      cancelled = true;
    };
  }, [market, refreshKey]);

  const baseArticles = useMemo(() => {
    const items = isLiveNewsMarket(market) ? articles : mockArticles;
    return market === "us" ? dedupeNewsArticles(items) : items;
  }, [market, articles, mockArticles]);

  /*
   * 「많이 언급된 종목」은 **서버가 최근 1,000건에서 센 것**을 받는다 (#9).
   *
   * 예전에는 `baseArticles`(최신 100건)를 셌다. 그런데 그 100건이 덮는 시간이
   * 34분이라(실측 2026-09-01: 16:14~16:47) 「오늘 많이 언급된 종목」이 사실
   * 「지난 30분」이었고, 새로고침마다 순위가 뒤집혔다. 표본을 늘리려고 목록을 더
   * 받는 건 답이 아니다 — 「오늘의 키워드」가 같은 이유로 이미 서버에 있다.
   *
   * 응답에 종목명이 함께 오고(#6), 국내 시장이면 서버가 해외 티커를 걸러 준다(#4).
   * 목록 필터(정렬·출처)와 무관한 **고정된 모수**라 `refreshKey`에만 다시 부른다.
   *
   * 상태 선언은 위 상태 블록에 있다 — 이 자리에 두면 `useEffect` 세 개 뒤에
   * `useState`가 오는 모양이 되고, 그 상태로 Fast Refresh가 훅 목록을 갈아끼울 때
   * 「Cannot access 'mentions' before initialization」이 났다.
   */
  useEffect(() => {
    if (!isLiveNewsMarket(market)) return;
    let cancelled = false;
    fetchTopSymbols(liveMarket, 6)
      .then((res) => {
        if (!cancelled) setMentions(res.symbols);
      })
      .catch(() => {
        // 갱신의 실패는 스트립을 비우지 않는다 — 위 둘과 같은 규칙.
        if (!cancelled && silentRefreshKey.current !== refreshKey) setMentions([]);
      });
    return () => {
      cancelled = true;
    };
  }, [market, liveMarket, refreshKey]);

  /*
   * 화면에 있는 기사의 종목 + 내 종목 = 시세를 받아야 할 목록.
   *
   * 이슈 스트립·행 종목 칩·레일 「내 종목 뉴스」·모달 관련 종목이 **이 한 요청을
   * 공유**한다. 각자 부르면 같은 화면에서 배치 요청이 네 번 나간다.
   */
  const quoteSymbols = useMemo(() => {
    const set = new Set(allArticleSymbols(baseArticles));
    for (const sym of mySymbols.symbols) set.add(sym);
    // 스트립의 언급 종목은 서버 집계에서 오므로 기사 60건 상한 밖일 수 있다.
    // 이름은 이미 붙어 있지만 등락률은 여기 없으면 영영 「—」로 남는다.
    for (const row of mentions) set.add(row.symbol);
    return [...set];
  }, [baseArticles, mySymbols.symbols, mentions]);
  const { quotes, loading: quotesLoading } = useSymbolQuotesState(quoteSymbols);
  const { indices } = useMarketIndices();

  const usingMySymbols = myOnly;

  /** 속보 필터를 걸기 **직전**의 목록. 속보 건수를 이 위에서 센다. */
  const filteredArticles = useMemo(() => {
    let items = baseArticles;

    if (!newsSearchActive && isLiveNewsMarket(market) && usingMySymbols) {
      const wanted = new Set(mySymbols.symbols.map((s) => s.toUpperCase()));
      items = items.filter((article) =>
        (article.symbols ?? []).some((s) => wanted.has(s.trim().toUpperCase())),
      );
    }

    if (keywordFilter) {
      items = items.filter((article) => (article.keywords ?? []).includes(keywordFilter));
    }
    if (symbolFilter) {
      items = items.filter((article) =>
        (article.symbols ?? []).some((s) => s.trim().toUpperCase() === symbolFilter),
      );
    }
    return items;
  }, [
    baseArticles,
    usingMySymbols,
    mySymbols.symbols,
    keywordFilter,
    symbolFilter,
    market,
    newsSearchActive,
  ]);

  /*
   * 속보는 **시간 창이 있다**(`BREAKING_MAX_AGE_MS`) — 조용한 날에는 0건이 정상이다.
   * 그래서 건수를 칩에 적고, 0이면 누르지 못하게 잠근다: 눌러도 빈 목록만 나오는
   * 필터는 고장으로 보인다(실적 화면의 「내 종목만」과 같은 규칙).
   *
   * 켜 둔 채로 목록이 새로 와 속보가 0이 되면 `breakingActive`가 스스로 꺼진다 —
   * 이펙트로 상태를 되돌리면 빈 목록이 한 프레임 그려진다.
   */
  const breakingArticles = useMemo(
    () => selectBreakingArticles(filteredArticles),
    [filteredArticles],
  );
  const breakingCount = breakingArticles.length;
  const breakingActive = breakingOnly && breakingCount > 0;

  const displayArticles = breakingActive ? breakingArticles : filteredArticles;

  const latestTickerArticle = useMemo(() => {
    if (!isLiveNewsMarket(market) || newsSearchActive || usingMySymbols) {
      return null;
    }
    // 지수별로 나눠 보던 배너가 없어졌으니 그냥 가장 새 기사다.
    return baseArticles[0] ?? null;
  }, [baseArticles, usingMySymbols, market, newsSearchActive]);

  const activeFilterLabel = usingMySymbols ? "내 종목" : "전체";

  const handleRefresh = useCallback(() => {
    setRefreshing(true);
    setRefreshKey((k) => k + 1);
    window.setTimeout(() => setRefreshing(false), 400);
  }, []);

  /*
    **90초마다 스스로 새로고침한다** (2026-09-16 · 2026-09-17 사용자 요청, `lib/screen-refresh`).
    주기가 5분에서 내려온 이유는 `NEWS_REFRESH_MS` 머리글에 있다 — 수집을 아무리
    빠르게 해도 이 시계가 느리면 그만큼이 그대로 지연으로 얹힌다.

    이 화면은 「실시간」을 내걸고도 마운트 시각의 기사 목록을 들고 서 있었다. 새 기사는
    수집 배치가 쉬지 않고 밀어 넣는데, 탭을 열어 둔 사람에게는 그게 하나도 오지 않았다 —
    새로고침 단추를 아는 사람만 최신을 봤다.

    단추와 **같은 문**으로 들어간다(`refreshKey`). 목록·브리핑·언급 종목 셋이 이미
    그 키에 매달려 있어서, 여기서 요청을 따로 걸면 같은 일을 하는 코드가 두 벌이 된다.
    다른 것은 `silentRefreshKey` 하나뿐 — 스피너를 띄우지 않고 실패해도 화면을 비우지
    않는다.

    안 보이는 탭에서는 돌지 않고, 다시 보이는 순간 한 주기가 지났으면 한 번 메운다
    (`useVisibleInterval`). 예시 목록을 보는 중이면(`isLiveNewsMarket`이 false) 부를
    서버가 없으므로 아예 켜지 않는다.
  */
  useVisibleInterval(
    () => {
      /*
        표식을 먼저 세우고 회차를 올린다. 갱신 함수(`setRefreshKey((k) => …)`) 안에서
        ref를 건드리면 StrictMode가 그 함수를 두 번 부르며 순수성 규칙에 걸린다 —
        `useVisibleInterval`이 콜백을 렌더마다 ref에 다시 담으므로 여기서 읽는
        `refreshKey`는 언제나 지금 값이다.
      */
      const next = refreshKey + 1;
      silentRefreshKey.current = next;
      setRefreshKey(next);
    },
    NEWS_REFRESH_MS,
    isLiveNewsMarket(market),
  );

  /*
   * 셸 선택·문맥 블록·조회 기록은 `useNewsArticleModal`이 맡는다. 이 화면이 그 규칙을
   * 혼자 들고 있던 동안 홈·지수·종목 화면은 iframe 셸 하나를 문맥 없이 띄웠다.
   *
   * 목록이 이미 배치로 받아 둔 시세를 넘겨 관련 종목 패널의 추가 요청을 0으로 만든다.
   */
  const { open: openArticle, modal: modals } = useNewsArticleModal({
    pool: baseArticles,
    quotes,
  });

  /*
   * 로그인하고 돌아왔을 때 **보던 기사를 다시 연다**(`?article=`). 그러지 않으면
   * 기껏 로그인하고 목록 맨 위에 떨어진다 — 액션 버튼에서 로그인으로 보낼 때 이
   * 파라미터를 실어 보낸다.
   */
  const pendingArticleId = searchParams.get("article");
  useEffect(() => {
    if (!pendingArticleId || baseArticles.length === 0) return;
    const target = baseArticles.find((a) => a.id === pendingArticleId);
    if (target) openArticle(target);
    // 한 번 열고 나면 파라미터를 지운다 — 새로고침마다 다시 열리지 않게.
    window.history.replaceState(null, "", window.location.pathname);
  }, [pendingArticleId, baseArticles, openArticle]);

  if (!isLiveNewsMarket(market)) {
    return (
      <>
        <PageScrollShell contentClassName="space-y-4">
          <NewsFeed
            articles={displayArticles}
            onArticleClick={openArticle}
            quotes={quotes}
            quotesLoading={quotesLoading}
            indices={indices}
          />
        </PageScrollShell>
        {modals}
      </>
    );
  }

  const sampleNames = mySymbols.symbols
    .map((sym) => quotes.get(sym)?.name || sym)
    .slice(0, 3);
  const showSampleBanner =
    usingMySymbols && !mySymbols.personal && !mySymbols.loading && !sampleBannerHidden;

  return (
    <>
      <PageScrollShell bare>
        <div className="page-shell-wide min-w-0 py-6 sm:py-8">
          <PageColumns
            rail={
              <NewsSideRail
                market={market}
                articles={baseArticles}
                mySymbols={mySymbols.symbols}
                mySymbolsPersonal={mySymbols.personal}
                quotes={quotes}
                onArticleClick={openArticle}
                onKeywordSelect={(kw) => setKeywordFilter((cur) => (cur === kw ? null : kw))}
              />
            }
          >
            {/*
              **제목 블록이 없다.** 다른 화면은 2026-09-02에 이미 걷어냈는데
              (page-content-shell.tsx의 PageActions 주석) 이 화면만 자기 헤더를
              따로 들고 있어 「뉴스」 한 줄과 부제가 남아 있었다 — 왼쪽 메뉴에서
              「국내 주식」을 눌러 들어온 사람에게 여기가 뉴스라고 다시 말하느라
              첫 기사를 그만큼 아래로 밀었다. 새로고침 버튼은 아래 필터 줄이 받는다.
            */}
            <div className="@container/news min-w-0">
              {!loading && !newsSearchActive && (
                <div className="space-y-3">
                  <NewsBreakingTicker
                    article={latestTickerArticle}
                    badgeLabel={`${activeFilterLabel} 최신`}
                  />
                  <NewsBriefingCard briefing={briefing} />
                  {/*
                    **모바일에서도 그린다.** 예전에 좁은 화면에서 이 스트립을 숨긴
                    이유는 종목을 고르는 길이 두 벌이었기 때문이다 — 카드 아홉 장이
                    375px에서 299px을 먹는데 바로 아래 pill 줄이 같은 일을 했다.
                    지금은 pill 줄에 종목이 없어 이것이 유일한 길이고, 카드도 지수를
                    덜어내고 두 줄 높이로 줄였다(news-issue-strip.tsx).
                  */}
                  <NewsIssueStrip
                    mentions={mentions}
                    quotes={quotes}
                    activeSymbol={symbolFilter}
                    onSelect={(symbol) => setSymbolFilter((cur) => (cur === symbol ? null : symbol))}
                  />
                </div>
              )}

              {!newsSearchActive && (
                <div className="mt-3 space-y-2">
                  {market === "domestic" && (
                    <div
                      className="-mx-1 flex min-w-0 gap-1.5 overflow-x-auto px-1 pb-0.5"
                      role="group"
                      aria-label="뉴스 카테고리"
                    >
                      {NEWS_TOPIC_CHIPS.map((c) => (
                        <Pill key={c.id || "all"} active={topic === c.id} onClick={() => setTopic(c.id)}>
                          {c.label}
                        </Pill>
                      ))}
                    </div>
                  )}
                  {/*
                    **버튼으로 남은 것은 셋뿐이다.**

                    코스피·코스닥·환율·속보는 여기서 빠졌다. 코드에 박아 둔 값이라
                    「오늘」과 무관한데, 바로 위 카드 줄이 매일 실제로 화제인 것을
                    보여 주는 자리라 같은 줄에 서면 어느 쪽이 오늘의 이야기인지
                    흐려졌다. 무엇보다 SK하이닉스가 카드로도 pill로도 있어 같은 종목이
                    같은 화면에 두 번 섰다.

                    남은 셋은 카드가 될 수 없는 것들이다 — 「내 종목」은 집계가 아니라
                    로그인한 사람의 목록이고, 최신순·인기순은 좁히는 게 아니라 **같은
                    목록에 던지는 다른 질문**이다.
                  */}
                  <div className="flex flex-wrap items-center gap-1.5">
                    <Pill active={usingMySymbols} onClick={() => setMyOnly((v) => !v)}>
                      ⭐ 내 종목
                      {!mySymbols.personal && !mySymbols.loading && (
                        <span className="ml-1 text-micro opacity-80">예시</span>
                      )}
                    </Pill>
                    <span className="mx-0.5 h-4 w-px bg-[var(--border)]" aria-hidden />
                    <NewsSortRow sort={sort} onSortChange={setSort} />
                    <span className="mx-0.5 h-4 w-px bg-[var(--border)]" aria-hidden />
                    <Pill
                      active={breakingActive}
                      disabled={breakingCount === 0}
                      title={
                        breakingCount === 0
                          ? "지금 목록에 속보가 없습니다"
                          : `속보 ${breakingCount}건만 봅니다`
                      }
                      onClick={() => setBreakingOnly((v) => !v)}
                    >
                      🚨 {NEWS_BREAKING_LABEL}
                      <span className="ml-1 font-num text-micro tabular-nums opacity-80">
                        {breakingCount}
                      </span>
                    </Pill>
                    {/* 새로고침은 제목 블록과 함께 사라진 자리를 대신해 이 줄 끝에 선다. */}
                    <button
                      type="button"
                      aria-label="새로고침"
                      onClick={handleRefresh}
                      className="ml-auto flex h-7 w-7 shrink-0 items-center justify-center rounded-lg border border-[var(--border)] text-[var(--text-secondary)] transition-colors hover:bg-[var(--surface-elevated)] hover:text-[var(--text-primary)]"
                    >
                      <RefreshCw className={cn("h-3.5 w-3.5", refreshing && "animate-spin")} />
                    </button>
                  </div>

                  {showSampleBanner && (
                    <NewsSampleSymbolsBanner
                      sampleNames={sampleNames}
                      onDismiss={() => setSampleBannerHidden(true)}
                    />
                  )}
                </div>
              )}

              <div className="mt-6 flex items-end justify-between gap-3 border-b border-[var(--border)] pb-3">
                <h2 className="flex min-w-0 flex-wrap items-center gap-2 text-section font-bold text-[var(--text-primary)]">
                  {newsSearchActive
                    ? `「${activeQuery}」 검색 결과`
                    : usingMySymbols
                      ? "내 종목 뉴스"
                      : market === "us"
                        ? "해외 뉴스"
                        : "국내 뉴스"}
                  {symbolFilter && (
                    <FilterChip
                      label={quotes.get(symbolFilter)?.name || symbolFilter}
                      onClear={() => setSymbolFilter(null)}
                    />
                  )}
                  {keywordFilter && (
                    <FilterChip label={`#${keywordFilter}`} onClear={() => setKeywordFilter(null)} />
                  )}
                </h2>
                <span className="shrink-0 text-cell text-[var(--text-muted)]">
                  {loading ? "불러오는 중…" : `${displayArticles.length}건`}
                </span>
              </div>

              {!newsSearchActive && !loading && (
                <p className="mt-1 text-cell text-[var(--text-muted)]">
                  전체 {totalCount.toLocaleString("ko-KR")}건 · 화면 {displayArticles.length}건
                </p>
              )}

              {/*
                실패에는 **되돌아갈 길**을 함께 준다 (§BUG-11). 예전에는 빨간 글씨 한 줄이
                전부였고, 그 글씨마저 서버 원문(`{"error":"internal"}`)이었다.
              */}
              {loadError && (
                <LoadErrorState
                  compact
                  className="mt-2"
                  message={loadError}
                  retrying={loading}
                  onRetry={() => setRefreshKey((k) => k + 1)}
                />
              )}

              {!loading && displayArticles.length === 0 ? (
                <p className="px-2 py-12 text-center text-sm text-[var(--text-muted)]">
                  {newsSearchActive
                    ? "검색 결과가 없습니다. 다른 키워드를 입력해 보세요."
                    : breakingActive
                    ? "지금 조건에 맞는 속보가 없습니다."
                    : usingMySymbols
                      ? "내 종목이 언급된 기사가 아직 없습니다."
                      : keywordFilter
                        ? `「${keywordFilter}」 키워드와 현재 필터를 함께 만족하는 기사가 없습니다.`
                        : symbolFilter
                          ? pageCopy.emptyFilter
                          : "표시할 뉴스가 없습니다. 잠시 후 새로고침해 주세요."}
                </p>
              ) : (
                <NewsFeed
                  articles={displayArticles}
                  onArticleClick={openArticle}
                  quotes={quotes}
                  quotesLoading={quotesLoading}
                  indices={indices}
                  /* 인기순은 시간순이 아니라 시간 구분선이 거짓말이 된다. */
                  grouped={sort === "latest" && !newsSearchActive}
                />
              )}
            </div>
          </PageColumns>
        </div>
      </PageScrollShell>

      {modals}
    </>
  );
}

/*
켜진 pill은 **손가락이 올라간 채로도** 켜져 보여야 한다 (2026-09-11 사용자 요청 8).

증상은 「속보를 눌러도 눌린 것처럼 안 보인다」였다. 실측하니 상태는 옳았다 —
`aria-pressed`는 true였고 `bg-[var(--brand)]`도 붙어 있었다. 그런데 커서를 치우면
파랗고, 올려 두면 회색이었다: `buttonVariants({variant:"outline"})`가 싣고 오는
`hover:bg-accent`·`hover:text-accent-foreground`가 같은 명시도에 더 나중이라 브랜드
칠을 덮는다. 그리고 **누른 직후가 바로 그 상태다.** 터치에서는 hover가 탭 뒤에도
남으므로 아예 계속 안 눌린 것처럼 보인다.

그래서 켜진 쪽은 hover도 같은 색으로 못 박고, 색 하나에만 기대지 않도록 체크
표시를 함께 그린다 — 색 대비만으로는 옆 pill과의 차이가 작다.

2026-10-12: 다크 모드에서도 같은 일이 났다 — outline의 `dark:bg-input/30`·`dark:border-input`이
브랜드 칠·테두리를 덮어, 켜진 pill이 어두운 바탕에 1px 그림자 테두리만 남았고 그 그림자는
가로 스크롤 줄(overflow)에 위아래가 잘렸다. 그래서 `buttonVariants`를 쓰지 않고 색을 직접 둔다.
*/
function Pill({
  active,
  disabled = false,
  title,
  onClick,
  children,
}: {
  active: boolean;
  /** 눌러도 결과가 없는 필터는 잠근다 — 고장으로 보이지 않게. */
  disabled?: boolean;
  title?: string;
  onClick: () => void;
  children: React.ReactNode;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      disabled={disabled}
      title={title}
      aria-pressed={active}
      className={cn(
        "inline-flex h-7 shrink-0 items-center gap-1 whitespace-nowrap rounded-full border px-2.5 text-cell font-medium outline-none transition-colors focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-[var(--brand)]/50",
        active
          ? "border-[var(--brand)] bg-[var(--brand)] text-white"
          : "border-[var(--border)] bg-[var(--surface)] text-[var(--text-secondary)] hover:border-[var(--text-muted)] hover:text-[var(--text-primary)]",
        disabled && "cursor-not-allowed opacity-50 hover:border-[var(--border)]",
      )}
    >
      {active && <Check className="h-3 w-3 shrink-0" aria-hidden />}
      {children}
    </button>
  );
}

function FilterChip({ label, onClear }: { label: string; onClear: () => void }) {
  return (
    <button
      type="button"
      onClick={onClear}
      aria-label={`${label} 필터 해제`}
      className="inline-flex items-center gap-1 rounded-full bg-[var(--brand-muted)] px-2 py-0.5 text-cell font-medium text-[var(--brand)]"
    >
      {label}
      <X className="size-3" />
    </button>
  );
}
