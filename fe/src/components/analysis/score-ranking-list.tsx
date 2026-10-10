"use client";

import { useCallback, useState } from "react";
import { useRouter } from "next/navigation";
import { useAuth } from "@/components/providers/auth-provider";
import { LoadErrorState } from "@/components/common/load-error-state";
import {
  RankingMemoModal,
  type RankingMemoTarget,
} from "@/components/market-radar/market-radar-memo-modal";
import { ScoreRankingRow } from "@/components/analysis/score-ranking-row";
import { ScoreRankingToolbar } from "@/components/analysis/score-ranking-toolbar";
import { useScoreRankings, type EnrichedScoreItem } from "@/hooks/use-score-rankings";
import { analysisSymbolHref } from "@/lib/analysis-tabs";
import { toggleWatchlistSymbol } from "@/lib/watchlist-store";
import type { ScoreRankingParams } from "@/lib/score-ranking-config";
import { cn } from "@/lib/utils";
import type { StockScoreListItem } from "@/lib/stock-score-api";

type ScoreRankingListProps = {
  params: ScoreRankingParams;
  /**
   * 툴바 **바로 아래**에 끼울 것. 좁은 화면의 「이 랭킹의 성적」 접힌 줄이 여기 선다.
   *
   * 목록 밖(부모)에서 그리면 국내/해외 탭보다 위로 올라간다 — 시장을 고르기도 전에
   * 그 시장의 성적이 먼저 보이는 순서가 된다.
   */
  afterToolbar?: React.ReactNode;
  /** 시장 탭의 「국내 ETF」를 눌렀을 때. 없으면 그 탭이 없다. */
  onEtf?: () => void;
  onParamsChange: (next: ScoreRankingParams) => void;
  /**
   * 서버가 미리 받아 둔 첫 목록. 기본 손잡이(국내·종합·실시간)로 들어왔을 때만
   * 온다 — 다른 손잡이면 서버가 받아 둔 것이 **틀린 목록**이다
   * (`app/(main)/analysis/page.tsx`).
   */
  seed?: { items: StockScoreListItem[]; tradeDate: string };
  activeSymbol?: string | null;
  onSelect: (symbol: string) => void;
  compact?: boolean;
  className?: string;
};

/** 목록에 그리는 종목 수. 실시간 차트가 100인 것과 맞춘다. */
const SCORE_RANKING_COUNT = 100;

