"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Loader2 } from "lucide-react";
import { MenuEmptyState } from "@/components/common/menu-empty-state";
import { LoadErrorState } from "@/components/common/load-error-state";
import { menuIcon } from "@/components/common/menu-icon";
import {
  SampleModeButton,
  SampleRibbon,
  SampleWrap,
  useAutoSample,
  useSampleMode,
} from "@/components/common/sample-mode";
import { useAuth } from "@/components/providers/auth-provider";
import { PageColumns } from "@/components/layout/page-columns";
import { PageActions, PageScrollShell } from "@/components/layout/page-content-shell";
import { WatchlistListHeader } from "@/components/watchlist/watchlist-columns";
import { WatchlistQuickLookModal } from "@/components/watchlist/watchlist-quick-look-modal";
import { WatchlistRow } from "@/components/watchlist/watchlist-row";
import { WatchlistSideRail } from "@/components/watchlist/watchlist-side-rail";
import { WatchlistToolbar } from "@/components/watchlist/watchlist-toolbar";
import { WatchlistTodayStrip } from "@/components/watchlist/watchlist-today-strip";
import { fetchMyToday, type MyTodayItem } from "@/lib/my-today-api";
import { summarizeWatchlistToday } from "@/lib/watchlist-today";
import { useHeaderSearch } from "@/components/providers/header-search-provider";
import { daysUntil, useUpcomingEvents } from "@/hooks/use-upcoming-events";
import { Card } from "@/components/ui/card";
import {
  addWatchlistClient,
  fetchQuotesClient,
  fetchWatchlistClient,
  removeWatchlistClient,
  reorderWatchlistClient,
} from "@/lib/market-api";
import { MENU_COPY } from "@/lib/menu-copy";
import { fetchSampleWatchlist, sampleWatchlistParts } from "@/lib/sample-api";
import { fetchScoresBatchClient, type StockScoreData } from "@/lib/stock-score-api";
import type { QuoteSummary, StockInfo, WatchlistItem } from "@/lib/quotes-types";
import {
  applyStreamPricesToQuotes,
  mergeQuoteSummariesForSymbols,
} from "@/lib/merge-quotes";
import { useQuoteStreamByMarket } from "@/hooks/use-quote-stream";
import { useVisibleInterval } from "@/hooks/use-visible-interval";
import { PRICE_POLL_MS, PRICE_POLL_MS_WHILE_STREAMING } from "@/lib/price-cadence";
import {
  buildWatchlistRows,
  filterWatchlistRows,
  isUsRow,
  sortWatchlistRows,
  type WatchlistMarketFilter,
  type WatchlistSort,
  type WatchlistTrendFilter,
} from "@/lib/watchlist-calc";
import { toast } from "sonner";
import { toUserMessage } from "@/lib/error-message";

const COPY = MENU_COPY.watchlist;
const EMPTY_TODAY: Map<string, MyTodayItem> = new Map();

/*
시세 폴링 주기. **5초에서 2초로 맞췄다 (2026-09-04 사용자 요청 1).**

같은 가격이 화면마다 다른 박자로 늙는 것을 없애는 것이 목적이라, 숫자는 이제
`lib/price-cadence` 한 곳에만 있다. 요청 수는 오히려 줄었다 — SSE가 붙어 있는 동안
이 폴링은 30초로 물러나고, 끊겼을 때만 2초로 돌아온다.
*/
const POLL_MS = PRICE_POLL_MS;
/** 실적·공시는 종목당 1요청이라 상한을 둔다 — 목록 위쪽부터. */
const EVENT_SYMBOL_LIMIT = 12;

function isQuotePending(
  symbol: string,
  quote: QuoteSummary | undefined,
  pendingSymbols: Set<string>,
): boolean {
  if (pendingSymbols.has(symbol)) return true;
  if (!quote) return true;
  return quote.currentPrice <= 0;
}

