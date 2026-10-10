"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Loader2 } from "lucide-react";
import { useSampleMode } from "@/components/common/sample-mode";
import { PageColumns } from "@/components/layout/page-columns";
import { PageScrollShell } from "@/components/layout/page-content-shell";
import { StockCatalogSearchField } from "@/components/market/stock-catalog-search-field";
import { IndexPulseCard } from "@/components/market/index-pulse-card";
import { PersonalScheduleCard } from "@/components/earnings/personal-schedule-card";
import { useSymbolQuickLook } from "@/components/stocks/symbol-quick-look-modal";
import { SymbolNewsCard } from "@/components/stocks/symbol-news-card";
import { SectionCard } from "@/components/ui/section-card";
import { Card } from "@/components/ui/card";
import { WatchlistQuickLookModal } from "@/components/watchlist/watchlist-quick-look-modal";
import { WatchlistSinceAddedCard } from "@/components/watchlist/watchlist-since-added-card";
import type { SymbolRecord } from "@/components/watchlist/lab/wl-shared";
import { WlBench } from "@/components/watchlist/lab/wl-bench";
import { WlBrief } from "@/components/watchlist/lab/wl-brief";
import { WlBoard } from "@/components/watchlist/lab/wl-board";
import { WlList } from "@/components/watchlist/lab/wl-list";
import { WlSummaryBar } from "@/components/watchlist/lab/wl-summary-bar";
import { WlToolbar, type WlSort } from "@/components/watchlist/lab/wl-toolbar";
import { useQuoteStreamByMarket } from "@/hooks/use-quote-stream";
import { useVisibleInterval } from "@/hooks/use-visible-interval";
import { useUpcomingEvents } from "@/hooks/use-upcoming-events";
import { fetchJournalNotesClient } from "@/lib/journal-api";
import { useIsRailColumn } from "@/lib/layout-lab";
import { applyStreamPricesToQuotes, mergeQuoteSummariesForSymbols } from "@/lib/merge-quotes";
import {
  addWatchlistClient,
  fetchQuotesClient,
  fetchWatchlistClient,
  removeWatchlistClient,
} from "@/lib/market-api";
import { PRICE_POLL_MS, PRICE_POLL_MS_WHILE_STREAMING } from "@/lib/price-cadence";
import { fetchPriceTargetsClient, type PriceTargetRecord } from "@/lib/price-targets-api";
import type { PriceTarget } from "@/lib/price-targets";
import type { QuoteSummary, StockInfo, WatchlistItem } from "@/lib/quotes-types";
import {
  fetchSampleJournal,
  fetchSampleStamps,
  fetchSampleTargets,
  fetchSampleWatchlist,
  sampleWatchlistParts,
  type SampleTargetRow,
} from "@/lib/sample-api";
import { fetchStampsClient } from "@/lib/stamp-api";
import { fetchScoresBatchClient, type StockScoreData } from "@/lib/stock-score-api";
import {
  buildWatchlistRows,
  filterWatchlistRows,
  isUsRow,
  type WatchlistMarketFilter,
  type WatchlistRow,
} from "@/lib/watchlist-calc";
import {
  attentionScore,
  buildWatchSignals,
  watchBucket,
  type WatchBucket,
  type WatchSignal,
} from "@/lib/watchlist-signals";

/**
 * 관심종목 개편 시안의 **공용 몸통** (2026-09-07).
 *
 * 시안 넷이 이 파일 하나를 나눠 쓴다. 데이터를 받아 오는 방식·신호를 만드는 규칙·정렬과
 * 필터가 넷 다 같아야, 사용자가 보는 차이가 정확히 **배치의 차이**가 된다. 지난 라운드
 * (09-06)에서 배운 것이 이것이다 — 시안마다 다시 그린 목업은 비교 대상이 아니라 열화판이
 * 된다. 그래서 값은 실서비스와 같은 경로로 받고(같은 `lib/*-api`), 조립도 실서비스와 같은
 * 함수(`buildWatchlistRows`)를 쓴다.
 *
 * # 요청은 몇 개 늘어나는가
 *
 * 목록·시세·점수·일정은 지금 화면이 이미 부르고 있다. 여기서 더하는 것은 **목록당 1회씩
 * 셋**뿐이다 — 목표가·메모·스탬프. 종목 수와 무관하게 각각 한 요청이고, 그 셋이 "내가 이
 * 종목에 무엇을 해 뒀는가"를 목록에서 처음으로 보이게 만든다.
 *
 * 스파크라인만 종목당 1회인데, 화면에 들어온 카드만 부른다(`useInView` + 공용 큐).
 *
 * # 확정되면
 *
 * 고른 배치를 `watchlist-client.tsx`에 옮기고 이 폴더와 `dev/samples` 등록을 지운다.
 * `lib/watchlist-signals.ts`는 남는다 — 그건 배치가 아니라 판정이라 어느 시안을 골라도 쓴다.
 */