/*
AI 추천 랭킹의 **왼쪽 목록** — 실시간 차트(마켓 레이더)와 같은 구조다.

# 무엇이 바뀌었나 (2026-09-03 사용자 지적)

두 화면을 같은 2단 배치로 맞춘 뒤에도 왼쪽 목록만 서로 달랐다. 이쪽은 카드 안에 든
표였다: 자체 머리줄(「종목 랭킹」)에 「간단/넓게」 밀도 토글, 정렬용 표 헤더, 그리고
`<Link>`로 만든 행. 그래서 같은 앱의 두 목록이 글꼴 크기도 버튼 모양도 달랐다.

지금은 **툴바 + 행 + 메모 창**을 실시간 차트와 같은 부품으로 만든다
(`radar-toolbar-parts` · `market-radar-row`의 아이콘 버튼 · `RankingMemoModal`).

# 정렬 토글이 사라진 자리

예전 표 헤더에는 종목명·등락·점수로 다시 정렬하는 버튼이 있었다. 지금은 그 일을
**축과 기간**이 한다 — 서버가 여섯 축 × 네 기간을 미리 줄 세워 두므로, 화면이 받은
100줄을 다시 뒤집는 것보다 정확하다(받은 100줄은 이미 그 축의 상위 100이다).

# 카드 밀도 토글도 사라졌다

「간단/넓게」는 이 목록이 본문 위에 쌓여 있던 시절의 손잡이다. 지금은 한 칸을
통째로 쓰므로 `compact`(좁은 화면)만 남는다 — 실시간 차트와 같은 규칙이다.
*/
export function ScoreRankingList({
  params,
  afterToolbar,
  onEtf,
  seed,
  onParamsChange,
  activeSymbol,
  onSelect,
  compact = false,
  className,
}: ScoreRankingListProps) {
  const { session } = useAuth();
  const router = useRouter();
  const { items, tradeDate, loading, error, retry } = useScoreRankings(
    params,
    SCORE_RANKING_COUNT,
    seed,
  );

  const [busySymbol, setBusySymbol] = useState<string | null>(null);
  const [memoItem, setMemoItem] = useState<RankingMemoTarget | null>(null);

  /*
    로그인이 필요한 동작은 **돌아올 곳을 들려 보낸다.** 지금 보고 있는 종목이 쿼리에
    실리므로 로그인 뒤 같은 자리로 돌아온다 (실시간 차트 목록과 같은 처리).
  */
  const requireLogin = useCallback(() => {
    const back = analysisSymbolHref(activeSymbol ?? "");
    router.push(`/login?next=${encodeURIComponent(back)}`);
  }, [router, activeSymbol]);

  const toggleWatchlist = useCallback(
    async (item: EnrichedScoreItem) => {
      if (!session) {
        requireLogin();
        return;
      }
      if (busySymbol === item.symbol) return;
      setBusySymbol(item.symbol);
      try {
        // 저장소가 낙관적 반영과 되돌리기를 함께 한다 — 목록은 구독으로 따라온다.
        await toggleWatchlistSymbol(item.symbol);
      } catch {
        // 되돌리기는 저장소가 끝냈다. 여기서 더 할 말은 없다.
      } finally {
        setBusySymbol(null);
      }
    },
    [session, busySymbol, requireLogin],
  );

  const openMemo = useCallback(
    (item: EnrichedScoreItem) => {
      if (!session) {
        requireLogin();
        return;
      }
      setMemoItem({
        symbol: item.symbol,
        name: item.name,
        rank: item.rank,
        currency: (item.currency ?? "KRW") as "KRW" | "USD",
        assetType: item.assetType as RankingMemoTarget["assetType"],
        price: item.closePrice ?? 0,
        originLabel: "AI 점수 순위",
      });
    },
    [session, requireLogin],
  );

  return (
    <div className={cn("flex min-h-0 min-w-0 flex-col bg-[var(--surface)]", className)}>
      <ScoreRankingToolbar
        params={params}
        tradeDate={tradeDate}
        onChange={onParamsChange}
        onEtf={onEtf}
        compact={compact}
      />

      {afterToolbar}

      <div
        className={cn(
          "min-h-0 flex-1 overflow-y-auto overscroll-contain",
          compact ? "pb-[env(safe-area-inset-bottom,0px)]" : "pb-1",
        )}
      >
        {loading && items.length === 0 ? (
          <p className="p-4 text-center text-cell text-[var(--text-muted)]">랭킹 로딩 중…</p>
        ) : error ? (
          <LoadErrorState compact message={error} retrying={loading} onRetry={retry} />
        ) : items.length === 0 ? (
          /*
            축·기간 조합은 배치가 미리 만들어 둔다. 아직 한 번도 안 돌았으면 그 조합만
            비어 있는 것이므로, 「점수가 없다」가 아니라 **아직 준비 중**이라고 말한다.
          */
          <p className="p-4 text-center text-cell leading-relaxed text-[var(--text-muted)]">
            {params.axis === "composite" && params.duration === "realtime"
              ? "아직 산출된 점수가 없습니다."
              : "이 조합의 랭킹을 아직 만들지 않았습니다 — 다음 점수 배치 뒤에 채워집니다."}
          </p>
        ) : (
          items.map((item) => (
            <ScoreRankingRow
              key={item.symbol}
              item={item}
              active={item.symbol === activeSymbol}
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