export function WatchlistClient() {
  const { activeQuery, isSearchActive, mode } = useHeaderSearch();
  const stockSearchActive = isSearchActive && mode === "stock";
  const { session } = useAuth();
  const { active: sample, setActive: setSample } = useSampleMode();

  const [items, setItems] = useState<WatchlistItem[]>([]);
  const [quotes, setQuotes] = useState<QuoteSummary[]>([]);
  const [scores, setScores] = useState<Map<string, StockScoreData>>(() => new Map());
  const [listLoading, setListLoading] = useState(true);
  const [quotesLoading, setQuotesLoading] = useState(false);
  const [adding, setAdding] = useState(false);
  const [pendingSymbols, setPendingSymbols] = useState<Set<string>>(() => new Set());
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [dragSymbol, setDragSymbol] = useState<string | null>(null);
  const [reorderSaving, setReorderSaving] = useState(false);
  const [openSymbol, setOpenSymbol] = useState<string | null>(null);

  const [sort, setSort] = useState<WatchlistSort>("manual");
  const [market, setMarket] = useState<WatchlistMarketFilter>("all");
  const [trend, setTrend] = useState<WatchlistTrendFilter>("all");

  const listGenRef = useRef(0);
  const quotesGenRef = useRef(0);
  const scoresGenRef = useRef(0);

  const symbols = useMemo(() => items.map((i) => i.symbol), [items]);

  /*
    오늘의 신호 — 홈 「내 종목」 카드와 같은 `/user/today` (2026-09-27 사용자 결정 A).
    뉴스 수 · 변화 한 줄 · 점수 변화 · AI 한 줄. 예시 모드와 비로그인은 부르지 않는다
    (남의 예시 종목에 내 신호를 얹을 이유가 없다). 키는 **종목 목록**이다 — 시세처럼
    틱마다 바뀌는 값을 키에 넣으면 요청이 틱마다 다시 나간다.
  */
  const todayKey = !sample && session ? symbols.join(",") : "";
  const [todayItems, setTodayItems] = useState<Map<string, MyTodayItem>>(() => new Map());
  useEffect(() => {
    if (!todayKey) return;
    let alive = true;
    fetchMyToday()
      .then((res) => {
        if (!alive) return;
        setTodayItems(new Map(res.items.map((i) => [i.symbol.toUpperCase(), i])));
      })
      .catch(() => {
        // 신호가 없어도 목록은 온전하다 — 줄만 생략된다.
      });
    return () => {
      alive = false;
    };
  }, [todayKey]);
  const today = todayKey ? todayItems : EMPTY_TODAY;

  const loadWatchlist = useCallback(async (): Promise<WatchlistItem[]> => {
    const gen = ++listGenRef.current;
    setListLoading(true);
    try {
      const list = await fetchWatchlistClient();
      if (gen !== listGenRef.current) return [];
      const next = list ?? [];
      setItems(next);
      setError(null);
      return next;
    } catch (err) {
      if (gen !== listGenRef.current) return [];
      setError(toUserMessage(err, "목록 로드 실패"));
      return [];
    } finally {
      if (gen === listGenRef.current) {
        setListLoading(false);
      }
    }
  }, []);

  const refreshQuotes = useCallback(async (symbolList: string[]) => {
    if (symbolList.length === 0) {
      setQuotes([]);
      setQuotesLoading(false);
      return;
    }
    const gen = ++quotesGenRef.current;
    setQuotesLoading(true);
    try {
      const nextQuotes = await fetchQuotesClient("watchlist");
      if (gen !== quotesGenRef.current) return;

      setQuotes((prev) => {
        const merged = mergeQuoteSummariesForSymbols(symbolList, prev, nextQuotes ?? []);
        setPendingSymbols((pending) => {
          if (pending.size === 0) return pending;
          const next = new Set(pending);
          for (const sym of pending) {
            const quote = merged.find((q) => q.symbol === sym);
            if (quote && quote.currentPrice > 0) {
              next.delete(sym);
            }
          }
          return next.size === pending.size ? pending : next;
        });
        return merged;
      });
    } catch {
      // keep last merged quotes
    } finally {
      if (gen === quotesGenRef.current) {
        setQuotesLoading(false);
      }
    }
  }, []);

  /**
   * 점수는 목록 전체를 **한 번에** 부른다(배치). 시세와 달리 하루 한 번 산출되는
   * 값이라 5초 폴링에 태우지 않고 목록이 바뀔 때만 다시 읽는다.
   */
  const refreshScores = useCallback(async (symbolList: string[]) => {
    if (symbolList.length === 0) {
      setScores(new Map());
      return;
    }
    const gen = ++scoresGenRef.current;
    try {
      const rows = await fetchScoresBatchClient(symbolList);
      if (gen !== scoresGenRef.current) return;
      setScores(new Map(rows.map((r) => [r.symbol.toUpperCase(), r])));
    } catch {
      // 점수는 보조 정보다 — 실패해도 목록은 그대로 쓸 수 있다.
    }
  }, []);

  const load = useCallback(async () => {
    // 예시 모드는 목록·시세·점수 세 요청(전부 인증 뒤에 있다)을 한 픽스처로 대신한다.
    // 조립은 아래 `buildWatchlistRows`가 그대로 하므로 행이 다르게 그려지지 않는다.
    if (sample) {
      const rows = await fetchSampleWatchlist().catch(() => []);
      const parts = sampleWatchlistParts(rows);
      setItems(parts.items as WatchlistItem[]);
      setQuotes(parts.quotes as unknown as QuoteSummary[]);
      setScores(parts.scores as Map<string, StockScoreData>);
      setListLoading(false);
      setQuotesLoading(false);
      setError(null);
      return;
    }
    const nextItems = await loadWatchlist();
    const nextSymbols = nextItems.map((i) => i.symbol);
    await Promise.all([refreshQuotes(nextSymbols), refreshScores(nextSymbols)]);
  }, [sample, loadWatchlist, refreshQuotes, refreshScores]);

  useEffect(() => {
    // 비동기 함수로 감싸 이펙트 본문에서 동기 setState가 일어나지 않게 한다
    // (캐스케이드 렌더 — react-hooks/set-state-in-effect).
    void (async () => {
      await load();
    })();
  }, [load]);

  /*
    가격을 푸시로도 받는다 (2026-09-04 사용자 요청 1).

    구독 대상을 시세 목록에서 만드는 이유는 자산 구분이 거기 있기 때문이다 —
    관심 목록에는 국내·해외가 섞이고, 서버는 그 값으로 어느 거래소 달력을 볼지
    정한다. 예시 모드는 고정 픽스처라 붙지 않는다.
  */
  const streamRows = useMemo(
    () =>
      sample
        ? []
        : quotes.map((q) => ({ symbol: q.symbol, assetType: q.assetType as string })),
    [quotes, sample],
  );
  const { prices: streamPrices, streaming } = useQuoteStreamByMarket(streamRows);

  /*
    스트림 가격은 **상태에 저장하지 않고 렌더 중에 얹는다.** 이펙트에서 `setQuotes`를
    하면 틱마다 렌더가 두 번 돈다 — 행 조립·정렬이 그 목록에 걸려 있어 두 배가
    그대로 비용이다.
  */
  const liveQuotes = useMemo(
    () => applyStreamPricesToQuotes(quotes, streamPrices),
    [quotes, streamPrices],
  );

  /*
    예시는 고정 값이다 — 폴링하면 인증 뒤의 시세 API를 헛되이 두드린다.

    그리고 **안 보이는 탭에서는 아예 돌지 않는다** (2026-09-15 사용자 요청 3).
    돌아오면 건너뛴 회차를 즉시 한 번 메운다 — 규칙은 `useVisibleInterval`에.
  */
  useVisibleInterval(
    () => {
      void refreshQuotes(symbols);
    },
    streaming ? PRICE_POLL_MS_WHILE_STREAMING : POLL_MS,
    !sample && symbols.length > 0,
  );

  async function handleAddStock(stock: StockInfo) {
    if (symbols.includes(stock.symbol)) {
      setNotice(`${stock.name}(${stock.symbol})은(는) 이미 관심종목에 있습니다.`);
      return;
    }
    setAdding(true);
    setError(null);
    setNotice(null);
    setPendingSymbols((prev) => new Set(prev).add(stock.symbol));
    // 낙관적 행 — 서버 응답을 기다리는 동안에도 목록 끝에 자리가 생겨야 "담겼다"는 게
    // 보인다. `id: -1`은 곧 `load()`가 진짜 항목으로 갈아끼운다.
    setItems((prev) =>
      prev.some((i) => i.symbol === stock.symbol)
        ? prev
        : [...prev, { id: -1, symbol: stock.symbol, createdAt: new Date().toISOString() }],
    );
    try {
      await addWatchlistClient(stock.symbol);
      setNotice(`${stock.name}(${stock.symbol})을(를) 관심종목에 추가했습니다.`);
      await load();
    } catch (err) {
      setItems((prev) => prev.filter((i) => i.symbol !== stock.symbol));
      setPendingSymbols((prev) => {
        const next = new Set(prev);
        next.delete(stock.symbol);
        return next;
      });
      setError(toUserMessage(err, "추가 실패"));
    } finally {
      setAdding(false);
    }
  }

  /*
    삭제에 **실행 취소**를 붙인다 (§A-2).

    확인 창을 세우지 않은 이유: 관심종목 해제는 하루에도 여러 번 하는 가벼운 행동이라
    매번 물으면 흐름을 끊는다. 대신 되돌릴 길을 준다 — 정상 사용자는 확인 창을 한 번도
    보지 않고, 잘못 누른 사람은 5초 안에 되돌린다.

    되돌릴 때 **담은 가격을 함께 복원한다**. 이 값이 「담은 뒤로 얼마나 움직였나」의
    기준선이고 그게 관심종목의 존재 이유인데, 예전에는 해제 후 다시 담으면 오늘 시세로
    조용히 덮여 기준선이 리셋됐다(실측: 257,500 → 258,000).
  */
  async function handleRemove(symbol: string) {
    const removed = items.find((i) => i.symbol === symbol);
    try {
      await removeWatchlistClient(symbol);
      setPendingSymbols((prev) => {
        if (!prev.has(symbol)) return prev;
        const next = new Set(prev);
        next.delete(symbol);
        return next;
      });
      await load();
      const label = quotes.find((q) => q.symbol === symbol)?.name || symbol;
      toast(`${label}을(를) 관심종목에서 뺐습니다.`, {
        action: {
          label: "실행 취소",
          onClick: () => {
            void (async () => {
              try {
                await addWatchlistClient(symbol, removed?.addedPrice);
                await load();
              } catch (err) {
                setError(toUserMessage(err, "되돌리지 못했습니다."));
              }
            })();
          },
        },
      });
    } catch (err) {
      setError(toUserMessage(err, "삭제 실패"));
    }
  }

  function moveSymbol(list: string[], from: string, to: string): string[] {
    const next = [...list];
    const fromIdx = next.indexOf(from);
    const toIdx = next.indexOf(to);
    if (fromIdx < 0 || toIdx < 0 || fromIdx === toIdx) return list;
    next.splice(fromIdx, 1);
    next.splice(toIdx, 0, from);
    return next;
  }

  async function persistWatchlistOrder(nextSymbols: string[]) {
    setReorderSaving(true);
    try {
      await reorderWatchlistClient(nextSymbols);
      setError(null);
    } catch (err) {
      setError(toUserMessage(err, "순서 저장 실패"));
      await loadWatchlist();
    } finally {
      setReorderSaving(false);
    }
  }

  const dragEnabled = sort === "manual" && !stockSearchActive && !reorderSaving;

  function handleWatchlistDrop(targetSymbol: string) {
    if (!dragSymbol || dragSymbol === targetSymbol || !dragEnabled) return;
    const nextSymbols = moveSymbol(symbols, dragSymbol, targetSymbol);
    // 순서만 바꾼다 — 담은 가격·담은 날짜는 항목에 붙어 따라간다.
    const bySymbol = new Map(items.map((i) => [i.symbol, i]));
    setItems(nextSymbols.map((s) => bySymbol.get(s)!).filter(Boolean));
    setDragSymbol(null);
    void persistWatchlistOrder(nextSymbols);
  }

  const rows = useMemo(
    // 스트림 가격이 얹힌 목록을 쓴다 — 행의 현재가·등락률이 지금 값을 따라간다.
    () => buildWatchlistRows(items, liveQuotes, scores),
    [items, liveQuotes, scores],
  );

  /*
    실패와 「0건」을 가른다. 예전에는 실패해도 목록 자리에 `internal` 한 줄만 뜨고
    「관심종목이 없습니다」가 함께 나와, 담아 둔 종목이 사라진 것처럼 보였다.
  */
  const loadFailed = !sample && !listLoading && Boolean(error) && items.length === 0;
  /*
    **`sample`이 섞이지 않은** 「내 것이 아직 없다」 (2026-09-14 사용자 요청 4).
    `useAutoSample`이 이 값을 받는데, 여기에 `!sample`을 끼우면 켜짐 → 0건 아님 →
    꺼짐 → 0건 → 켜짐으로 무한히 돈다. 아래 `isEmpty`가 그 위에 `!sample`을 얹는다.
  */
  const hasNothingYet = !listLoading && !error && items.length === 0;
  useAutoSample(listLoading ? null : hasNothingYet);
  const isEmpty = !sample && hasNothingYet;

  const counts = useMemo(
    () => ({
      all: rows.length,
      domestic: rows.filter((r) => r.quote && !isUsRow(r)).length,
      us: rows.filter((r) => isUsRow(r)).length,
      up: rows.filter((r) => r.quote && r.quote.currentPrice > 0 && r.quote.changeRate > 0).length,
      down: rows.filter((r) => r.quote && r.quote.currentPrice > 0 && r.quote.changeRate < 0).length,
    }),
    [rows],
  );

  const visibleRows = useMemo(() => {
    let next = filterWatchlistRows(rows, market, trend);
    if (stockSearchActive) {
      const q = activeQuery.toLowerCase();
      next = next.filter(
        (r) =>
          r.symbol.toLowerCase().includes(q) ||
          (r.quote?.name?.toLowerCase() ?? "").includes(q),
      );
    }
    return sortWatchlistRows(next, sort);
  }, [rows, market, trend, sort, stockSearchActive, activeQuery]);

  /*
   * 일정은 **내가 정한 순서**의 앞 12종목만 조회한다. 레일 카드와 행의 임박 표식이 이
   * 결과 하나를 나눠 쓴다.
   *
   * 화면에 보이는 순서(`visibleRows`)를 쓰지 않는 이유: 등락률 정렬이 켜져 있으면 5초
   * 폴링마다 상위 12종목이 바뀌고, 그때마다 실적 API를 열두 번씩 다시 부르게 된다.
   */
  const eventTargets = useMemo(
    () =>
      rows
        .slice(0, EVENT_SYMBOL_LIMIT)
        .map((r) => ({ symbol: r.symbol, name: r.quote?.name || r.symbol })),
    [rows],
  );
  const { events, bySymbol: eventsBySymbol } = useUpcomingEvents(eventTargets, EVENT_SYMBOL_LIMIT);

  const todaySummary = useMemo(
    () =>
      summarizeWatchlistToday(
        rows.map((r) => {
          const next = eventsBySymbol.get(r.symbol)?.[0];
          return {
            symbol: r.symbol,
            name: r.quote?.name || r.symbol,
            changeRate: r.quote && r.quote.currentPrice > 0 ? r.quote.changeRate : undefined,
            today: today.get(r.symbol.toUpperCase()),
            eventDays: next ? daysUntil(next.date) : null,
          };
        }),
      ),
    [rows, eventsBySymbol, today],
  );

  const openRow = openSymbol ? rows.find((r) => r.symbol === openSymbol) ?? null : null;

  return (
    <>
      <PageScrollShell>
        {/*
          0건 안내 카드가 이미 같은 버튼을 들고 있다 (§R-2). 두 곳에 다 두면 한 화면에
          「예시로 둘러보기」가 둘이 되어 무엇이 다른지 묻게 만든다 — 실측으로 빈
          보유 화면의 보이는 버튼이 그 문구 ×2 였다.
        */}
        <PageActions>
          {isEmpty ? null : (
            <SampleModeButton active={sample} onToggle={setSample} emphasize={!session} />
          )}
        </PageActions>
        {sample && (
          <div className="mb-3">
            <SampleRibbon
              loggedIn={Boolean(session)}
              loginHref="/login?next=%2Fwatchlist"
              onDismiss={() => setSample(false)}
              // 안내 카드가 들고 있던 첫 동작 — 카드가 예시 뒤로 물러났으니 여기 선다.
              action={{ label: "종목 찾아보기", href: "/analysis" }}
            />
          </div>
        )}

        {loadFailed ? (
          <LoadErrorState
            message={error ?? "관심종목을 불러오지 못했습니다."}
            retrying={listLoading}
            onRetry={() => void load()}
          />
        ) : isEmpty ? (
          <MenuEmptyState
            icon={menuIcon("watchlist")}
            headline={COPY.emptyHeadline}
            description={COPY.emptyDescription}
            steps={COPY.steps}
            primary={{ label: "종목 찾아보기", href: "/analysis" }}
            secondary={
              <SampleModeButton
                active={false}
                onToggle={() => setSample(true)}
                emphasize
                className="border border-[var(--sample-border)] px-3.5 py-1.5"
              />
            }
            footnote={COPY.emptyFootnote}
          />
        ) : (
        <PageColumns
          rail={
            <WatchlistSideRail
              rows={visibleRows}
              quotes={liveQuotes}
              events={events}
              adding={adding}
              onAddStock={(stock) => void handleAddStock(stock)}
            />
          }
        >
          <div className="space-y-3">
            {error && (
              <div className="rounded-lg border border-amber-500/30 bg-amber-500/10 px-4 py-2 text-sm text-amber-200">
                {error}
              </div>
            )}
            {notice && (
              <div className="rounded-lg border border-[var(--brand)]/30 bg-[var(--brand-muted)] px-4 py-2 text-sm text-[var(--text-primary)]">
                {notice}
              </div>
            )}

            {rows.length > 0 && <WatchlistTodayStrip summary={todaySummary} />}

            {rows.length > 0 && (
              <WatchlistToolbar
                sort={sort}
                onSortChange={setSort}
                market={market}
                onMarketChange={setMarket}
                trend={trend}
                onTrendChange={setTrend}
                counts={counts}
              />
            )}

            {listLoading && visibleRows.length === 0 ? (
              <Card className="flex items-center justify-center gap-2 p-8 text-sm text-[var(--text-muted)]">
                <Loader2 className="h-4 w-4 animate-spin opacity-70" />
                관심종목 불러오는 중…
              </Card>
            ) : visibleRows.length === 0 ? (
              <Card className="p-8 text-center text-sm text-[var(--text-muted)]">
                {rows.length === 0
                  ? "관심종목이 없습니다. 오른쪽 「종목 추가」에서 검색해 담아보세요."
                  : "조건에 맞는 종목이 없습니다."}
              </Card>
            ) : (
              <>
                {dragEnabled && rows.length > 1 && (
                  <p className="hidden text-cell text-[var(--text-muted)] sm:block">
                    {reorderSaving
                      ? "순서 저장 중…"
                      : "왼쪽 핸들을 드래그해 순서를 변경할 수 있습니다. 행을 누르면 상세가 열립니다."}
                  </p>
                )}
                {!dragEnabled && sort !== "manual" && (
                  <p className="text-cell text-[var(--text-muted)]">
                    보기 순서만 바꿉니다 — 「내 순서」로 돌아가면 내가 정한 순서가 그대로입니다.
                  </p>
                )}

                {/*
                  행의 칸이 접히는 기준 — 화면 폭이 아니라 **이 컬럼의 폭**이다.
                  좌측 사이드바와 우측 레일(300px)이 붙으면 1280px 화면에서도 여기는
                  600px대가 된다. `@container` 없이는 그 상태에서 행이 가로로 넘친다.
                  헤더 줄도 같은 컨테이너 안에 있어야 행과 같은 지점에서 칸을 접는다.
                */}
                <SampleWrap active={sample} className="mt-1">
                <div className="@container">
                  <WatchlistListHeader />
                  <div className="mt-1.5 space-y-1.5">
                  {visibleRows.map((row) => (
                    <WatchlistRow
                      key={row.symbol}
                      row={row}
                      pending={
                        isQuotePending(row.symbol, row.quote, pendingSymbols) ||
                        (quotesLoading && !row.quote?.watchlistMetrics)
                      }
                      draggable={dragEnabled}
                      isDragging={dragSymbol === row.symbol}
                      isDropTarget={dragSymbol != null && dragSymbol !== row.symbol && dragEnabled}
                      events={eventsBySymbol.get(row.symbol) ?? []}
                      today={today.get(row.symbol.toUpperCase())}
                      onOpen={() => setOpenSymbol(row.symbol)}
                      onRemove={() => void handleRemove(row.symbol)}
                      onDragStart={() => setDragSymbol(row.symbol)}
                      onDragEnd={() => setDragSymbol(null)}
                      onDragOver={(e) => {
                        if (!dragSymbol || !dragEnabled) return;
                        e.preventDefault();
                      }}
                      onDrop={(e) => {
                        e.preventDefault();
                        handleWatchlistDrop(row.symbol);
                      }}
                    />
                  ))}
                  </div>
                </div>
                </SampleWrap>
              </>
            )}
          </div>
        </PageColumns>
        )}
      </PageScrollShell>

      <WatchlistQuickLookModal
        row={openRow}
        score={openRow ? scores.get(openRow.symbol.toUpperCase()) ?? null : null}
        open={openSymbol != null}
        onClose={() => setOpenSymbol(null)}
        onRemove={(symbol) => void handleRemove(symbol)}
        onHoldingAdded={() => void refreshQuotes(symbols)}
      />
    </>
  );
}
