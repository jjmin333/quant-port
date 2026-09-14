"use client";

import { EMPTY_RECORD, type WlListProps } from "@/components/watchlist/lab/wl-shared";
import { WlCard } from "@/components/watchlist/lab/wl-card";

/**
 * 시안 B — **카드 보드**.
 *
 * 한 종목이 카드 한 장이다. 열이 바뀌는 기준은 화면 폭이 아니라 **본문 컬럼 폭**(`@`)이라,
 * 우측 레일이 붙은 1280px 화면에서도 두 장이 정확히 들어간다(레일 없는 폭이면 세 장).
 *
 * 표와의 맞바꿈이 분명한 시안이다 — 값이 세로선 위에 서지 않으므로 종목끼리 숫자를
 * 견주기는 목록보다 어렵고, 대신 한 종목을 이해하는 데 필요한 것이 카드 한 장 안에 다 있다.
 */
export function WlBoard({ rows, records, signals, onOpen, onRemove }: WlListProps) {
  return (
    <div className="@container">
      <div className="grid gap-2.5 @xl:grid-cols-2 @4xl:grid-cols-3">
        {rows.map((row) => (
          <WlCard
            key={row.symbol}
            row={row}
            record={records.get(row.symbol) ?? EMPTY_RECORD}
            signals={signals.get(row.symbol) ?? []}
            onOpen={() => onOpen(row.symbol)}
            onRemove={() => onRemove(row.symbol)}
          />
        ))}
      </div>
    </div>
  );
}
