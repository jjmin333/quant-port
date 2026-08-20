"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { ChevronDown, TrendingDown, TrendingUp } from "lucide-react";
import { Card } from "@/components/ui/card";
import { fetchStockDailyCloses } from "@/lib/daily-close-api";
import type { DailyCloseRow, DailyCloseSeries } from "@/lib/daily-close-types";
import { formatPercent, formatPrice } from "@/lib/format";
import { trendTextClass } from "@/lib/trend-colors";
import { cn } from "@/lib/utils";

type DailyCloseTabProps = {
  symbol: string;
  currency: "KRW" | "USD";
  compact?: boolean;
};

/** `'26년 8월 6일 (목)` — matches the dated-list styling used elsewhere. */
function formatTradeDate(tradeDate: string, weekday: string): { head: string; sub: string } {
  const [year, month, day] = tradeDate.split("-");
  if (!year || !month || !day) return { head: tradeDate, sub: weekday };
  return {
    head: `'${year.slice(2)}년 ${Number(month)}월`,
    sub: `${Number(day)}일${weekday ? ` (${weekday})` : ""}`,
  };
}

function scoreToneClass(score: number): string {
  if (score >= 70) return "text-[var(--profit)]";
  if (score >= 50) return "text-[var(--text-primary)]";
  if (score >= 30) return "text-[var(--text-secondary)]";
  return "text-[var(--loss)]";
}

function DailyCloseItem({
  row,
  currency,
  compact,
}: {
  row: DailyCloseRow;
  currency: "KRW" | "USD";
  compact: boolean;
}) {
  const [open, setOpen] = useState(false);
  const { head, sub } = formatTradeDate(row.tradeDate, row.weekday);
  const rate = row.changeRate;
  const hasSummary = row.aiSummary.trim() !== "";

  const body = (
    <>
      <div className="min-w-0 text-left">
        <p className={cn("text-[var(--text-muted)]", compact ? "text-[11px]" : "text-[11px]")}>{head}</p>
        <p
          className={cn(
            "font-medium text-[var(--text-primary)]",
            compact ? "text-[13px]" : "text-[13px]",
          )}
        >
          {sub}
        </p>
      </div>

      <div className="text-right">
        {rate == null ? (
          <p className="text-[13px] text-[var(--text-muted)]">—</p>
        ) : (
          <p
            className={cn(
              "flex items-center justify-end gap-1 font-medium tabular-nums",
              compact ? "text-[13px]" : "text-[13px]",
              trendTextClass(rate),
            )}
          >
            {rate >= 0 ? (
              <TrendingUp className="h-3.5 w-3.5 shrink-0" aria-hidden />
            ) : (
              <TrendingDown className="h-3.5 w-3.5 shrink-0" aria-hidden />
            )}
            {formatPercent(rate)}
          </p>
        )}
        <p className="text-[11px] tabular-nums text-[var(--text-muted)]">
          {formatPrice(row.close, currency)}
        </p>
      </div>

      <div className="w-12 text-right sm:w-16">
        {/* A missing score prints "-" — never 0, which would read as a real
            (and very bearish) reading. */}
        {row.score == null ? (
          <span className="text-[15px] font-semibold text-[var(--text-muted)]">-</span>
        ) : (
          <span className={cn("text-[15px] font-semibold tabular-nums", scoreToneClass(row.score))}>
            {row.score > 0 ? `+${row.score}` : row.score}
          </span>
        )}
      </div>
    </>
  );

  if (!hasSummary) {
    return (
      <div className="grid grid-cols-[minmax(0,1fr)_auto_auto] items-center gap-3 border-b border-[var(--border)]/70 px-3 py-2.5 last:border-b-0">
        {body}
      </div>
    );
  }

  return (
    <div className="border-b border-[var(--border)]/70 last:border-b-0">
      <button
        type="button"
        onClick={() => setOpen((v) => !v)}
        aria-expanded={open}
        className="grid w-full grid-cols-[minmax(0,1fr)_auto_auto] items-center gap-3 px-3 py-2.5 text-left transition-colors hover:bg-[var(--surface-elevated)]/50"
      >
        {body}
        <ChevronDown
          className={cn(
            "h-3.5 w-3.5 shrink-0 text-[var(--text-muted)] transition-transform",
            open && "rotate-180",
          )}
          aria-hidden
        />
      </button>
      {open && (
        <div className="px-3 pb-3">
          <p className="rounded-lg bg-[var(--surface-elevated)]/60 px-3 py-2 text-[12px] leading-relaxed text-[var(--text-secondary)]">
            {row.aiSummary}
          </p>
          {row.analysisSource === "rule" && (
            <p className="mt-1 text-[10px] text-[var(--text-muted)]">
              규칙 기반 요약 (LLM 미사용)
            </p>
          )}
        </div>
      )}
    </div>
  );
}

export function DailyCloseTab({ symbol, currency, compact = false }: DailyCloseTabProps) {
  const [series, setSeries] = useState<DailyCloseSeries | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      setSeries(await fetchStockDailyCloses(symbol, 60));
    } catch (e) {
      setSeries(null);
      setError(e instanceof Error ? e.message : "일별 시세를 불러오지 못했습니다.");
    } finally {
      setLoading(false);
    }
  }, [symbol]);

  useEffect(() => {
    void load();
  }, [load]);

  const rows = useMemo(() => series?.rows ?? [], [series]);
  const scored = useMemo(() => rows.filter((r) => r.score != null).length, [rows]);
  const seriesCurrency = series?.currency ?? currency;

  return (
    <Card className="p-0">
      <div className="flex flex-wrap items-baseline justify-between gap-2 border-b border-[var(--border)] px-3 py-2.5">
        <h2 className="text-sm font-semibold text-[var(--text-primary)]">일별 시세 · AI 점수</h2>
        <p className="text-[10px] text-[var(--text-muted)]">
          정규장 종가 기준 · 점수 {scored}/{rows.length}일
        </p>
      </div>

      <div className="grid grid-cols-[minmax(0,1fr)_auto_auto] gap-3 border-b border-[var(--border)] bg-[var(--surface-elevated)]/40 px-3 py-1.5 text-[10px] font-medium text-[var(--text-muted)]">
        <span>날짜</span>
        <span className="text-right">실제주가</span>
        <span className="w-12 text-right sm:w-16">점수</span>
      </div>

      {loading ? (
        <p className="px-3 py-8 text-center text-xs text-[var(--text-muted)]">일별 시세 로딩 중…</p>
      ) : error ? (
        <p className="px-3 py-8 text-center text-xs text-[var(--text-muted)]">{error}</p>
      ) : rows.length === 0 ? (
        <p className="px-3 py-8 text-center text-xs text-[var(--text-muted)]">
          아직 적재된 일별 종가가 없습니다. 정규장 종가 이후 자동으로 쌓입니다.
        </p>
      ) : (
        <div>
          {rows.map((row) => (
            <DailyCloseItem
              key={row.tradeDate}
              row={row}
              currency={seriesCurrency}
              compact={compact}
            />
          ))}
        </div>
      )}
    </Card>
  );
}
