"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { ScoreRankingList } from "@/components/analysis/score-ranking-list";
import { StockWorkspaceClient } from "@/components/stock-workspace/stock-workspace-client";
import { useHeaderSearch } from "@/components/providers/header-search-provider";
import { useShellMode } from "@/components/providers/shell-mode-provider";
import { analysisSymbolHref } from "@/lib/analysis-tabs";
import { parseMarketRadarSymbol } from "@/lib/market-radar-config";
import {
  parseScoreRankingParams,
  scoreRankingSearchParams,
  type ScoreRankingParams,
} from "@/lib/score-ranking-config";
import {
  compactShowsDetail,
  shouldHistoryBackOnCompactBack,
  symbolSelectNavMode,
} from "@/lib/market-radar-mobile-nav";
import { parseStockWorkspaceTab, type StockWorkspaceTabId } from "@/lib/stock-workspace-tabs";
import { cn } from "@/lib/utils";

type AnalysisClientProps = {
  defaultSymbol: string | null;
};

/**
 * AI 추천 랭킹 — **왼쪽 목록 · 오른쪽 종목**, 실시간 차트와 같은 배치.
 *
 * # 왜 다시 지었나 (2026-09-03 사용자 결정)
 *
 * 같은 종목을 어디서 눌렀느냐에 따라 **다른 화면**이 나왔다. 실시간 차트에서 누르면
 * 개요·AI·차트·호가까지 있는 워크스페이스가 뜨는데, 이 화면에서 누르면 자체 헤더와
 * 자체 탭바(AI 분석부터 시작하는 일곱 개)에 오른쪽 사이드 레일이 붙은 **다른 것**이
 * 떴다. 종목 정보를 받아 오는 코드도 두 벌이라(각자 `fetchStockClient` ·
 * `fetchQuoteDetailClient` · `fetchFundamentalsClient`를 불렀다) 같은 종목을 두 화면이
 * 각자 물어보고 있었다.
 *
 * 지금은 **오른쪽이 통째로 `StockWorkspaceClient`**다. 종목 화면을 고칠 자리가 하나뿐이
 * 되고, 요청도 그 한 벌만 나간다.
 *
 * # 마켓 레이더와 같은 규칙을 쓴다
 *
 * 목록↔상세를 오가는 규칙(`lib/market-radar-mobile-nav`)은 그대로 가져다 쓴다 —
 * 좁은 화면에서 종목을 고르면 히스토리를 쌓아 뒤로가기가 목록으로 돌아오고, 넓은
 * 화면에서는 주소만 바꿔 오른쪽 칸을 갈아 끼운다. 두 화면이 다른 규칙을 쓰면
 * 「랭킹에서는 뒤로가기가 앱을 나간다」 같은 차이가 다시 생긴다.
 */
