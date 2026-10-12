"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { SearchX } from "lucide-react";
import { AIAnalysisTab } from "@/components/analysis/ai-analysis-tab";
import { EarningsTab } from "@/components/analysis/earnings-tab";
import { InsiderTab } from "@/components/analysis/insider-tab";
import { StockCommunityTab } from "@/components/community/stock-community-tab";
import {
  CompanyInfoTab,
  FinancialsChartsTab,
  ValuationTab,
} from "@/components/analysis/lazy-tabs";
import {
  StockMetricsColumn,
  type MetricsRailBreakpoint,
} from "@/components/stock-workspace/stock-metrics-column";
import { StockOverviewTab } from "@/components/stock-workspace/stock-overview-tab";
import { StockWorkspaceHeader } from "@/components/stock-workspace/stock-workspace-header";
import { useAuth } from "@/components/providers/auth-provider";
import { MyRecordRow } from "@/components/stock-workspace/my-record-row";
import { MiniToggle } from "@/components/ui/mini-toggle";
import { StockRedditPanel } from "@/components/community/stock-reddit-panel";
import { fetchCommunityPosts } from "@/lib/community-api";
import { useCommunityVisible } from "@/hooks/use-community-visible";
import { fetchStockSignals, type StockSignals } from "@/lib/stock-signals-api";
import type { CommunityPost } from "@/lib/community-types";
import { useSymbolContext } from "@/hooks/use-symbol-context";
import { usePriceScenario } from "@/hooks/use-price-scenario";
import { StockWorkspaceTabBar } from "@/components/stock-workspace/stock-workspace-tab-bar";
import { StockNewsPanel } from "@/components/stocks/stock-news-panel";
import { useExchangeRate } from "@/hooks/use-exchange-rate";
import { useMarketDataSettings } from "@/hooks/use-market-data-settings";
import { usePinnedSymbol } from "@/hooks/use-pinned-symbol";
import { useVisibleInterval } from "@/hooks/use-visible-interval";
import { AxisScoreBadge } from "@/components/analysis/stock-score-card";
import { TAB_SCORE_AXIS, type StockScoreData } from "@/lib/stock-score-api";
import { useScoreChanges } from "@/hooks/use-score-changes";
import { useStockScore } from "@/hooks/use-stock-score";
import { useStockSummary } from "@/hooks/use-stock-summary";
import { useQuoteDetail } from "@/hooks/use-quote-detail";
import { fetchFundamentalsClient } from "@/lib/fundamentals-api";
import type { FundamentalsData } from "@/lib/fundamentals-types";
import { fetchStockClient } from "@/lib/market-api";
import type { StockInfo } from "@/lib/quotes-types";
import {
  FUNDAMENTAL_VIEWS,
  parseFundamentalView,
  parseStockWorkspaceTab,
  STOCK_WORKSPACE_TABS,
  type FundamentalView,
  stockWorkspaceHref,
  type StockWorkspaceTabId,
} from "@/lib/stock-workspace-tabs";
import { PortfolioFetchError } from "@/lib/api-client";
import { rememberStock } from "@/lib/recent-stocks";
import { cn } from "@/lib/utils";
import { buttonVariants } from "@/components/ui/button";

function stockNewsMarket(stock: { assetType?: string; currency?: string }): "domestic" | "us" {
  if (stock.assetType === "us" || stock.currency === "USD") return "us";
  return "domestic";
}

/**
 * Each tab shows the one score axis it is actually about — 차트은 추세, 밸류에이션은
 * 가치 — rather than repeating the composite everywhere. Renders nothing when
 * that axis has no data, so a tab never shows an empty score chip.
 */
function TabScore({
  tab,
  score,
  onOpenAnalysis,
}: {
  tab: string;
  score: StockScoreData | null;
  onOpenAnalysis: () => void;
}) {
  const axis = TAB_SCORE_AXIS[tab];
  if (!axis || score?.breakdown?.[axis] == null) return null;
  return (
    <div className="flex flex-wrap items-center gap-2">
      <AxisScoreBadge score={score} axis={axis} />
      <button
        type="button"
        onClick={onOpenAnalysis}
        className="text-cell font-medium text-[var(--brand)] hover:underline"
      >
        점수 상세 →
      </button>
    </div>
  );
}

