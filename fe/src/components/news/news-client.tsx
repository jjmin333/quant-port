"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useSearchParams } from "next/navigation";
import { RefreshCw, X } from "lucide-react";
import type { NewsArticle, NewsMarket, NewsSort, NewsSourceId } from "@/lib/news-types";
import { getMockNews, sourcesForMarket } from "@/lib/mock-news";
import { fetchDomesticNews, fetchUsNews } from "@/lib/news-api";
import { fetchNewsBriefing, type NewsMarketBriefing } from "@/lib/news-briefing-api";
import {
  filterArticlesByIndex,
  filterLabel,
  findLatestForFilter,
  NEWS_INDEX_FILTER_LABELS,
  NEWS_INDEX_PILLS,
  type NewsIndexFilter,
} from "@/lib/news-index-filter";
import {
  filterArticlesByUSIndex,
  findLatestForUSFilter,
  NEWS_US_INDEX_FILTER_LABELS,
  NEWS_US_INDEX_PILLS,
  usFilterLabel,
  type NewsUSIndexFilter,
} from "@/lib/news-us-index-filter";
import { dedupeNewsArticles } from "@/lib/news-dedupe";
import {
  articleMatchesDashboardIndex,
  dashboardIndexLabel,
  dashboardIndexToPillFilter,
  pillFilterToDashboardIndex,
  type DashboardIndexId,
} from "@/lib/news-dashboard-indices";
import { sectionTitleForBoard } from "@/lib/news-market-briefing";
import {
  allArticleSymbols,
  articlesInHour,
  buildNewsFlow,
  topSymbolMentions,
} from "@/lib/news-derive";
import { useMySymbols, useSymbolQuotes } from "@/hooks/use-news-symbols";
import { useMarketIndices } from "@/hooks/use-market-indices";
import { useHeaderSearch } from "@/components/providers/header-search-provider";
import { PageScrollShell } from "@/components/layout/page-content-shell";
import { PageColumns } from "@/components/layout/page-columns";
import { NewsSideRail } from "@/components/news/news-side-rail";
import { NewsBreakingTicker } from "@/components/news/news-breaking-ticker";
import { NewsFeed } from "@/components/news/news-feed";
import { NewsBriefingCard } from "@/components/news/news-briefing-card";
import { NewsIssueStrip } from "@/components/news/news-issue-strip";
import { NewsFlowCard } from "@/components/news/news-flow-card";
import { useNewsArticleModal } from "@/components/news/news-article-modal-host";
import { NewsPublisherFilterChip } from "@/components/news/news-rail-cards";
import { NewsSampleSymbolsBanner } from "@/components/news/news-login-gate";
import {
  EMPTY_SECONDARY_FILTERS,
  NewsSecondaryFilterRow,
  type NewsSecondaryFilters,
} from "@/components/news/news-secondary-filters";
import { cn } from "@/lib/utils";

type LiveNewsMarket = Extract<NewsMarket, "domestic" | "us">;
type ActiveIndexFilter = NewsIndexFilter | NewsUSIndexFilter;

/** 「내 종목」은 지수·종목 pill과 같은 줄에 있지만 성격이 달라 별도 상태로 둔다. */
const MY_SYMBOLS_FILTER = "__mine__";

const PAGE_COPY: Record<
  LiveNewsMarket,
  { subtitle: string; emptyBreaking: string; emptyFilter: string }
> = {
  domestic: {
    subtitle: "코스피·코스닥과 주요 종목 뉴스를 카테고리별로 모아 보여드립니다.",
    emptyBreaking:
      "최근 24시간 내에 발행된 속보 기사가 없습니다. 「전체」에는 다른 증시 뉴스도 표시됩니다.",
    emptyFilter: "관련 기사가 없습니다. 「전체」를 선택하면 다른 증시 뉴스도 표시됩니다.",
  },
  us: {
    subtitle: "나스닥·S&P500과 해외 주요 종목·매크로 뉴스를 번역해 제공합니다.",
    emptyBreaking:
      "최근 24시간 내 속보성 기사가 없습니다. 「전체」에는 다른 해외 증시 뉴스도 표시됩니다.",
    emptyFilter: "관련 기사가 없습니다. 「전체」를 선택하면 다른 해외 증시 뉴스도 표시됩니다.",
  },
};

