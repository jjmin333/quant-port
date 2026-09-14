"use client";

import { useState } from "react";
import { ChevronDown } from "lucide-react";
import { EMPTY_RECORD, type WlListProps } from "@/components/watchlist/lab/wl-shared";
import { WlSignalRow } from "@/components/watchlist/lab/wl-signal-row";
import {
  BUCKET_HINT,
  BUCKET_LABEL,
  type WatchBucket,
} from "@/lib/watchlist-signals";
import type { WatchlistRow } from "@/lib/watchlist-calc";
import { cn } from "@/lib/utils";

/**
 * 시안 C — **오늘의 브리핑**.
 *
 * 행 모양은 시안 A와 **똑같다**. 다른 것은 목록을 세 칸으로 나눠 세운다는 점뿐이다 —
 * 「지금 볼 것 / 움직인 것 / 조용한 것」. 판정은 `lib/watchlist-signals`가 한다.
 *
 * 정렬로 같은 일을 할 수 있지 않나 — 할 수 없다. 정렬은 순서만 주고 **경계**를 주지
 * 않는다. 위에서 몇 번째 줄까지가 "오늘 볼 것"인지는 여전히 사용자가 판단해야 하고,
 * 종목이 스무 개면 그 판단을 매번 다시 한다. 칸은 그 경계를 화면이 대신 긋는다.
 *
 * 「조용한 것」은 **접어 둔다.** 목록의 절반 이상이 보통 여기 들어가고, 그 종목들이
 * 화면에서 하는 일은 스크롤을 늘리는 것뿐이다. 지운 게 아니라 접은 것이라 한 번 누르면
 * 지금과 같은 전체 목록이 된다.
 */
export function WlBrief({
  records,
  signals,
  onOpen,
  onRemove,
  buckets,
}: WlListProps & { buckets: Map<WatchBucket, WatchlistRow[]> }) {
  const [quietOpen, setQuietOpen] = useState(false);

  const order: WatchBucket[] = ["action", "watch", "quiet"];

  return (
    <div className="@container space-y-3">
      {order.map((bucket) => {
        const rows = buckets.get(bucket) ?? [];
        if (rows.length === 0) return null;
        const collapsible = bucket === "quiet";
        const open = !collapsible || quietOpen;

        return (
          <section key={bucket}>
            <button
              type="button"
              onClick={collapsible ? () => setQuietOpen((v) => !v) : undefined}
              aria-expanded={collapsible ? open : undefined}
              className={cn(
                "mb-1.5 flex w-full items-baseline gap-2 text-left",
                collapsible && "rounded-md hover:opacity-80",
              )}
            >
              <span
                className={cn(
                  "flex items-center gap-1.5 text-section font-bold",
                  bucket === "action" ? "text-[var(--brand)]" : "text-[var(--text-primary)]",
                )}
              >
                {bucket === "action" && (
                  <span className="size-1.5 rounded-full bg-[var(--brand)]" aria-hidden />
                )}
                {BUCKET_LABEL[bucket]}
                <span className="font-num text-note tabular-nums text-[var(--text-muted)]">
                  {rows.length}
                </span>
              </span>
              <span className="min-w-0 flex-1 truncate text-cell text-[var(--text-muted)]">
                {BUCKET_HINT[bucket]}
              </span>
              {collapsible && (
                <ChevronDown
                  className={cn(
                    "size-4 shrink-0 text-[var(--text-muted)] transition-transform",
                    open && "rotate-180",
                  )}
                />
              )}
            </button>

            {open && (
              <div className="space-y-1.5">
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
            )}
          </section>
        );
      })}
    </div>
  );
}