/**
 * 이 종목의 최근 커뮤니티 글 3건 — **한 번 받아 둘이 나눠 쓴다** (2026-10-07).
 *   · 탭: 글이 하나도 없으면 「커뮤니티」 탭을 감춘다(사용자 결정 — 글만 센다, 리포트는 안 센다).
 *   · 개요의 「이 종목 이야기」 카드: 같은 3건을 그린다(예전엔 카드가 따로 받았다).
 * 묻다 실패하면 탭은 보인다(`has: true`) — 실패로 탭이 사라지면 있는 글을 못 본다.
 * 답이 오기 전에는 `has: null`이고 탭을 감춘다: 대부분의 종목은 글이 없어, 떴다 사라지는 쪽이 더 잦다.
 */
function useStockCommunityPosts(
  symbol: string,
  enabled: boolean,
): { posts: CommunityPost[] | null; has: boolean | null } {
  const [state, setState] = useState<{ symbol: string; posts: CommunityPost[]; failed: boolean } | null>(null);
  useEffect(() => {
    // 커뮤니티를 볼 수 없으면(관리자 전용, lib/hidden-features) 묻지 않는다 — 탭과 카드가 모두 빠진다.
    if (!enabled) return;
    let alive = true;
    fetchCommunityPosts({ symbol, limit: 3 })
      .then((p) => alive && setState({ symbol, posts: p.items, failed: false }))
      .catch(() => alive && setState({ symbol, posts: [], failed: true }));
    return () => {
      alive = false;
    };
  }, [symbol, enabled]);
  if (!enabled || state?.symbol !== symbol) return { posts: null, has: null };
  return { posts: state.posts, has: state.failed || state.posts.length > 0 };
}

/**
 * 해외 종목의 레딧 자료 — 「레딧」 탭을 세울지와 그 탭의 내용을 **한 번 받아** 둘이 나눠 쓴다
 * (2026-10-09 요청 4). 국내 종목은 묻지 않는다(레딧 집계 대상이 아니다). 답이 오기 전과
 * 실패하면 탭을 감춘다 — 커뮤니티 탭과 달리 이쪽은 우리 글이 아니라 외부 집계라 없어도 잃는 게 없다.
 */
/** `undefined`는 받는 중, `null`은 자료 없음. */
function useStockReddit(symbol: string, overseas: boolean | null): StockSignals | null | undefined {
  const [state, setState] = useState<{ symbol: string; signals: StockSignals | null } | null>(null);
  useEffect(() => {
    if (!overseas) return;
    let alive = true;
    fetchStockSignals(symbol)
      .then((s) => alive && setState({ symbol, signals: s }))
      .catch(() => alive && setState({ symbol, signals: null }));
    return () => {
      alive = false;
    };
  }, [symbol, overseas]);
  if (overseas === false) return null;
  if (!overseas || state?.symbol !== symbol) return undefined;
  return state.signals && state.signals.social.length > 0 ? state.signals : null;
}

type StockWorkspaceClientProps = {
  symbol: string;
  /** 서버가 이미 아는 종목명(SEO 요약). 종목 조회가 오기 전 제목에 쓴다. */
  initialName?: string;
  /** Controlled tab (embedded /quotes). Omit to use URL routing on /stocks/[symbol]. */
  activeTab?: StockWorkspaceTabId;
  onTabChange?: (tab: StockWorkspaceTabId) => void;
  embedded?: boolean;
  compact?: boolean;
  onBack?: () => void;
  className?: string;
  /*
    탭 목록·모드를 바꾸는 prop은 없다 (2026-10-09 요청 1). 화면마다 탭을 갈아 끼울 수 있던
    시절에는 같은 종목이 진입 경로에 따라 다른 화면이 됐다. 다른 것은 자리(폭·뒤로 가기)뿐이다.
  */
  /**
   * Width at which the 240px metrics rail appears. Pass "xl" when the page
   * already spends a column on something else (마켓 레이더's ranking list),
   * otherwise the rail starves the tab content at laptop widths.
   */
  railFrom?: MetricsRailBreakpoint;
};

