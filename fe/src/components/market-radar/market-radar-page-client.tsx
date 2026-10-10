"use client";

import dynamic from "next/dynamic";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { useHeaderSearch } from "@/components/providers/header-search-provider";
import { useShellMode } from "@/components/providers/shell-mode-provider";
import { MarketRadarList } from "@/components/market-radar/market-radar-list";
import { MarketRadarToolbar } from "@/components/market-radar/market-radar-toolbar";
import {
  parseMarketRadarFromSearchParams,
  syncMarketRadarUrl,
  useMarketRankings,
} from "@/hooks/use-market-rankings";
import type { MarketRadarParams, RankingsResponse } from "@/lib/market-radar-types";
import {
  compactShowsDetail,
  shouldHistoryBackOnCompactBack,
  shouldPinListOnParamsChange,
  symbolSelectNavMode,
} from "@/lib/market-radar-mobile-nav";
import { marketRadarParamsKey, marketRadarSearchParams, parseMarketRadarSymbol } from "@/lib/market-radar-config";

import {
  type StockWorkspaceTabId,
} from "@/lib/stock-workspace-tabs";
import { cn } from "@/lib/utils";

const StockWorkspaceClient = dynamic(
  () =>
    import("@/components/stock-workspace/stock-workspace-client").then((m) => ({
      default: m.StockWorkspaceClient,
    })),
  {
    loading: () => (
      <div className="flex flex-1 items-center justify-center text-cell text-[var(--text-muted)]">
        종목 화면 로딩 중…
      </div>
    ),
  },
);

function WorkspaceSkeleton() {
  return (
    <div className="flex flex-1 items-center justify-center text-cell text-[var(--text-muted)]">
      종목 화면 준비 중…
    </div>
  );
}

/**
 * 마켓 레이더 화면 본체.
 *
 * `app/(main)/market/page.tsx`에서 통째로 옮겨 왔다 (2026-09-22). 그 파일이
 * `"use client"`였던 동안 이 라우트의 서버 HTML에는 데이터가 한 글자도 없었다 —
 * 실측은 `lib/market-api.server.ts` 머리글. 페이지를 서버 컴포넌트로 되돌리려면
 * 훅을 쓰는 부분이 별도 클라이언트 모듈이어야 한다.
 *
 * @param seed 서버가 미리 받아 둔 첫 목록. 기본 손잡이로 들어온 요청에만 붙는다.
 */
