"use client";

import {
  EMPTY_RECORD,
  type WlListProps,
} from "@/components/watchlist/lab/wl-shared";
import { WlSignalRow } from "@/components/watchlist/lab/wl-signal-row";

/**
 * 시안 A — **신호 목록**.
 *
 * 지금과 같은 세로 목록이되 칸 구성이 바뀐다(`wl-signal-row` 머리말). 헤더 줄은 없다:
 * 값마다 라벨이 붙어 다니거나(담은 뒤·목표) 라벨이 필요 없어서(이름·현재가·스파크라인)
 * 맨 위 한 줄이 가리킬 칸이 남지 않는다. 대신 **행 높이가 종목마다 다르다** — 말할 거리가
 * 있는 종목만 둘째 줄이 붙는다.
 */
export function WlList({ rows, records, signals, onOpen, onRemove }: WlListProps) {
  return (
    <div className="@container space-y-1.5">
      {rows.map((row) => (
        <WlSignalRow
          key={row.symbol}
          row={row}
          record={records.get(row.symbol) ?? EMPTY_RECORD}
          signals={signals.get(row.symbol) ?? []}
          onOpen={() => onOpen(row.symbol)}
          onRemove={() => onRemove(row.symbol)}
        />
      ))}
    </div>
  );
}
