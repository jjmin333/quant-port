"use client";

import {
  RANKING_DURATION_OPTIONS,
  RANKING_TYPE_OPTIONS,
  changeRateHint,
  rankingRefreshHint,
} from "@/lib/market-radar-config";
import type { MarketRadarParams } from "@/lib/market-radar-types";
import { useMarketDataSettings } from "@/hooks/use-market-data-settings";
import { cn } from "@/lib/utils";

type MarketRadarToolbarProps = {
  params: MarketRadarParams;
  rankedAt: string | null;
  refreshedAt?: string | null;
  onChange: (next: MarketRadarParams) => void;
  compact?: boolean;
};

export function MarketRadarToolbar({
  params,
  rankedAt,
  refreshedAt,
  onChange,
  compact,
}: MarketRadarToolbarProps) {
  const { settings } = useMarketDataSettings();

  function patch(partial: Partial<MarketRadarParams>) {
    onChange({ ...params, ...partial });
  }

  const displayAt = refreshedAt ?? rankedAt;
  const rankedLabel =
    displayAt &&
    new Date(displayAt).toLocaleTimeString("ko-KR", {
      timeZone: "Asia/Seoul",
      hour: "2-digit",
      minute: "2-digit",
      second: "2-digit",
    });

  return (
    <div
      className={cn(
        "min-w-0 shrink-0 space-y-2 border-b border-[var(--border)] bg-[var(--surface)]",
        compact ? "px-3 py-2" : "px-3 py-2.5",
      )}
    >
      <div className="flex min-w-0 flex-wrap items-center gap-1.5">
        <Segmented
          value={params.market}
          options={[
            { id: "KR", label: "국내" },
            { id: "US", label: "해외" },
          ]}
          compact={compact}
          onChange={(market) => patch({ market: market as MarketRadarParams["market"] })}
        />
        {/* On touch the two selects share the leftover row width instead of
            shrink-wrapping their labels and leaving ~100px of dead space. */}
        <select
          value={params.type}
          onChange={(e) => patch({ type: e.target.value as MarketRadarParams["type"] })}
          aria-label="정렬 기준"
          className={cn(
            "min-w-0 max-w-full rounded-md border border-[var(--border)] bg-[var(--background)] px-2 text-[var(--text-primary)]",
            compact ? "h-9 flex-1 text-[12px]" : "h-8 text-[11px]",
          )}
        >
          {RANKING_TYPE_OPTIONS.map((o) => (
            <option key={o.id} value={o.id}>
              {o.label}
            </option>
          ))}
        </select>
        <select
          value={params.duration}
          onChange={(e) => patch({ duration: e.target.value as MarketRadarParams["duration"] })}
          aria-label="기간"
          className={cn(
            "min-w-0 max-w-full rounded-md border border-[var(--border)] bg-[var(--background)] px-2 text-[var(--text-primary)]",
            compact ? "h-9 flex-1 text-[12px]" : "h-8 text-[11px]",
          )}
        >
          {RANKING_DURATION_OPTIONS.map((o) => (
            <option key={o.id} value={o.id}>
              {o.label}
            </option>
          ))}
        </select>
      </div>
      {/* Stacked on touch: side by side these two truncate each other to
          unreadable fragments at 375px. */}
      <div
        className={cn(
          "flex min-w-0 gap-x-2",
          compact ? "flex-col gap-y-0.5" : "items-center justify-between",
        )}
      >
        <p
          className={cn(
            "min-w-0 text-[10px] text-[var(--text-muted)]",
            compact ? "leading-snug" : "truncate",
          )}
        >
          {changeRateHint(params.duration)}
        </p>
        {rankedLabel && (
          <span
            className={cn(
              "text-[10px] tabular-nums text-[var(--text-muted)]",
              compact ? "" : "shrink-0",
            )}
          >
            {rankedLabel} 갱신 · {rankingRefreshHint(params.duration, settings.rankingsPollRealtimeMinutes)}
          </span>
        )}
      </div>
    </div>
  );
}

function Segmented<T extends string>({
  value,
  options,
  compact,
  onChange,
}: {
  value: T;
  options: { id: T; label: string }[];
  compact?: boolean;
  onChange: (id: T) => void;
}) {
  return (
    <div className="flex shrink-0 rounded-lg border border-[var(--border)] p-0.5">
      {options.map((o) => (
        <button
          key={o.id}
          type="button"
          onClick={() => onChange(o.id)}
          aria-pressed={value === o.id}
          className={cn(
            "rounded-md font-medium transition-colors",
            compact ? "px-3 py-1.5 text-[12px]" : "px-2.5 py-1 text-[11px]",
            value === o.id
              ? "bg-[var(--brand-muted)] text-[var(--brand)]"
              : "text-[var(--text-muted)] hover:text-[var(--text-secondary)]",
          )}
        >
          {o.label}
        </button>
      ))}
    </div>
  );
}