function isLiveNewsMarket(market: NewsMarket): market is LiveNewsMarket {
  return market === "domestic" || market === "us";
}

type NewsClientProps = {
  market: NewsMarket;
};

export function NewsClient({ market }: NewsClientProps) {
  const { activeQuery, isSearchActive, mode } = useHeaderSearch();
  const searchParams = useSearchParams();
  const marketSources = useMemo(() => sourcesForMarket(market), [market]);
  const newsSearchActive =
    isSearchActive &&
    ((market === "domestic" && mode === "news-domestic") ||
      (market === "us" && mode === "news-us"));

  const [sourceId, setSourceId] = useState<NewsSourceId | "all">("all");
  const [indexFilter, setIndexFilter] = useState<ActiveIndexFilter | typeof MY_SYMBOLS_FILTER>(
    "all",
  );
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
  const [dashboardIndexFilter, setDashboardIndexFilter] = useState<DashboardIndexId | null>(null);
  const [hourFilter, setHourFilter] = useState<number | null>(null);
  const [publisherFilter, setPublisherFilter] = useState<string | null>(null);
  const [secondary, setSecondary] = useState<NewsSecondaryFilters>(EMPTY_SECONDARY_FILTERS);
  const [sampleBannerHidden, setSampleBannerHidden] = useState(false);
  const [sort, setSort] = useState<NewsSort>("latest");
  const [refreshKey, setRefreshKey] = useState(0);
  const [refreshing, setRefreshing] = useState(false);
  const [articles, setArticles] = useState<NewsArticle[]>([]);
  const [totalCount, setTotalCount] = useState(0);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [loading, setLoading] = useState(isLiveNewsMarket(market));
  const [briefing, setBriefing] = useState<NewsMarketBriefing | null>(null);

  const liveMarket: LiveNewsMarket = market === "us" ? "us" : "domestic";
  const mySymbols = useMySymbols(liveMarket);

  const mockArticles = useMemo(
    () => getMockNews({ market, sourceId, sort }),
    [market, sourceId, sort],
  );

  const indexPills = market === "us" ? NEWS_US_INDEX_PILLS : NEWS_INDEX_PILLS;
  const indexLabels =
    market === "us" ? NEWS_US_INDEX_FILTER_LABELS : NEWS_INDEX_FILTER_LABELS;
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
    setIndexFilter("all");
    setSourceId("all");
    setKeywordFilter(null);
    setSymbolFilter(null);
    setDashboardIndexFilter(null);
    setHourFilter(null);
    setPublisherFilter(null);
    setSecondary(EMPTY_SECONDARY_FILTERS);
  }, [market]);

  useEffect(() => {
    if (!isLiveNewsMarket(market)) {
      return;
    }

    let cancelled = false;
    queueMicrotask(() => {
      if (!cancelled) {
        setLoading(true);
        setLoadError(null);
      }
    });

    const fetchNews = market === "domestic" ? fetchDomesticNews : fetchUsNews;
    fetchNews({
      market,
      sourceId,
      sort,
      query: newsSearchActive ? activeQuery : undefined,
    })
      .then(({ articles: items, totalCount: count }) => {
        if (!cancelled) {
          setArticles(items);
          setTotalCount(count);
        }
      })
      .catch((err: unknown) => {
        if (!cancelled) {
          setLoadError(err instanceof Error ? err.message : "뉴스를 불러오지 못했습니다");
          setArticles([]);
          setTotalCount(0);
        }
      })
      .finally(() => {
        if (!cancelled) {
          setLoading(false);
        }
      });

    return () => {
      cancelled = true;
    };
  }, [market, sourceId, sort, refreshKey, activeQuery, newsSearchActive]);

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
        if (!cancelled) setBriefing(null);
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
   * 화면에 있는 기사의 종목 + 내 종목 = 시세를 받아야 할 목록.
   *
   * 이슈 스트립·행 종목 칩·레일 「내 종목 뉴스」·모달 관련 종목이 **이 한 요청을
   * 공유**한다. 각자 부르면 같은 화면에서 배치 요청이 네 번 나간다.
   */
  const quoteSymbols = useMemo(() => {
    const set = new Set(allArticleSymbols(baseArticles));
    for (const sym of mySymbols.symbols) set.add(sym);
    return [...set];
  }, [baseArticles, mySymbols.symbols]);
  const quotes = useSymbolQuotes(quoteSymbols);
  const { indices } = useMarketIndices();

  const mentions = useMemo(() => topSymbolMentions(baseArticles), [baseArticles]);
  const flowBuckets = useMemo(() => buildNewsFlow(baseArticles), [baseArticles]);

  const usingMySymbols = indexFilter === MY_SYMBOLS_FILTER;

  const activePulseIndexId = useMemo((): DashboardIndexId | null => {
    if (dashboardIndexFilter) {
      return dashboardIndexFilter;
    }
    if (usingMySymbols || newsSearchActive) {
      return null;
    }
    return pillFilterToDashboardIndex(liveMarket, indexFilter as NewsIndexFilter | NewsUSIndexFilter);
  }, [dashboardIndexFilter, usingMySymbols, newsSearchActive, liveMarket, indexFilter]);

  const handleIndexCardSelect = useCallback(
    (indexId: DashboardIndexId) => {
      setSymbolFilter(null);
      const mapped = dashboardIndexToPillFilter(liveMarket, indexId);
      if (mapped) {
        setDashboardIndexFilter(null);
        setIndexFilter((cur) => (cur === mapped ? "all" : mapped));
        return;
      }
      setIndexFilter("all");
      setDashboardIndexFilter((cur) => (cur === indexId ? null : indexId));
    },
    [liveMarket],
  );

  const displayArticles = useMemo(() => {
    let items = baseArticles;

    if (!newsSearchActive && isLiveNewsMarket(market)) {
      if (usingMySymbols) {
        const wanted = new Set(mySymbols.symbols.map((s) => s.toUpperCase()));
        items = items.filter((article) =>
          (article.symbols ?? []).some((s) => wanted.has(s.trim().toUpperCase())),
        );
      } else {
        items =
          market === "us"
            ? filterArticlesByUSIndex(items, indexFilter as NewsUSIndexFilter)
            : filterArticlesByIndex(items, indexFilter as NewsIndexFilter);
      }
    }

    if (dashboardIndexFilter) {
      items = items.filter((article) => articleMatchesDashboardIndex(article, dashboardIndexFilter));
    }

    if (keywordFilter) {
      items = items.filter((article) => (article.keywords ?? []).includes(keywordFilter));
    }
    if (symbolFilter) {
      items = items.filter((article) =>
        (article.symbols ?? []).some((s) => s.trim().toUpperCase() === symbolFilter),
      );
    }
    if (hourFilter !== null) {
      items = articlesInHour(items, new Date(hourFilter));
    }
    if (publisherFilter) {
      items = items.filter((article) => article.publisher.id === publisherFilter);
    }
    if (secondary.withImage) {
      items = items.filter((article) => Boolean(article.imageUrl));
    }
    if (secondary.translatedOnly) {
      items = items.filter((article) => Boolean(article.headlineKo?.trim()));
    }
    return items;
  }, [
    baseArticles,
    indexFilter,
    dashboardIndexFilter,
    usingMySymbols,
    mySymbols.symbols,
    keywordFilter,
    symbolFilter,
    hourFilter,
    publisherFilter,
    secondary,
    market,
    newsSearchActive,
  ]);

  const latestTickerArticle = useMemo(() => {
    if (!isLiveNewsMarket(market) || newsSearchActive || usingMySymbols) {
      return null;
    }
    if (market === "us") {
      return findLatestForUSFilter(baseArticles, indexFilter as NewsUSIndexFilter);
    }
    return findLatestForFilter(baseArticles, indexFilter as NewsIndexFilter);
  }, [baseArticles, indexFilter, usingMySymbols, market, newsSearchActive]);

  const activeFilterLabel = usingMySymbols
    ? "내 종목"
    : market === "us"
      ? usFilterLabel(indexFilter as NewsUSIndexFilter)
      : filterLabel(indexFilter as NewsIndexFilter);

  const handleRefresh = useCallback(() => {
    setRefreshing(true);
    setRefreshKey((k) => k + 1);
    window.setTimeout(() => setRefreshing(false), 400);
  }, []);

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
          <NewsFeed articles={displayArticles} onArticleClick={openArticle} quotes={quotes} indices={indices} />
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
                activePublisherId={publisherFilter}
                onPublisherSelect={setPublisherFilter}
                onArticleClick={openArticle}
                onKeywordSelect={(kw) => setKeywordFilter((cur) => (cur === kw ? null : kw))}
              />
            }
          >
            <div className="@container/news min-w-0">
              <div className="flex items-start justify-between gap-4">
                <div className="min-w-0">
                  <h1 className="text-[26px] font-bold tracking-tight text-[var(--text-primary)]">
                    뉴스
                  </h1>
                  <p className="mt-1.5 text-[13px] leading-relaxed text-[var(--text-muted)]">
                    {pageCopy.subtitle}
                  </p>
                </div>

                <button
                  type="button"
                  aria-label="새로고침"
                  onClick={handleRefresh}
                  className="mt-1 flex h-8 w-8 shrink-0 items-center justify-center rounded-lg border border-[var(--border)] text-[var(--text-secondary)] transition-colors hover:bg-[var(--surface-elevated)] hover:text-[var(--text-primary)]"
                >
                  <RefreshCw className={cn("h-3.5 w-3.5", refreshing && "animate-spin")} />
                </button>
              </div>

              {!loading && !newsSearchActive && (
                <div className="mt-5 space-y-3">
                  <NewsBreakingTicker
                    article={latestTickerArticle}
                    badgeLabel={`${activeFilterLabel} 최신`}
                  />
                  <NewsBriefingCard briefing={briefing} />
                  <NewsIssueStrip
                    market={market}
                    articles={baseArticles}
                    mentions={mentions}
                    quotes={quotes}
                    activeSymbol={symbolFilter}
                    activeIndexId={activePulseIndexId}
                    onSelect={(symbol) => {
                      setDashboardIndexFilter(null);
                      setSymbolFilter((cur) => (cur === symbol ? null : symbol));
                    }}
                    onSelectIndex={handleIndexCardSelect}
                  />
                  <NewsFlowCard
                    buckets={flowBuckets}
                    activeHour={hourFilter}
                    onSelectHour={setHourFilter}
                  />
                </div>
              )}

              {!newsSearchActive && (
                <div className="mt-5 space-y-2">
                  <div className="flex flex-wrap gap-1.5">
                    <Pill
                      active={indexFilter === indexPills[0] && !dashboardIndexFilter}
                      onClick={() => {
                        setDashboardIndexFilter(null);
                        setIndexFilter(indexPills[0]);
                      }}
                    >
                      {indexLabels[indexPills[0] as keyof typeof indexLabels]}
                    </Pill>
                    <Pill
                      active={usingMySymbols}
                      onClick={() => {
                        setDashboardIndexFilter(null);
                        setIndexFilter((cur) => (cur === MY_SYMBOLS_FILTER ? "all" : MY_SYMBOLS_FILTER));
                      }}
                    >
                      ⭐ 내 종목
                      {!mySymbols.personal && !mySymbols.loading && (
                        <span className="ml-1 text-[10px] opacity-80">예시</span>
                      )}
                    </Pill>
                    {indexPills.slice(1).map((key) => (
                      <Pill
                        key={key}
                        active={indexFilter === key}
                        onClick={() => {
                          setDashboardIndexFilter(null);
                          setIndexFilter(key);
                        }}
                      >
                        {indexLabels[key as keyof typeof indexLabels]}
                      </Pill>
                    ))}
                    {/*
                     * 코드에 박아둔 종목 pill 대신 **오늘 실제로 많이 언급된** 종목을
                     * 붙인다. 매일 달라지는 것이 이 자리의 값어치다.
                     */}
                    {mentions.slice(0, 3).map((mention) => (
                      <Pill
                        key={mention.symbol}
                        active={symbolFilter === mention.symbol}
                        onClick={() =>
                          setSymbolFilter((cur) =>
                            cur === mention.symbol ? null : mention.symbol,
                          )
                        }
                      >
                        {quotes.get(mention.symbol)?.name || mention.symbol}
                        <span className="ml-1 font-mono text-[10px] opacity-70">
                          {mention.count}
                        </span>
                      </Pill>
                    ))}
                  </div>

                  <NewsSecondaryFilterRow
                    market={market}
                    sort={sort}
                    onSortChange={setSort}
                    sourceId={sourceId}
                    sources={marketSources}
                    onSourceChange={setSourceId}
                    filters={secondary}
                    onFiltersChange={setSecondary}
                  />

                  {showSampleBanner && (
                    <NewsSampleSymbolsBanner
                      sampleNames={sampleNames}
                      onDismiss={() => setSampleBannerHidden(true)}
                    />
                  )}
                </div>
              )}

              <div className="mt-6 flex items-end justify-between gap-3 border-b border-[var(--border)] pb-3">
                <h2 className="flex min-w-0 flex-wrap items-center gap-2 text-sm font-bold text-[var(--text-primary)]">
                  {newsSearchActive
                    ? `「${activeQuery}」 검색 결과`
                    : usingMySymbols
                      ? "내 종목 뉴스"
                      : dashboardIndexFilter
                        ? `${dashboardIndexLabel(dashboardIndexFilter)} 뉴스`
                        : sectionTitleForBoard(indexFilter as ActiveIndexFilter, market)}
                  {symbolFilter && (
                    <FilterChip
                      label={quotes.get(symbolFilter)?.name || symbolFilter}
                      onClear={() => setSymbolFilter(null)}
                    />
                  )}
                  {dashboardIndexFilter && (
                    <FilterChip
                      label={dashboardIndexLabel(dashboardIndexFilter)}
                      onClear={() => setDashboardIndexFilter(null)}
                    />
                  )}
                  {keywordFilter && (
                    <FilterChip label={`#${keywordFilter}`} onClear={() => setKeywordFilter(null)} />
                  )}
                  {hourFilter !== null && (
                    <FilterChip
                      label={`${new Date(hourFilter).getHours()}시`}
                      onClear={() => setHourFilter(null)}
                    />
                  )}
                  {publisherFilter && (
                    <NewsPublisherFilterChip
                      name={
                        baseArticles.find((a) => a.publisher.id === publisherFilter)?.publisher
                          .name ?? publisherFilter
                      }
                      onClear={() => setPublisherFilter(null)}
                    />
                  )}
                </h2>
                <span className="shrink-0 text-xs text-[var(--text-muted)]">
                  {loading ? "불러오는 중…" : `${displayArticles.length}건`}
                </span>
              </div>

              {!newsSearchActive && !loading && (
                <p className="mt-1 text-[11px] text-[var(--text-muted)]">
                  전체 {totalCount.toLocaleString("ko-KR")}건 · 화면 {displayArticles.length}건
                </p>
              )}

              {loadError && <p className="mt-2 text-xs text-red-400">{loadError}</p>}

              {!loading && displayArticles.length === 0 ? (
                <p className="px-2 py-12 text-center text-sm text-[var(--text-muted)]">
                  {newsSearchActive
                    ? "검색 결과가 없습니다. 다른 키워드를 입력해 보세요."
                    : usingMySymbols
                      ? "내 종목이 언급된 기사가 아직 없습니다."
                      : keywordFilter
                        ? `「${keywordFilter}」 키워드와 현재 필터를 함께 만족하는 기사가 없습니다.`
                        : indexFilter === "breaking"
                          ? pageCopy.emptyBreaking
                          : indexFilter !== "all"
                            ? `${indexLabels[indexFilter as keyof typeof indexLabels]} ${pageCopy.emptyFilter}`
                            : "표시할 뉴스가 없습니다. 잠시 후 새로고침해 주세요."}
                </p>
              ) : (
                <NewsFeed
                  articles={displayArticles}
                  onArticleClick={openArticle}
                  quotes={quotes}
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

function Pill({
  active,
  onClick,
  children,
}: {
  active: boolean;
  onClick: () => void;
  children: React.ReactNode;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      aria-pressed={active}
      className={cn(
        "inline-flex items-center rounded-full border px-3 py-1.5 text-[12px] font-medium transition-colors",
        active
          ? "border-[var(--brand)] bg-[var(--brand)] text-white"
          : "border-[var(--border)] bg-[var(--surface)] text-[var(--text-secondary)] hover:border-[var(--text-muted)] hover:text-[var(--text-primary)]",
      )}
    >
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
      className="inline-flex items-center gap-1 rounded-full bg-[var(--brand-muted)] px-2 py-0.5 text-[11px] font-medium text-[var(--brand)]"
    >
      {label}
      <X className="size-3" />
    </button>
  );
}
