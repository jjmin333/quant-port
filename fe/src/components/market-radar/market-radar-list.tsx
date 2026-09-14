"use client";

import { useCallback, useState } from "react";
import { useRouter } from "next/navigation";
import { useAuth } from "@/components/providers/auth-provider";
import { MarketRadarRow } from "@/components/market-radar/market-radar-row";
import {
  RankingMemoModal,
  memoTargetFromRankingItem,
  type RankingMemoTarget,
} from "@/components/market-radar/market-radar-memo-modal";
import { addWatchlistClient, removeWatchlistClient } from "@/lib/market-api";
import { marketRadarHref } from "@/lib/market-radar-config";
import type { EnrichedRankingItem, MarketRadarParams } from "@/lib/market-radar-types";
import { cn } from "@/lib/utils";

type MarketRadarListProps = {
  items: EnrichedRankingItem[];
  selectedSymbol: string;
  radarParams: MarketRadarParams;
  loading: boolean;
  compact?: boolean;
  onSelect: (symbol: string) => void;
  /** 관심 상태가 바뀌면 훅의 목록에도 반영한다(낙관적 갱신의 되돌리기 포함). */
  onWatchlistChange: (symbol: string, watchlisted: boolean) => void;
};

export function MarketRadarList({
  items,
  selectedSymbol,
  radarParams,
  loading,
  compact = false,
  onSelect,
  onWatchlistChange,
}: MarketRadarListProps) {
  const { session } = useAuth();
  const router = useRouter();
  const [busySymbol, setBusySymbol] = useState<string | null>(null);
  const [memoItem, setMemoItem] = useState<RankingMemoTarget | null>(null);

  /*
    로그인이 필요한 동작은 **로그인 화면으로 보내되 돌아올 곳을 들려 보낸다.**
    지금 보고 있는 시장·정렬·기간이 쿼리에 그대로 실리므로, 로그인 뒤에 같은
    목록으로 돌아온다 — 처음부터 다시 고르게 하면 방금 하려던 일을 잊는다.
  */
  const requireLogin = useCallback(() => {
    router.push(`/login?next=${encodeURIComponent(marketRadarHref(radarParams))}`);
  }, [router, radarParams]);

  /*
    관심 등록은 **낙관적으로** 반영한다. 5초 폴링이 도는 화면이라 서버 왕복을
    기다리면 그 사이 갱신이 눌린 상태를 덮어써서, 눌렀는데 안 눌린 것처럼 보인다.
    실패하면 되돌린다.
  */
  const toggleWatchlist = useCallback(
    async (item: EnrichedRankingItem) => {
      if (!session) {
        requireLogin();
        return;
      }
      const symbol = item.symbol;
      if (busySymbol === symbol) return;

      const next = !item.isWatchlisted;
      setBusySymbol(symbol);
      onWatchlistChange(symbol, next);
      try {
        if (next) await addWatchlistClient(symbol);
        else await removeWatchlistClient(symbol);
      } catch {
        onWatchlistChange(symbol, !next);
      } finally {
        setBusySymbol(null);
      }
    },
    [session, busySymbol, onWatchlistChange, requireLogin],
  );

  const openMemo = useCallback(
    (item: EnrichedRankingItem) => {
      if (!session) {
        requireLogin();
        return;
      }
      setMemoItem(memoTargetFromRankingItem(item));
    },
    [session, requireLogin],
  );

  return (
    <div className="flex min-h-0 min-w-0 flex-1 flex-col bg-[var(--surface)]">
      <div
        className={cn(
          "min-h-0 flex-1 overflow-y-auto overscroll-contain",
          // Touch viewports get a little runway past the last row so the final
          // item clears the home indicator; desktop needs no filler.
          compact ? "pb-[env(safe-area-inset-bottom,0px)]" : "pb-1",
        )}
      >
        {loading && items.length === 0 ? (
          <p className="p-4 text-center text-cell text-[var(--text-muted)]">랭킹 로딩 중…</p>
        ) : items.length === 0 ? (
          <p className="p-4 text-center text-cell text-[var(--text-muted)]">
            {radarParams.duration === "realtime"
              ? "표시할 종목이 없습니다."
              : "해당 기간 랭킹 데이터를 수집 중이거나 표시할 종목이 없습니다."}
          </p>
        ) : (
          items.map((item) => (
            <MarketRadarRow
              key={item.symbol}
              item={item}
              active={item.symbol === selectedSymbol}
              compact={compact}
              onSelect={onSelect}
              onToggleWatchlist={toggleWatchlist}
              onOpenMemo={openMemo}
              watchlistBusy={busySymbol === item.symbol}
            />
          ))
        )}
      </div>

      <RankingMemoModal item={memoItem} onClose={() => setMemoItem(null)} />
    </div>
  );
}
