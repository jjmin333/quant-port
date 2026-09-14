"use client";

import { memo } from "react";
import { Heart, NotepadText } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { PriceDisplay } from "@/components/ui/price-display";
import { ValueFlash } from "@/components/ui/value-flash";
import { RowIconButton } from "@/components/market-radar/market-radar-row";
import { StockAvatar } from "@/components/stocks/stock-avatar";
import type { EnrichedScoreItem } from "@/hooks/use-score-rankings";
import { formatPercent } from "@/lib/format";
import { SCORE_NEUTRAL_TEXT, priceAsOfLabel } from "@/lib/stock-score-api";
import { trendTextClass } from "@/lib/trend-colors";
import { cn } from "@/lib/utils";

type ScoreRankingRowProps = {
  item: EnrichedScoreItem;
  active: boolean;
  compact?: boolean;
  onSelect: (symbol: string) => void;
  onToggleWatchlist: (item: EnrichedScoreItem) => void;
  onOpenMemo: (item: EnrichedScoreItem) => void;
  watchlistBusy?: boolean;
};

/*
AI 추천 랭킹 한 줄 — **실시간 차트 행과 같은 모양**이다 (2026-09-03 사용자 지적).

순위 · 로고 · 종목명(+보유) · 가격/등락률 · 관심 · 메모. 여백·글자 크기·아이콘 버튼
규격까지 `market-radar-row`와 맞췄고, 아이콘 버튼은 아예 그 파일의 것을 그대로 쓴다.

# 다른 점 하나 — 점수 칸

이 목록은 **점수로 줄 세운 목록**이라 그 숫자가 행에 있어야 한다. 실시간 차트에서
거래대금이 있던 자리가 아니라 오른쪽 끝(아이콘 앞)에 세로로 세운다.

숫자는 `rankValue` — 지금 고른 축과 기간이 실제로 줄 세운 값이다. 종합 점수(`score`)를
그리면 「추세 순」 목록에서 숫자가 뒤죽박죽으로 보인다. 서버가 안 채워 준 옛 응답에는
`score`로 물러선다.

# 왜 button이 아니라 div인가

행 전체가 「종목 선택」이고 그 안에 관심·메모 **버튼이 들어간다.** 버튼 안의 버튼은
무효 HTML이라 브라우저가 마크업을 재구성하고, 그러면 안쪽 버튼이 클릭을 못 받는다
(실시간 차트 행이 같은 이유로 div다).
*/
function ScoreRankingRowImpl({
  item,
  active,
  compact,
  onSelect,
  onToggleWatchlist,
  onOpenMemo,
  watchlistBusy,
}: ScoreRankingRowProps) {
  const currency = (item.currency ?? "KRW") as "KRW" | "USD";
  const price = item.closePrice ?? 0;
  const rankValue = item.rankValue ?? item.score;

  /*
    껍데기는 버튼이 아니다 — 이유는 `market-radar-row.tsx`와 같다(버튼 안의 버튼).
    두 목록은 같은 `RowIconButton`을 쓰므로 구조도 같이 맞춘다.
  */
  return (
    <div
      className={cn(
        "flex w-full items-center border-b border-[var(--border)]/70 transition-colors",
        compact ? "gap-2.5 px-3 py-2" : "gap-2 px-2 py-1.5",
        active ? "bg-[var(--brand-muted)]" : "hover:bg-[var(--surface-elevated)]",
      )}
    >
      <button
        type="button"
        onClick={() => onSelect(item.symbol)}
        aria-current={active ? "true" : undefined}
        aria-label={`${item.name} ${item.rank}위`}
        className={cn(
          "flex min-w-0 flex-1 cursor-pointer items-center bg-transparent p-0 text-left",
          "focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-[var(--brand)]",
          compact ? "gap-2.5" : "gap-2",
        )}
      >
      <div
        className={cn(
          "flex shrink-0 items-baseline justify-end tabular-nums",
          compact ? "w-8" : "w-7",
        )}
      >
        <span
          className={cn(
            "font-num font-semibold text-[var(--text-primary)]",
            compact ? "text-note" : "text-cell",
          )}
        >
          {item.rank}
        </span>
      </div>

      <StockAvatar
        symbol={item.symbol}
        name={item.name}
        currency={currency}
        assetType={item.assetType as "domestic" | "us" | "etf" | undefined}
        size="xs"
        className={cn("shrink-0", compact ? "h-7 w-7 text-micro" : "h-6 w-6 text-micro")}
      />

      <div className="min-w-0 flex-1">
        <div className="flex min-w-0 items-center gap-1">
          <span
            className={cn(
              "truncate font-medium leading-tight text-[var(--text-primary)]",
              compact ? "text-note" : "text-cell",
            )}
          >
            {item.name}
          </span>
          {item.isHeld && (
            <Badge variant="info" className="shrink-0 px-1 py-0 text-micro leading-none">
              보유
            </Badge>
          )}
        </div>
        <div
          className={cn(
            "mt-0.5 flex min-w-0 items-baseline gap-1.5 font-num tabular-nums leading-tight",
            compact ? "text-cell" : "text-micro",
          )}
          title={priceAsOfLabel(item.priceAsOf)}
        >
          {/* 실시간 차트 행과 같은 플래시를 준다 — 같은 가격이 화면마다 다르게
              (한쪽은 말없이) 바뀌던 것을 없앤다(2026-09-04 사용자 요청 4). */}
          <ValueFlash
            value={price > 0 ? price : null}
            resetKey={item.symbol}
            className="truncate text-[var(--text-primary)]"
          >
            {price > 0 ? (
              <PriceDisplay value={price} currency={currency} size="inherit" />
            ) : (
              "—"
            )}
          </ValueFlash>
          <span
            className={cn(
              "shrink-0",
              typeof item.changeRate === "number" && item.changeRate !== 0
                ? trendTextClass(item.changeRate)
                : "text-[var(--text-muted)]",
            )}
          >
            {typeof item.changeRate === "number" ? formatPercent(item.changeRate) : "—"}
          </span>
        </div>
      </div>

      {/*
        점수는 신호등을 쓰지 않는다 (08-12 사용자 결정). 순위가 이미 높낮이를 말하고
        있어서, 백 줄 전부에 색을 입히면 초록 벽이 될 뿐이다.
      */}
      <div className="w-9 shrink-0 text-right leading-tight">
        <span className={cn("font-num text-note font-bold tabular-nums", SCORE_NEUTRAL_TEXT)}>
          {rankValue}
        </span>
        {item.grade && (
          <span className={cn("block text-micro font-medium", SCORE_NEUTRAL_TEXT)}>
            {item.grade}
          </span>
        )}
      </div>

      </button>

      {/* 주 동작의 **형제**다 — 버튼 안에 들어가면 다시 중첩이 된다. */}
      <div className="flex shrink-0 items-center gap-0.5">
        <RowIconButton
          compact={compact}
          active={item.isWatchlisted}
          busy={watchlistBusy}
          label={item.isWatchlisted ? `${item.name} 관심 해제` : `${item.name} 관심 등록`}
          onClick={() => onToggleWatchlist(item)}
        >
          <Heart
            className={cn("size-full", item.isWatchlisted && "fill-current")}
            aria-hidden="true"
          />
        </RowIconButton>
        <RowIconButton
          compact={compact}
          label={`${item.name} 메모 작성`}
          onClick={() => onOpenMemo(item)}
        >
          <NotepadText className="size-full" aria-hidden="true" />
        </RowIconButton>
      </div>
    </div>
  );
}

/*
`memo`가 여기서 값을 하는 이유는 실시간 차트와 다르다 — 이 목록은 폴링하지 않으므로
행이 저절로 바뀌지 않는다. 다만 관심 등록 한 번이 목록 전체를 새 배열로 만들기 때문에
(`patchWatchlistFlag`와 같은 방식) 그때 100행이 전부 다시 그려지는 것을 막는다.
*/
export const ScoreRankingRow = memo(ScoreRankingRowImpl);