export function MarketRadarPageClient({ seed }: { seed?: RankingsResponse | null }) {
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
  /*
    시장·정렬·기간을 바꿨을 때 URL에 symbol이 남아 있어도 compact는 목록을
    보여 준다. deep link(`?symbol=`) 첫 진입에는 false — paramsKey effect가
    마운트 때는 건너뛴다.
  */
  const [listPinned, setListPinned] = useState(false);

  const { data, items, loading, error, pricesAt, syncWatchlistSymbol } =
    useMarketRankings(params, seed);

  const showCompactDetail = compactShowsDetail(isCompact, symbolParam, listPinned);
  const workspaceSymbol = symbolParam || selectedSymbol;

  useEffect(() => {
    if (symbolParam) setSelectedSymbol(symbolParam);
  }, [symbolParam]);

  /*
    뒤로 가기로 `?symbol=` 없는 첫 주소에 돌아오면 처음 보던 것(목록 1위)을 다시 연다
    (2026-10-09 — 데스크톱 선택이 push라 이 전이가 생긴다). 안 그러면 마지막에 고른
    종목이 패널에 남아 「이전에 보던 종목」이 아닌 것이 보인다.
  */
  const hadSymbolParam = useRef(Boolean(symbolParam));
  useEffect(() => {
    const leftSymbol = hadSymbolParam.current && !symbolParam;
    hadSymbolParam.current = Boolean(symbolParam);
    if (symbolParam) return;
    if (items.length === 0) return;
    setSelectedSymbol((prev) =>
      !leftSymbol && prev && items.some((i) => i.symbol === prev) ? prev : items[0].symbol,
    );
  }, [items, symbolParam]);

  const prevParamsKey = useRef(paramsKey);
  useEffect(() => {
    if (!shouldPinListOnParamsChange(prevParamsKey.current, paramsKey, isCompact)) {
      prevParamsKey.current = paramsKey;
      return;
    }
    prevParamsKey.current = paramsKey;
    setListPinned(true);
  }, [paramsKey, isCompact]);

  const selectSymbol = useCallback(
    (symbol: string) => {
      setSelectedSymbol(symbol);
      setListPinned(false);

      const q = marketRadarSearchParams(params);
      q.set("symbol", symbol);
      const url = `/market?${q.toString()}`;
      const nav = symbolSelectNavMode(isCompact, symbolParam);

      if (nav === "push") router.push(url, { scroll: false });
      else router.replace(url, { scroll: false });
    },
    [isCompact, params, router, symbolParam],
  );

  const backToRankingList = useCallback(() => {
    if (shouldHistoryBackOnCompactBack(symbolParam)) router.back();
  }, [router, symbolParam]);

  useEffect(() => {
    registerStockPickHandler(selectSymbol);
    return () => registerStockPickHandler(null);
  }, [registerStockPickHandler, selectSymbol]);

  function updateParams(next: MarketRadarParams) {
    syncMarketRadarUrl(router, next, selectedSymbol || symbolParam || null);
  }

  const showWorkspace =
    Boolean(workspaceSymbol) && (!isCompact || showCompactDetail);

  const listPanel = (
    <div className="flex min-h-0 flex-1 flex-col overflow-hidden">
      <MarketRadarToolbar
        params={params}
        rankedAt={data?.rankedAt ?? null}
        refreshedAt={data?.refreshedAt ?? null}
        pricesAt={pricesAt}
        sessionClosedAt={data?.sessionClosedAt ?? null}
        onChange={updateParams}
        compact={isCompact}
      />
      {error && (
        <p className="shrink-0 border-b border-[var(--border)] bg-rose-500/10 px-3 py-2 text-cell text-rose-400">
          {error}
        </p>
      )}
      <MarketRadarList
        items={items}
        selectedSymbol={selectedSymbol}
        radarParams={params}
        loading={loading}
        compact={isCompact}
        onSelect={selectSymbol}
        onWatchlistChange={syncWatchlistSymbol}
      />
    </div>
  );

  if (isCompact) {
    return (
      <main className="relative flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden">
        {/*
          compact 목록은 **항상 마운트**한다. 상세일 때 `hidden`만 걸어 스크롤·DOM을
          유지 — 뒤로가면 remount 없이 즉시 보인다. `inert`로 숨겨진 목록이 포커스를
          받지 않게 한다.
        */}
        <div
          className={cn(
            "flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden",
            showCompactDetail && "hidden",
          )}
          aria-hidden={showCompactDetail}
          {...(showCompactDetail ? { inert: true as const } : {})}
        >
          {listPanel}
        </div>

        {showCompactDetail && (
          <div className="absolute inset-0 z-10 flex min-h-0 flex-col overflow-hidden bg-[var(--background)]">
            {/*
              하단의 「밸류에이션 →」 전폭 링크는 뺐다(2026-09-03 사용자 결정).
              좁은 화면에서 상시로 한 줄을 차지했는데, 같은 곳은 바로 위 탭바의
              밸류에이션 탭으로 이미 한 번에 닿는다.
            */}
            {showWorkspace ? (
              <StockWorkspaceClient
                symbol={workspaceSymbol}
                embedded
                compact
                activeTab={workspaceTab}
                onTabChange={setWorkspaceTab}
                onBack={backToRankingList}
                railFrom="xl"
                className="min-h-0 flex-1"
              />
            ) : (
              <WorkspaceSkeleton />
            )}
          </div>
        )}
      </main>
    );
  }

  return (
    <main className="flex min-h-0 min-w-0 flex-1 justify-center overflow-hidden">
      <div className="page-shell-wide flex h-full min-w-0 flex-row">
        <div className="flex h-full min-h-0 w-[min(100%,20rem)] min-w-[13.75rem] shrink-0 flex-col overflow-hidden border-r border-[var(--border)] lg:min-w-[17.5rem] 2xl:w-[23rem]">
          {listPanel}
        </div>
        {showWorkspace ? (
          <StockWorkspaceClient
            symbol={workspaceSymbol}
            embedded
            activeTab={workspaceTab}
            onTabChange={setWorkspaceTab}
            railFrom="xl"
            className="min-h-0 min-w-0 flex-1"
          />
        ) : (
          <div className="flex flex-1 items-center justify-center text-sm text-[var(--text-muted)]">
            {loading ? "랭킹 로딩 중…" : "종목을 선택하세요."}
          </div>
        )}
      </div>
    </main>
  );
}
