"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Loader2 } from "lucide-react";
import { MenuEmptyState } from "@/components/common/menu-empty-state";
import { menuIcon } from "@/components/common/menu-icon";
import {
  SampleModeButton,
  SampleRibbon,
  SampleWrap,
  useSampleMode,
} from "@/components/common/sample-mode";
import { useAuth } from "@/components/providers/auth-provider";
import { PageHeader } from "@/components/layout/header";
import { PageColumns } from "@/components/layout/page-columns";
import { PageScrollShell } from "@/components/layout/page-content-shell";
import { WatchlistListHeader } from "@/components/watchlist/watchlist-columns";
import { WatchlistQuickLookModal } from "@/components/watchlist/watchlist-quick-look-modal";
import { WatchlistRow } from "@/components/watchlist/watchlist-row";
import { WatchlistSideRail } from "@/components/watchlist/watchlist-side-rail";
import { WatchlistToolbar } from "@/components/watchlist/watchlist-toolbar";
import { useHeaderSearch } from "@/components/providers/header-search-provider";
import { useUpcomingEvents } from "@/hooks/use-upcoming-events";
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
import { mergeQuoteSummariesForSymbols } from "@/lib/merge-quotes";
import {
  buildWatchlistRows,
  filterWatchlistRows,
  isUsRow,
  sortWatchlistRows,
  type WatchlistMarketFilter,
  type WatchlistSort,
  type WatchlistTrendFilter,
} from "@/lib/watchlist-calc";

const COPY = MENU_COPY.watchlist;

const POLL_MS = 5_000;
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
      setError(err instanceof Error ? err.message : "목록 로드 실패");
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

  useEffect(() => {
    // 예시는 고정 값이다 — 폴링하면 인증 뒤의 시세 API를 헛되이 두드린다.
    if (sample || symbols.length === 0) return;
    const id = window.setInterval(() => {
      void refreshQuotes(symbols);
    }, POLL_MS);
    return () => window.clearInterval(id);
  }, [sample, symbols, refreshQuotes]);

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
      setError(err instanceof Error ? err.message : "추가 실패");
    } finally {
      setAdding(false);
    }
  }

  async function handleRemove(symbol: string) {
    try {
      await removeWatchlistClient(symbol);
      setPendingSymbols((prev) => {
        if (!prev.has(symbol)) return prev;
        const next = new Set(prev);
        next.delete(symbol);
        return next;
      });
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : "삭제 실패");
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
      setError(err instanceof Error ? err.message : "순서 저장 실패");
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
    () => buildWatchlistRows(items, quotes, scores),
    [items, quotes, scores],
  );

  const isEmpty = !sample && !listLoading && !error && items.length === 0;

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

  const openRow = openSymbol ? rows.find((r) => r.symbol === openSymbol) ?? null : null;

  return (
    <>
      <PageHeader
        title={COPY.title}
        subtitle={COPY.subtitle}
        actions={
          <SampleModeButton
            active={sample}
            onToggle={setSample}
            emphasize={isEmpty || !session}
          />
        }
      />
      <PageScrollShell>
        {sample && (
          <div className="mb-3">
            <SampleRibbon
              loggedIn={Boolean(session)}
              loginHref="/login?next=%2Fwatchlist"
              onDismiss={() => setSample(false)}
            />
          </div>
        )}

        {isEmpty ? (
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
              quotes={quotes}
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
                  <p className="hidden text-xs text-[var(--text-muted)] sm:block">
                    {reorderSaving
                      ? "순서 저장 중…"
                      : "왼쪽 핸들을 드래그해 순서를 변경할 수 있습니다. 행을 누르면 상세가 열립니다."}
                  </p>
                )}
                {!dragEnabled && sort !== "manual" && (
                  <p className="text-xs text-[var(--text-muted)]">
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
