"use client";

import dynamic from "next/dynamic";
import { Suspense, useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { PageHeader } from "@/components/layout/header";
import { useHeaderSearch } from "@/components/providers/header-search-provider";
import { useShellMode } from "@/components/providers/shell-mode-provider";
import { MarketRadarList } from "@/components/market-radar/market-radar-list";
import { MarketRadarToolbar } from "@/components/market-radar/market-radar-toolbar";
import { QuotesSymbolListFooter } from "@/components/quotes/quotes-symbol-list";
import {
  parseMarketRadarFromSearchParams,
  syncMarketRadarUrl,
  useMarketRankings,
} from "@/hooks/use-market-rankings";
import type { MarketRadarParams } from "@/lib/market-radar-types";
import { marketRadarParamsKey, marketRadarSearchParams, parseMarketRadarSymbol } from "@/lib/market-radar-config";
import { cn } from "@/lib/utils";
import {
  MARKET_RADAR_WORKSPACE_TABS,
  type StockWorkspaceTabId,
} from "@/lib/stock-workspace-tabs";

const StockWorkspaceClient = dynamic(
  () =>
    import("@/components/stock-workspace/stock-workspace-client").then((m) => ({
      default: m.StockWorkspaceClient,
    })),
  {
    loading: () => (
      <div className="flex flex-1 items-center justify-center text-xs text-[var(--text-muted)]">
        차트 패널 로딩 중…
      </div>
    ),
  },
);

const MarketRadarTop5Chart = dynamic(
  () =>
    import("@/components/market-radar/market-radar-top5-chart").then((m) => ({
      default: m.MarketRadarTop5Chart,
    })),
  { ssr: false },
);

type MobilePanel = "list" | "detail";

function WorkspaceSkeleton() {
  return (
    <div className="flex flex-1 items-center justify-center text-xs text-[var(--text-muted)]">
      차트 패널 준비 중…
    </div>
  );
}

function MarketRadarPageInner() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const { registerStockPickHandler } = useHeaderSearch();
  const { isCompact } = useShellMode();

  const params = useMemo(
    () => parseMarketRadarFromSearchParams(searchParams),
    [searchParams],
  );

  const paramsKey = marketRadarParamsKey(params);
  const symbolParam = parseMarketRadarSymbol(searchParams);

  const [selectedSymbol, setSelectedSymbol] = useState("");
  const [workspaceTab, setWorkspaceTab] = useState<StockWorkspaceTabId>("overview");
  const [mobilePanel, setMobilePanel] = useState<MobilePanel>("list");
  const [showTop5, setShowTop5] = useState(false);

  const { data, items, loading, error, syncWatchlistSymbol } = useMarketRankings(params);

  useEffect(() => {
    if (!symbolParam) return;
    setSelectedSymbol(symbolParam);
    if (isCompact) setMobilePanel("detail");
  }, [symbolParam, isCompact]);

  useEffect(() => {
    if (symbolParam) return;
    if (items.length === 0) return;
    setSelectedSymbol((prev) =>
      prev && items.some((i) => i.symbol === prev) ? prev : items[0].symbol,
    );
  }, [items, symbolParam]);

  // Changing 시장/정렬/기간 sends a compact viewer back to the list — but only on
  // an actual change. Running on mount too would clobber a `?symbol=` deep link,
  // which used to leave shared mobile links stuck on the ranking list.
  const prevParamsKey = useRef(paramsKey);
  useEffect(() => {
    if (prevParamsKey.current === paramsKey) return;
    prevParamsKey.current = paramsKey;
    if (isCompact) setMobilePanel("list");
  }, [paramsKey, isCompact]);

  const selectSymbol = useCallback(
    (symbol: string) => {
      setSelectedSymbol(symbol);
      if (isCompact) setMobilePanel("detail");
      const q = marketRadarSearchParams(params);
      q.set("symbol", symbol);
      router.replace(`/market?${q.toString()}`, { scroll: false });
    },
    [isCompact, params, router],
  );

  useEffect(() => {
    registerStockPickHandler(selectSymbol);
    return () => registerStockPickHandler(null);
  }, [registerStockPickHandler, selectSymbol]);

  function updateParams(next: MarketRadarParams) {
    syncMarketRadarUrl(router, next, selectedSymbol || symbolParam || null);
  }

  const showWorkspace =
    Boolean(selectedSymbol) && (!isCompact || mobilePanel === "detail");

  const listPanel = (
    <div className="flex min-h-0 flex-1 flex-col overflow-hidden">
      <MarketRadarToolbar
        params={params}
        rankedAt={data?.rankedAt ?? null}
        refreshedAt={data?.refreshedAt ?? null}
        onChange={updateParams}
        compact={isCompact}
      />
      {error && (
        <p className="shrink-0 border-b border-[var(--border)] bg-rose-500/10 px-3 py-2 text-[11px] text-rose-400">
          {error}
        </p>
      )}
      <MarketRadarList
        items={items}
        selectedSymbol={selectedSymbol}
        rankingType={params.type}
        radarParams={params}
        loading={loading}
        compact={isCompact}
        onSelect={selectSymbol}
      />
      {items.length > 0 && (
        <button
          type="button"
          onClick={() => setShowTop5((v) => !v)}
          aria-expanded={showTop5}
          className={cn(
            "shrink-0 border-t border-[var(--border)] text-center text-[var(--text-muted)] hover:text-[var(--brand)]",
            // Comfortable tap target on touch; unobtrusive strip on desktop.
            isCompact ? "py-2.5 text-[12px]" : "py-1.5 text-[10px]",
          )}
        >
          {showTop5 ? "TOP5 차트 숨기기" : "TOP5 차트 보기"}
        </button>
      )}
      {showTop5 && items.length > 0 && (
        <div className="shrink-0">
          <MarketRadarTop5Chart items={items} compact={isCompact} />
        </div>
      )}
    </div>
  );

  if (isCompact) {
    return (
      <>
        <PageHeader title="실시간 차트" subtitle="실시간 순위 · 차트 · 시세" width="wide" />
        <main className="flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden">
          {mobilePanel === "list" ? (
            listPanel
          ) : showWorkspace ? (
            <>
              <StockWorkspaceClient
                symbol={selectedSymbol}
                embedded
                compact
                tabs={MARKET_RADAR_WORKSPACE_TABS}
                activeTab={workspaceTab}
                onTabChange={setWorkspaceTab}
                onBack={() => setMobilePanel("list")}
                onWatchlistChange={syncWatchlistSymbol}
                loginNext="/market"
                // Same breakpoint as the desktop branch: the compact/desktop
                // switch is a JS media query while the rail is a CSS one, and
                // they can disagree for a frame around 1024px.
                railFrom="xl"
                className="min-h-0 flex-1"
              />
              <div className="shrink-0 border-t border-[var(--border)] pb-[env(safe-area-inset-bottom,0px)]">
                <QuotesSymbolListFooter symbol={selectedSymbol} />
              </div>
            </>
          ) : (
            <WorkspaceSkeleton />
          )}
        </main>
      </>
    );
  }

  return (
    <>
      <PageHeader title="실시간 차트" subtitle="실시간 순위 · 차트 · 시세" width="wide" />
      <main className="flex min-h-0 min-w-0 flex-1 justify-center overflow-hidden">
        <div className="page-shell-wide flex h-full min-w-0 flex-row">
          <div className="flex h-full min-h-0 w-[min(100%,20rem)] min-w-[13.75rem] shrink-0 flex-col overflow-hidden border-r border-[var(--border)] lg:min-w-[17.5rem] 2xl:w-[23rem]">
            {listPanel}
          </div>
          {showWorkspace ? (
            <StockWorkspaceClient
              symbol={selectedSymbol}
              embedded
              tabs={MARKET_RADAR_WORKSPACE_TABS}
              activeTab={workspaceTab}
              onTabChange={setWorkspaceTab}
              onWatchlistChange={syncWatchlistSymbol}
              loginNext="/market"
              // The ranking list already owns ~320px here, so hold the metrics
              // rail until xl — at 1024–1279 it left tabs only ~208px.
              railFrom="xl"
              className="min-h-0 min-w-0 flex-1"
            />
          ) : (
            <div className="flex flex-1 items-center justify-center text-sm text-[var(--text-muted)]">
              {loading ? "랭킹 로딩 중…" : "종목을 선택하면 차트가 표시됩니다."}
            </div>
          )}
        </div>
      </main>
    </>
  );
}

export default function MarketRadarPage() {
  return (
    <Suspense
      fallback={
        <main className="flex flex-1 items-center justify-center p-8 text-sm text-[var(--text-muted)]">
          마켓 레이더 로딩 중…
        </main>
      }
    >
      <MarketRadarPageInner />
    </Suspense>
  );
}