export function AnalysisClient({ defaultSymbol }: AnalysisClientProps) {
  const router = useRouter();
  const searchParams = useSearchParams();
  const { isCompact } = useShellMode();
  const { registerStockPickHandler } = useHeaderSearch();

  // 빈 문자열이 「없음」이다 — 아래 목록↔상세 헬퍼들이 그 규약을 쓴다.
  const symbolParam = parseMarketRadarSymbol(searchParams);
  const tab = parseStockWorkspaceTab(searchParams.get("tab"));

  /*
    목록의 세 손잡이(시장·축·기간)도 **주소에 산다.** 실시간 차트가 그렇게 하고 있고,
    그래야 「해외 · 성장 · 최근 1주」 목록을 그대로 링크로 보낼 수 있다. 컴포넌트
    상태로 두면 새로고침 한 번에 국내·종합·실시간으로 되돌아간다.
  */
  const rankingParams = useMemo(
    () => parseScoreRankingParams(searchParams),
    [searchParams],
  );

  /*
    시장·정렬을 바꾼 뒤 목록을 붙잡아 두는 자리(마켓 레이더의 `listPinned`와 같은 뜻).
    좁은 화면에서 「목록으로」를 눌렀는데 히스토리가 없을 때도 이걸로 돌아간다.
  */
  const [listPinned, setListPinned] = useState(false);

  const showCompactDetail = compactShowsDetail(isCompact, symbolParam, listPinned);
  /*
    좁은 화면에서는 **기본 종목을 열지 않는다.** 주소에 종목이 없으면 사용자는
    아직 아무것도 고르지 않은 것이고, 그때 화면에 있어야 하는 것은 목록이다.
    넓은 화면은 오른쪽 칸이 비어 있으면 허전하므로 기본 종목을 채운다.
  */
  const workspaceSymbol = symbolParam || (isCompact ? "" : defaultSymbol ?? "");

  // 지금 목록이 무엇을 보고 있는지 — 종목·탭을 바꾸는 모든 이동이 이걸 함께 싣는다.
  const rankingQuery = useMemo(
    () => scoreRankingSearchParams(rankingParams),
    [rankingParams],
  );

  const selectSymbol = useCallback(
    (symbol: string) => {
      setListPinned(false);
      const url = analysisSymbolHref(symbol, tab, rankingQuery);
      const nav = symbolSelectNavMode(isCompact, symbolParam);
      if (nav === "push") router.push(url, { scroll: false });
      else router.replace(url, { scroll: false });
    },
    [isCompact, rankingQuery, router, symbolParam, tab],
  );

  const setTab = useCallback(
    (next: StockWorkspaceTabId) => {
      if (!workspaceSymbol) return;
      router.replace(analysisSymbolHref(workspaceSymbol, next, rankingQuery), {
        scroll: false,
      });
    },
    [rankingQuery, router, workspaceSymbol],
  );

  /*
    목록의 손잡이를 바꾸면 **주소를 갈아 끼운다**(히스토리를 쌓지 않는다 — 축을 여섯 번
    눌러 본 사람이 뒤로가기를 여섯 번 해야 나가는 화면이 되면 안 된다).

    좁은 화면에서는 목록을 붙잡아 둔다. 상세를 보다가 축을 바꿨다면 보고 싶은 것은
    새 목록이지, 방금 그 종목의 상세가 아니다(마켓 레이더의 `listPinned`와 같은 판단).
  */
  const setRankingParams = useCallback(
    (next: ScoreRankingParams) => {
      if (isCompact) setListPinned(true);
      const q = scoreRankingSearchParams(next);
      if (symbolParam) {
        q.set("symbol", symbolParam);
        q.set("tab", tab);
      }
      const query = q.toString();
      router.replace(query ? `/analysis?${query}` : "/analysis", { scroll: false });
    },
    [isCompact, router, symbolParam, tab],
  );

  const backToList = useCallback(() => {
    if (shouldHistoryBackOnCompactBack(symbolParam)) {
      router.back();
      return;
    }
    setListPinned(true);
  }, [router, symbolParam]);

  useEffect(() => {
    registerStockPickHandler(selectSymbol);
    return () => registerStockPickHandler(null);
  }, [registerStockPickHandler, selectSymbol]);

  const listPanel = (
    <ScoreRankingList
      params={rankingParams}
      onParamsChange={setRankingParams}
      activeSymbol={workspaceSymbol}
      onSelect={selectSymbol}
      compact={isCompact}
      className="min-h-0 flex-1"
    />
  );

  if (isCompact) {
    return (
      <main className="relative flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden">
        {/*
          목록은 **항상 마운트**한다. 상세일 때 `hidden`만 걸어 스크롤·DOM을 유지 —
          뒤로가면 remount 없이 즉시 보인다(마켓 레이더와 같은 처리).
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

        {showCompactDetail && workspaceSymbol && (
          <div className="absolute inset-0 z-10 flex min-h-0 flex-col overflow-hidden bg-[var(--background)]">
            <StockWorkspaceClient
              symbol={workspaceSymbol}
              embedded
              compact
              activeTab={tab}
              onTabChange={setTab}
              onBack={backToList}
              railFrom="xl"
              className="min-h-0 flex-1"
            />
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
        {workspaceSymbol ? (
          <StockWorkspaceClient
            symbol={workspaceSymbol}
            embedded
            activeTab={tab}
            onTabChange={setTab}
            railFrom="xl"
            className="min-h-0 min-w-0 flex-1"
          />
        ) : (
          <div className="flex flex-1 items-center justify-center text-sm text-[var(--text-muted)]">
            종목을 선택하면 상세가 표시됩니다.
          </div>
        )}
      </div>
    </main>
  );
}