export type LabVariant = "signal" | "board" | "brief" | "bench";

/** 일정·뉴스는 종목당 비용이 있어 위쪽부터 자른다 — 지금 화면과 같은 상한. */
const EVENT_SYMBOL_LIMIT = 12;

function toTarget(row: PriceTargetRecord | SampleTargetRow): PriceTarget {
  return {
    id: String(row.id),
    symbol: row.symbol,
    targetPrice: row.targetPrice,
    targetPrice2: row.targetPrice2,
    stopPrice: row.stopPrice,
    basePrice: row.basePrice,
    baseDate: row.baseDate,
    reachedDate: row.reachedDate,
    peakPrice: row.peakPrice,
    troughPrice: row.troughPrice,
    currency: row.currency === "USD" ? "USD" : "KRW",
    note: row.note,
    createdAt: row.createdAt,
  };
}

function countBySymbol(rows: Array<{ symbol?: string }>): Map<string, number> {
  const out = new Map<string, number>();
  for (const row of rows) {
    const key = row.symbol?.toUpperCase();
    if (!key) continue;
    out.set(key, (out.get(key) ?? 0) + 1);
  }
  return out;
}

export function WatchlistLabClient({ variant }: { variant: LabVariant }) {
  const { active: sample } = useSampleMode();
  /*
    시안 D의 2단은 **오른쪽 기둥을 세울 폭이 있을 때만**이다. 판정을 D 안이 아니라
    여기서 하는 이유: 좁은 화면의 D는 나머지 셋과 같은 화면(목록 + 우측 레일)이어야
    하는데, D가 스스로 판정하면 레일이 통째로 사라진다(실측 375px에서 종목 추가·일정·
    뉴스 카드가 하나도 없었다).
  */
  const twoPane = useIsRailColumn();

  const [items, setItems] = useState<WatchlistItem[]>([]);
  const [quotes, setQuotes] = useState<QuoteSummary[]>([]);
  const [scores, setScores] = useState<Map<string, StockScoreData>>(() => new Map());
  const [targets, setTargets] = useState<PriceTarget[]>([]);
  const [noteCounts, setNoteCounts] = useState<Map<string, number>>(() => new Map());
  const [stampCounts, setStampCounts] = useState<Map<string, number>>(() => new Map());
  const [loading, setLoading] = useState(true);
  const [openSymbol, setOpenSymbol] = useState<string | null>(null);

  const [sort, setSort] = useState<WlSort>("attention");
  const [market, setMarket] = useState<WatchlistMarketFilter>("all");
  const [onlySignal, setOnlySignal] = useState(false);

  const quotesGenRef = useRef(0);
  const symbols = useMemo(() => items.map((i) => i.symbol), [items]);

  const refreshQuotes = useCallback(async (symbolList: string[]) => {
    if (symbolList.length === 0) return;
    const gen = ++quotesGenRef.current;
    try {
      const next = await fetchQuotesClient("watchlist");
      if (gen !== quotesGenRef.current) return;
      setQuotes((prev) => mergeQuoteSummariesForSymbols(symbolList, prev, next ?? []));
    } catch {
      // 마지막으로 합쳐 둔 시세를 그대로 쓴다.
    }
  }, []);

  /**
   * 내 기록 셋(목표가·메모·스탬프). 실패해도 목록은 그대로 쓸 수 있어야 하므로
   * 하나씩 삼킨다 — 관심종목 화면이 목표가 API 때문에 통째로 비면 안 된다.
   */
  const loadRecords = useCallback(async (isSample: boolean) => {
    const [t, n, s] = await Promise.all([
      (isSample ? fetchSampleTargets() : fetchPriceTargetsClient()).catch(() => []),
      (isSample ? fetchSampleJournal() : fetchJournalNotesClient()).catch(() => []),
      (isSample ? fetchSampleStamps() : fetchStampsClient()).catch(() => []),
    ]);
    setTargets(t.map(toTarget));
    setNoteCounts(countBySymbol(n));
    setStampCounts(countBySymbol(s));
  }, []);

  const load = useCallback(async () => {
    if (sample) {
      const rows = await fetchSampleWatchlist().catch(() => []);
      const parts = sampleWatchlistParts(rows);
      setItems(parts.items as WatchlistItem[]);
      setQuotes(parts.quotes as unknown as QuoteSummary[]);
      setScores(parts.scores as Map<string, StockScoreData>);
      await loadRecords(true);
      setLoading(false);
      return;
    }
    const list = (await fetchWatchlistClient().catch(() => [])) ?? [];
    setItems(list);
    const next = list.map((i) => i.symbol);
    await Promise.all([
      refreshQuotes(next),
      fetchScoresBatchClient(next)
        .then((rows) => setScores(new Map(rows.map((r) => [r.symbol.toUpperCase(), r]))))
        .catch(() => {}),
      loadRecords(false),
    ]);
    setLoading(false);
  }, [sample, refreshQuotes, loadRecords]);

  useEffect(() => {
    void (async () => {
      await load();
    })();
  }, [load]);

  const streamRows = useMemo(
    () =>
      sample ? [] : quotes.map((q) => ({ symbol: q.symbol, assetType: q.assetType as string })),
    [quotes, sample],
  );
  const { prices: streamPrices, streaming } = useQuoteStreamByMarket(streamRows);
  const liveQuotes = useMemo(
    () => applyStreamPricesToQuotes(quotes, streamPrices),
    [quotes, streamPrices],
  );

  // 안 보이는 탭에서는 돌지 않는다 (2026-09-15 사용자 요청 3 — `useVisibleInterval`).
  useVisibleInterval(
    () => void refreshQuotes(symbols),
    streaming ? PRICE_POLL_MS_WHILE_STREAMING : PRICE_POLL_MS,
    !sample && symbols.length > 0,
  );

  const rows = useMemo(
    () => buildWatchlistRows(items, liveQuotes, scores),
    [items, liveQuotes, scores],
  );

  const eventTargets = useMemo(
    () =>
      rows.slice(0, EVENT_SYMBOL_LIMIT).map((r) => ({
        symbol: r.symbol,
        name: r.quote?.name || r.symbol,
      })),
    [rows],
  );
  const { events, bySymbol: eventsBySymbol } = useUpcomingEvents(eventTargets, EVENT_SYMBOL_LIMIT);

  const targetBySymbol = useMemo(
    () => new Map(targets.map((t) => [t.symbol.toUpperCase(), t])),
    [targets],
  );

  const records = useMemo(() => {
    const out = new Map<string, SymbolRecord>();
    for (const row of rows) {
      const key = row.symbol.toUpperCase();
      out.set(row.symbol, {
        target: targetBySymbol.get(key),
        notes: noteCounts.get(key) ?? 0,
        stamps: stampCounts.get(key) ?? 0,
        held: Boolean(row.quote?.isHeld),
      });
    }
    return out;
  }, [rows, targetBySymbol, noteCounts, stampCounts]);

  const signals = useMemo(() => {
    const out = new Map<string, WatchSignal[]>();
    for (const row of rows) {
      out.set(
        row.symbol,
        buildWatchSignals({
          row,
          events: eventsBySymbol.get(row.symbol) ?? [],
          target: targetBySymbol.get(row.symbol.toUpperCase()),
        }),
      );
    }
    return out;
  }, [rows, eventsBySymbol, targetBySymbol]);

  const counts = useMemo(
    () => ({
      all: rows.length,
      domestic: rows.filter((r) => r.quote && !isUsRow(r)).length,
      us: rows.filter((r) => isUsRow(r)).length,
      signal: rows.filter((r) => (signals.get(r.symbol)?.length ?? 0) > 0).length,
    }),
    [rows, signals],
  );

  const visibleRows = useMemo(() => {
    let next = filterWatchlistRows(rows, market, "all");
    if (onlySignal) next = next.filter((r) => (signals.get(r.symbol)?.length ?? 0) > 0);
    if (sort === "manual") return next;
    const sorted = [...next];
    sorted.sort((a, b) => {
      switch (sort) {
        case "attention":
          return (
            attentionScore(signals.get(b.symbol) ?? []) -
            attentionScore(signals.get(a.symbol) ?? [])
          );
        case "change":
          return (b.quote?.changeRate ?? -Infinity) - (a.quote?.changeRate ?? -Infinity);
        case "since":
          return (b.sinceAddedRate ?? -Infinity) - (a.sinceAddedRate ?? -Infinity);
        case "score":
          return (b.score ?? -Infinity) - (a.score ?? -Infinity);
        default:
          return 0;
      }
    });
    return sorted;
  }, [rows, market, onlySignal, sort, signals]);

  const buckets = useMemo(() => {
    const out = new Map<WatchBucket, WatchlistRow[]>([
      ["action", []],
      ["watch", []],
      ["quiet", []],
    ]);
    for (const row of visibleRows) {
      out.get(watchBucket(signals.get(row.symbol) ?? []))!.push(row);
    }
    return out;
  }, [visibleRows, signals]);

  async function handleRemove(symbol: string) {
    if (sample) {
      setItems((prev) => prev.filter((i) => i.symbol !== symbol));
      return;
    }
    await removeWatchlistClient(symbol).catch(() => {});
    await load();
  }

  async function handleAdd(stock: StockInfo) {
    if (sample) return;
    await addWatchlistClient(stock.symbol).catch(() => {});
    await load();
  }

  const openRow = openSymbol ? rows.find((r) => r.symbol === openSymbol) ?? null : null;

  const listProps = {
    rows: visibleRows,
    records,
    signals,
    onOpen: setOpenSymbol,
    onRemove: (symbol: string) => void handleRemove(symbol),
  };

  const body = loading ? (
    <Card className="flex items-center justify-center gap-2 p-8 text-sm text-[var(--text-muted)]">
      <Loader2 className="h-4 w-4 animate-spin opacity-70" />
      관심종목 불러오는 중…
    </Card>
  ) : visibleRows.length === 0 ? (
    <Card className="p-8 text-center text-sm text-[var(--text-muted)]">
      조건에 맞는 종목이 없습니다.
    </Card>
  ) : variant === "board" ? (
    <WlBoard {...listProps} />
  ) : variant === "brief" ? (
    <WlBrief {...listProps} buckets={buckets} />
  ) : (
    <WlList {...listProps} />
  );

  if (variant === "bench" && twoPane) {
    return (
      <>
        <PageScrollShell>
          <WlBench
            {...listProps}
            scores={scores}
            eventsBySymbol={eventsBySymbol}
            summary={<WlSummaryBar rows={rows} signals={signals} />}
            toolbar={
              <WlToolbar
                sort={sort}
                onSortChange={setSort}
                market={market}
                onMarketChange={setMarket}
                onlySignal={onlySignal}
                onOnlySignalChange={setOnlySignal}
                counts={counts}
              />
            }
          />
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

  return (
    <>
      <PageScrollShell>
        <PageColumns
          rail={
            <LabRail
              rows={visibleRows}
              events={events}
              onAdd={(stock) => void handleAdd(stock)}
            />
          }
        >
          <div className="space-y-2.5">
            <WlSummaryBar rows={rows} signals={signals} />
            <WlToolbar
              sort={sort}
              onSortChange={setSort}
              market={market}
              onMarketChange={setMarket}
              onlySignal={onlySignal}
              onOnlySignalChange={setOnlySignal}
              counts={counts}
            />
            {body}
          </div>
        </PageColumns>
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

/**
 * 시안의 우측 레일.
 *
 * 지금 레일에서 **두 장을 뺐다** — 「오늘의 관심종목」(상승/보합/하락·평균)은 본문 맨 위
 * 요약 줄이 같은 것을 말하고, 「오늘 많이 움직인 종목」은 신호 칩이 그 종목을 목록 안에서
 * 직접 가리키므로 옆에 다시 세울 이유가 없다. 남은 넷은 본문이 답하지 않는 것들이다.
 */
function LabRail({
  rows,
  events,
  onAdd,
}: {
  rows: WatchlistRow[];
  events: ReturnType<typeof useUpcomingEvents>["events"];
  onAdd: (stock: StockInfo) => void;
}) {
  const quickLook = useSymbolQuickLook();
  const newsTargets = useMemo(
    () =>
      rows
        .filter((r) => r.quote)
        .map((r) => ({
          symbol: r.symbol,
          name: r.quote?.name || r.symbol,
          market: (r.quote?.currency === "USD" ? "us" : "domestic") as "domestic" | "us",
        })),
    [rows],
  );

  return (
    <>
      <SectionCard title="종목 추가">
        <p className="mb-1.5 text-cell text-muted-foreground">
          종목명 또는 티커로 검색한 뒤 목록에서 선택하세요.
        </p>
        <StockCatalogSearchField suggestionsId="wl-lab-suggestions" onSelect={onAdd} />
      </SectionCard>

      {rows.length > 0 && <WatchlistSinceAddedCard rows={rows} onPick={quickLook.open} />}
      {rows.length > 0 && <PersonalScheduleCard events={events} onPick={quickLook.open} />}
      {newsTargets.length > 0 && (
        <SymbolNewsCard symbols={newsTargets} title="관심종목 뉴스" subjectLabel="관심종목 상위" />
      )}
      <IndexPulseCard />

      {quickLook.modal}
    </>
  );
}
