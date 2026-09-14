"use client";

import { Trash2 } from "lucide-react";
import { Card } from "@/components/ui/card";
import { WatchlistRangeBar } from "@/components/watchlist/watchlist-range-bar";
import {
  chipSignals,
  PriceBlock,
  ScoreBadge,
  SignalChipRow,
  SinceAdded,
  SymbolLabel,
  WlSpark,
  type SymbolRecord,
} from "@/components/watchlist/lab/wl-shared";
import type { WatchSignal } from "@/lib/watchlist-signals";
import type { WatchlistRow } from "@/lib/watchlist-calc";
import { cn } from "@/lib/utils";

/**
 * 시안 A·C가 쓰는 관심종목 한 줄.
 *
 * # 지금 행과 무엇이 다른가
 *
 * 지금 행은 칸이 아홉이고 그중 셋(전일·전주·전월)이 **같은 종류의 숫자**다. 열두 종목을
 * 훑으면 등락률 서른여섯 개를 읽고 그중 무엇이 중요한지는 스스로 판단해야 한다.
 *
 * 여기서는 그 셋을 지운다. 대신 —
 *
 *  - **최근 흐름**(40일 스파크라인)이 그 자리를 대신한다. 전주·전월 두 숫자가 말하려던
 *    것("요즘 오르는 중인가")을 선 하나가 더 정확히 말한다.
 *  - **오늘 등락률**은 현재가 아래에 그대로 남는다 — 전일 칸이 사라진 게 아니라 값이
 *    있어야 할 자리로 옮겨간 것이다.
 *  - **담은 뒤**는 티커 옆으로 올라온다. 지금은 `@3xl` 미만에서 통째로 접히는데, 이
 *    화면의 존재 이유를 좁은 화면에서 가장 먼저 지우고 있었다.
 *  - **신호 줄**이 둘째 줄에 붙는다. 말할 거리가 있는 종목만 두 줄이 되므로, 조용한
 *    종목은 지금보다 오히려 낮다.
 *
 * 칸이 접히는 기준은 지금과 같이 **컨테이너 폭**이다(`@` 접두사). 1280px 화면에서도
 * 좌측 내비와 우측 레일을 빼면 본문은 600px대라 뷰포트로 판정하면 어긋난다.
 */
export function WlSignalRow({
  row,
  record,
  signals,
  onOpen,
  onRemove,
  className,
}: {
  row: WatchlistRow;
  record: SymbolRecord;
  signals: WatchSignal[];
  onOpen: () => void;
  onRemove: () => void;
  className?: string;
}) {
  const q = row.quote;
  const metrics = q?.watchlistMetrics;

  return (
    <Card
      className={cn(
        /* 칸이 접히는 기준은 **행 자신의 폭**이다 — 목록 컨테이너와 거의 같지만,
           카드 보드처럼 좁은 칸에 같은 부품이 들어갈 때 판정이 어긋나지 않는다. */
        "@container px-3 py-2 transition-colors hover:border-[var(--brand)] hover:bg-[var(--surface-elevated)]/40",
        className,
      )}
    >
      <div className="flex items-center gap-2 @xl:gap-3">
        <button
          type="button"
          onClick={onOpen}
          aria-label={`${q?.name ?? row.symbol} 상세`}
          className="flex min-w-0 flex-1 items-center gap-2 rounded-lg text-left @xl:gap-3"
        >
          <SymbolLabel
            row={row}
            record={record}
            className="flex-[2.4]"
            sub={<SinceAdded row={row} className="shrink-0" />}
          />

          <ScoreBadge score={row.score} grade={row.scoreGrade} className="shrink-0" />

          <WlSpark
            symbol={row.symbol}
            changeRate={q?.changeRate ?? 0}
            className="hidden h-7 w-[5rem] shrink-0 @md:flex"
          />

          <WatchlistRangeBar
            size="sm"
            pct={row.rangePct}
            low={metrics?.week52Low}
            high={metrics?.week52High}
            currency={q?.currency ?? "KRW"}
            className="hidden min-w-[8.5rem] flex-[1.3] @xl:flex"
          />

          <PriceBlock quote={q} className="w-[6.75rem] shrink-0 @xl:w-[7.5rem]" />
        </button>

        <button
          type="button"
          onClick={onRemove}
          aria-label={`${q?.name ?? row.symbol} 관심종목에서 삭제`}
          className="shrink-0 rounded-lg p-1.5 text-[var(--text-muted)] hover:bg-[var(--surface-elevated)] hover:text-[var(--danger)]"
        >
          <Trash2 className="h-4 w-4" />
        </button>
      </div>

      <SignalChipRow signals={chipSignals(signals)} max={5} className="mt-1.5 pl-8" />
    </Card>
  );
}