export function StockWorkspaceClient({
  symbol,
  initialName,
  activeTab: controlledTab,
  onTabChange,
  embedded = false,
  compact = false,
  onBack,
  className,
  railFrom = "lg",
}: StockWorkspaceClientProps) {
  const communityVisible = useCommunityVisible();
  const community = useStockCommunityPosts(symbol, communityVisible);
  const router = useRouter();
  const searchParams = useSearchParams();
  const exchangeRate = useExchangeRate(true);
  const { fundamentalsPollMs } = useMarketDataSettings();
  // Fetched here, not inside the header line: the header renders that line
  // twice (wide vs narrow layout) and each copy would otherwise fetch.
  const { data: aiSummary, loading: aiSummaryLoading } = useStockSummary(symbol);
  // Fetched once here: 개요 탭이 전체 분해를 그리고, 나머지 탭은 자기 축 하나만
  // 배지로 보여 준다.
  const { data: stockScore, loading: stockScoreLoading } = useStockScore(symbol);
  /*
    점수 변화 한 줄 — 개요 카드가 쓴다.

    설명 전체(`/score/explain`, 15KB)가 아니라 요약 입구(`/scores/changes`, 종목당
    약 260B)를 부른다. 개요는 「얼마나, 왜」 두 값만 쓰고, 나머지를 쓰는 AI 분석 탭은
    그 탭이 열릴 때 자기 것을 받는다.
  */
  const scoreChanges = useScoreChanges(useMemo(() => [symbol], [symbol]));
  const scoreChange = scoreChanges.get(symbol.trim().toUpperCase()) ?? null;

  const urlTab = parseStockWorkspaceTab(searchParams.get("tab"));
  /* 「기업·재무」 탭 안의 보기. 옛 주소(?tab=valuation)가 고른 보기로 시작하고, 종목을 옮겨도 유지한다. */
  const [fundView, setFundView] = useState<FundamentalView>(() => parseFundamentalView(searchParams.get("tab")));
  const [embeddedTab, setEmbeddedTab] = useState<StockWorkspaceTabId>("overview");
  const [stock, setStock] = useState<StockInfo | null>(null);
  const [fundamentals, setFundamentals] = useState<FundamentalsData | null>(null);
  const [fundamentalsLoading, setFundamentalsLoading] = useState(false);
  /** 지금 화면이 보고 있는 심볼. 늦게 온 재무 응답을 버릴 기준이다. */
  const fundamentalsSymbolRef = useRef<string | null>(null);
  const [stockLoading, setStockLoading] = useState(false);
  /** 카탈로그에 없는 심볼(404). 로딩과 구분해야 화면이 멈추지 않는다. */
  const [stockMissing, setStockMissing] = useState(false);
  /*
    관심 등록·해제 버튼은 상세 헤더에서 뺐다(2026-09-03 사용자 결정) — 하트는
    랭킹 목록의 행마다 이미 있다. 그 상태는 이제 공유 저장소(`lib/watchlist-store`)가
    한 벌로 들고 있어, 어느 화면에서 누르든 나머지 화면이 같은 프레임에 따라온다.
  */

  const requestedTab = controlledTab ?? (embedded ? embeddedTab : urlTab);
  // ?tab=community는 커뮤니티를 볼 수 없는 사람에게 개요로 연다.
  const effectiveTab: StockWorkspaceTabId =
    !communityVisible && requestedTab === "community" ? "overview" : requestedTab;
  const reddit = useStockReddit(symbol, stock ? stockNewsMarket(stock) === "us" : null);
  // 보고 있는 중이면 감추지 않는다 — 링크로 들어온 커뮤니티·레딧 탭이 발밑에서 사라지면 안 된다.
  const tabs = STOCK_WORKSPACE_TABS.filter((t) =>
    t.id === "community"
      ? communityVisible && (community.has === true || effectiveTab === "community")
      : t.id === "reddit"
        ? reddit != null || effectiveTab === "reddit"
        : true,
  );

  /*
    시나리오는 **개요가 열렸을 때만** 받는다 — 2026-10-07부터 종합 점수 카드 안에 선다
    (요청 9 — 그전엔 밸류에이션 탭). 다른 탭에서는 그리지 않으니 받지 않는다.
  */
  const { scenario, loading: scenarioLoading } = usePriceScenario(symbol, effectiveTab === "overview");

  /*
    「이 종목에 대한 내 기록」 — 발견→근거→판단→반영→추적을 잇는 줄 (요구 9).

    비로그인이면 **부르지 않는다.** 응답이 전부 「내」 기록이라 401이 정상이고,
    화면도 줄을 안 그린다.
  */
  const { session } = useAuth();
  const myRecord = useSymbolContext(symbol, Boolean(session));
  /*
    호가는 받지 않는다(`wantsOrderBook` 기본 false) — 호가창을 그리던 차트 탭이 없어졌다
    (2026-10-09 요청 1). 예전에도 그 탭이 열렸을 때만 구독했다(실측: 호가 프레임이 체결의 4.3배).
  */
  const { detail: quote, loading: quoteLoading } = useQuoteDetail(symbol);

  /*
    이 화면이 떠 있는 동안은 **이 종목을 보고 있다** (2026-09-15 사용자 요청 4).

    탭이 숨어 유예가 지나도 이 종목의 가격은 계속 흐른다(`lib/pinned-symbols`).
    시장은 종목을 받아 오기 전에는 모르므로 `stock`이 채워진 뒤에 붙는다 — 그전에
    KRW로 넘겨 두면 해외 종목이 국내 연결에 핀을 건다.
  */
  usePinnedSymbol(symbol, stock ? stockNewsMarket(stock) : null);

  useEffect(() => {
    if (!symbol) return;
    let cancelled = false;
    setStockLoading(true);
    setStockMissing(false);
    fetchStockClient(symbol)
      .then((stockRow) => {
        if (cancelled) return;
        setStock(stockRow);
        // 검색이 아니라 링크·랭킹으로 들어온 것도 「최근 본 종목」이다.
        if (stockRow?.symbol) rememberStock(stockRow);
      })
      .catch((err) => {
        if (cancelled) return;
        /*
          **없는 종목과 일시적 실패를 가른다.**

          예전에는 어떤 실패든 `{symbol, name: symbol, market: "—"}` 라는 가짜 종목을
          만들어 워크스페이스를 그대로 그렸다. 그러면 시세·요약·차트가 전부 오지
          않으므로 화면은 「시세 로딩 중…」에서 멈춘 채 끝나지 않았다(실측:
          `/stocks/ZZZZZZ` 를 22초 넘게 관찰해도 그대로).

          404는 「이 심볼은 존재하지 않는다」는 확정된 답이므로 로딩이 아니라
          「찾을 수 없음」으로 말해야 한다. 그 밖의 실패(500·네트워크)는 종목이
          있을 수도 있으니 기존 폴백을 유지한다.
        */
        if (err instanceof PortfolioFetchError && err.status === 404) {
          setStock(null);
          setStockMissing(true);
          return;
        }
        setStock({
          symbol,
          name: symbol,
          market: "—",
          currency: "KRW",
        });
      })
      .finally(() => {
        if (!cancelled) setStockLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [symbol]);

  /*
    종목이 바뀌면 이 함수도 새로 만들어진다 — 그 자체가 「새 종목으로 다시 받아라」다.
    아래 첫 적재 이펙트와 되풀이가 **같은 함수**를 쓰게 하려고 `useCallback`으로 뺐다.
  */
  const loadFundamentals = useCallback(() => {
    if (!symbol) return;
    setFundamentalsLoading(true);
    /*
      **늦게 온 응답이 새 종목의 값을 덮지 않게 한다.** 종목을 빠르게 옮기면 앞
      종목의 응답이 뒤에 도착할 수 있다. 예전에는 이펙트의 `cancelled` 플래그가
      그 일을 했는데, 되풀이가 이펙트 밖으로 나오면서 기준을 **심볼 자체**로
      바꿨다 — 지금 화면이 보고 있는 심볼의 응답만 받는다.
    */
    fetchFundamentalsClient(symbol)
      .then((data) => {
        if (fundamentalsSymbolRef.current === symbol) setFundamentals(data);
      })
      .catch(() => {
        if (fundamentalsSymbolRef.current === symbol) setFundamentals(null);
      })
      .finally(() => {
        if (fundamentalsSymbolRef.current === symbol) setFundamentalsLoading(false);
      });
  }, [symbol]);

  useEffect(() => {
    fundamentalsSymbolRef.current = symbol;
    if (!symbol) {
      setFundamentals(null);
      return;
    }
    loadFundamentals();
    // 화면이 사라진 뒤 도착한 응답은 버린다 — 위 `then`들이 이 값을 견준다.
    return () => {
      fundamentalsSymbolRef.current = null;
    };
  }, [symbol, loadFundamentals]);

  /*
    되풀이는 **보이는 동안만** (2026-09-15 사용자 요청 3). 상세 화면을 탭 여러 개에
    띄워 두면 숨은 것까지 같은 주기로 서버를 두드리고 있었다.
  */
  useVisibleInterval(loadFundamentals, fundamentalsPollMs, Boolean(symbol));


  useEffect(() => {
    if (embedded && controlledTab == null) {
      setEmbeddedTab("overview");
    }
  }, [symbol, embedded, controlledTab]);

  const stockRow = useMemo(
    () =>
      stock ?? {
        symbol,
        name: initialName || symbol,
        market: "—",
        currency: "KRW" as const,
      },
    [stock, symbol, initialName],
  );

  function setTab(tab: StockWorkspaceTabId) {
    if (onTabChange) {
      onTabChange(tab);
      return;
    }
    if (embedded) {
      setEmbeddedTab(tab);
      return;
    }
    router.replace(stockWorkspaceHref(symbol, tab), { scroll: false });
  }

  const analysisLoading = stockLoading || fundamentalsLoading;

  /*
    존재하지 않는 심볼. 탭·차트·시세를 그려 봐야 전부 빈 값이므로 워크스페이스 자체를
    그리지 않고 「찾을 수 없음」과 출구만 준다.
  */
  if (stockMissing) {
    return (
      <div
        className={cn(
          "flex min-h-0 min-w-0 flex-1 flex-col items-center justify-center gap-4 bg-[var(--background)] px-6 py-16 text-center",
          className,
        )}
      >
        <div className="grid size-11 place-items-center rounded-full bg-[var(--surface-elevated)] text-[var(--text-secondary)]">
          <SearchX className="h-5 w-5" />
        </div>
        <div className="space-y-1.5">
          <h2 className="text-base font-semibold text-[var(--text-primary)]">
            종목을 찾을 수 없습니다
          </h2>
          <p className="max-w-[420px] text-cell leading-relaxed text-[var(--text-muted)]">
            <span className="font-num font-semibold text-[var(--text-secondary)]">{symbol}</span>{" "}
            는 등록된 종목이 아닙니다. 종목코드나 티커를 다시 확인해 주세요.
          </p>
        </div>
        <div className="flex flex-wrap items-center justify-center gap-2">
          <Link
            href="/analysis"
            className={cn(buttonVariants({ variant: "default", size: "sm" }), "bg-[var(--brand)] text-white hover:opacity-90")}
          >
            종목 찾아보기
          </Link>
          {onBack && (
            <button
              type="button"
              onClick={onBack}
              className={cn(buttonVariants({ variant: "outline", size: "sm" }), "border-[var(--border)] text-[var(--text-secondary)] hover:bg-[var(--surface-elevated)]")}
            >
              뒤로
            </button>
          )}
        </div>
      </div>
    );
  }

  return (
    <div
      className={cn(
        "flex min-h-0 min-w-0 flex-col overflow-hidden bg-[var(--background)]",
        className,
      )}
    >
      <StockWorkspaceHeader
        stock={stockRow}
        quote={quote}
        exchangeRate={exchangeRate}
        loading={stockLoading && !stock}
        onBack={onBack}
        compact={compact}
        headingLevel={embedded ? 2 : 1}
      />

      {/*
        헤더 **바로 아래**다. 「이 종목이 무엇인가」(헤더) 다음에 「나는 이 종목에
        무엇을 해뒀나」가 오고, 그다음이 탭이다. 탭 아래로 내리면 탭을 옮길 때마다
        줄이 사라졌다 나타난다.
      */}
      <MyRecordRow context={myRecord} />

      <StockWorkspaceTabBar active={effectiveTab} onChange={setTab} tabs={tabs} />

      <div className="flex min-h-0 flex-1 overflow-hidden">
        {effectiveTab === "overview" && (
          <StockMetricsColumn quote={quote} railFrom={railFrom} />
        )}

        {/* Every tab scrolls here. Restricting this to 개요 once clipped the taller
            tabs on desktop — 내부자 거래 lost ~910px of 1521px with no scrollbar. */}
        <div className="flex min-h-0 min-w-0 flex-1 flex-col overflow-y-auto overflow-x-hidden">
          {effectiveTab === "overview" && (
            <StockOverviewTab
              stock={stockRow}
              quote={quote}
              fundamentals={fundamentals}
              compact={compact}
              railFrom={railFrom}
              aiSummary={aiSummary}
              aiSummaryLoading={aiSummaryLoading}
              score={stockScore}
              scoreLoading={stockScoreLoading}
              scoreChange={scoreChange}
              onOpenExplain={() => setTab("ai")}
              scenario={scenario}
              scenarioLoading={scenarioLoading}
              communityPosts={community.posts}
            />
          )}

          {effectiveTab === "ai" && (
            <div className="p-3">
              <AIAnalysisTab
                symbol={symbol}
                currency={stockRow.currency}
                data={aiSummary}
                loading={aiSummaryLoading}
                compact={compact}
              />
            </div>
          )}

          {/* The per-tab AI 요약 banners are gone: the headline now sits atop
              the 개요 tab and the full text in the AI 분석 tab, so repeating a
              section of it above every tab only ate vertical space. */}
          {/*
            「기업·재무」 — 기업 정보 · 밸류에이션 · 재무제표 한 탭 (2026-10-07 요청 10).
            보기 토글은 **왼쪽 위에 크게**(2026-10-09 요청 2 — 오른쪽 위 작은 토글은 처음 온 사람이
            못 찾았다), 점수 배지는 그 오른쪽(보기마다 그 축: 가치 · 수익성).
            90일 시나리오·애널리스트 의견은 개요로 옮겼다(요청 9).
          */}
          {effectiveTab === "company" && (
            <div className="space-y-2 p-3">
              <div className="flex flex-wrap items-center gap-2">
                <MiniToggle
                  size="lg"
                  className="w-full sm:w-auto"
                  options={FUNDAMENTAL_VIEWS.map((v) => ({ id: v.id, label: v.label }))}
                  value={fundView}
                  onChange={setFundView}
                />
                <TabScore tab={fundView} score={stockScore} onOpenAnalysis={() => setTab("overview")} />
              </div>
              {fundView === "company" && (
                <CompanyInfoTab stock={stockRow} quote={quote} fundamentals={fundamentals} />
              )}
              {fundView === "valuation" && (
                <ValuationTab
                  stock={stockRow}
                  fundamentals={fundamentals}
                  quote={quote}
                  loading={analysisLoading}
                />
              )}
              {fundView === "financials" && (
                <FinancialsChartsTab
                  stock={stockRow}
                  fundamentals={fundamentals}
                  loading={analysisLoading}
                />
              )}
            </div>
          )}

          {effectiveTab === "earnings" && (
            <div className="space-y-2 p-3">
              <TabScore tab="earnings" score={stockScore} onOpenAnalysis={() => setTab("overview")} />
              <EarningsTab symbol={symbol} currency={stockRow.currency} fundamentals={fundamentals} />
            </div>
          )}

          {effectiveTab === "insider" && (
            <div className="space-y-2 p-3">
              <TabScore tab="insider" score={stockScore} onOpenAnalysis={() => setTab("overview")} />
              <InsiderTab symbol={symbol} currency={stockRow.currency} />
            </div>
          )}

          {effectiveTab === "news" && (
            <div className="space-y-2 p-3">
              <TabScore tab="news" score={stockScore} onOpenAnalysis={() => setTab("overview")} />
              <StockNewsPanel
                symbol={symbol}
                name={stockRow.name}
                market={stockNewsMarket(stockRow)}
              />
            </div>
          )}

          {effectiveTab === "community" && (
            <div className="p-3">
              <StockCommunityTab symbol={symbol} name={stockRow.name} />
            </div>
          )}

          {effectiveTab === "reddit" && (
            <div className="p-3">
              {reddit ? (
                <StockRedditPanel symbol={symbol} signals={reddit} />
              ) : (
                <p className="p-4 text-center text-cell text-[var(--text-muted)]">
                  {reddit === undefined ? "레딧 자료를 불러오는 중…" : "이 종목은 레딧 자료가 없습니다."}
                </p>
              )}
            </div>
          )}

          {quoteLoading && !quote && (
            <p className="p-4 text-center text-cell text-[var(--text-muted)]">시세 로딩 중…</p>
          )}
        </div>
      </div>
    </div>
  );
}
