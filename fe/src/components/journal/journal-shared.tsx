"use client";

import { StockAvatar } from "@/components/stocks/stock-avatar";
import { formatPrice } from "@/lib/format";
import {
  JOURNAL_TAG_TONE,
  noteReturnPct,
  type JournalNote,
} from "@/lib/journal-types";
import type { QuoteSummary } from "@/lib/quotes-types";
import { cn } from "@/lib/utils";

/**
 * 메모장 화면의 공용 조각.
 *
 * 목록 행·상세 모달·레일이 같은 가격 줄과 같은 태그 칩을 그린다. 세 곳에 따로
 * 두면 "작성 당시 → 현재"의 화살표 방향이나 색이 화면마다 어긋난다.
 */

/** 상승/하락 색. 이 앱은 빨강=상승, 파랑=하락이다. */
function toneClass(pct: number | null): string {
  if (pct === null) return "text-[var(--text-muted)]";
  if (pct > 0) return "text-[var(--profit)]";
  if (pct < 0) return "text-[var(--loss)]";
  return "text-[var(--text-secondary)]";
}

export function JournalTagChips({
  tags,
  onSelect,
  active,
  className,
}: {
  tags?: string[];
  onSelect?: (tag: string) => void;
  active?: string | null;
  className?: string;
}) {
  if (!tags || tags.length === 0) return null;
  return (
    <span className={cn("inline-flex flex-wrap items-center gap-1", className)}>
      {tags.map((tag) => {
        const tone = JOURNAL_TAG_TONE[tag] ?? "bg-[var(--surface-elevated)] text-[var(--text-muted)]";
        const classes = cn(
          "rounded-full px-2 py-0.5 text-[10px] font-medium",
          tone,
          active === tag && "ring-1 ring-[var(--brand)]",
        );
        if (!onSelect) {
          return (
            <span key={tag} className={classes}>
              {tag}
            </span>
          );
        }
        return (
          <button
            key={tag}
            type="button"
            onClick={(e) => {
              // 행 전체가 모달을 여는 버튼이라, 태그로 거르려는 클릭이 모달까지 열면
              // 두 가지 일이 한 번에 일어난다.
              e.stopPropagation();
              onSelect(tag);
            }}
            className={cn(classes, "transition-opacity hover:opacity-80")}
          >
            {tag}
          </button>
        );
      })}
    </span>
  );
}

/**
 * 「작성 당시 184,500 → 현재 212,000 (+14.9%)」 한 줄.
 *
 * 현재가가 없으면 작성 당시 값만 말한다. 없는 값을 0%로 그리면 "그대로다"라는
 * 하지 않은 말을 하게 된다.
 */
export function JournalPriceStrip({
  note,
  quote,
  className,
  compact,
}: {
  note: JournalNote;
  quote?: QuoteSummary;
  className?: string;
  compact?: boolean;
}) {
  const base = note.priceAtWrite;
  if (!base || base <= 0) return null;

  const currency = note.priceCurrency ?? "KRW";
  const current = quote?.currentPrice;
  const pct = noteReturnPct(note, current);

  return (
    <div
      className={cn(
        "flex flex-wrap items-center gap-x-2 gap-y-1 rounded-lg border border-[var(--border)] bg-[var(--surface-elevated)] px-2.5 py-1.5",
        className,
      )}
    >
      <span className="text-[10px] text-[var(--text-muted)]">작성 당시</span>
      <span className="font-mono text-xs font-medium tabular-nums text-[var(--text-primary)]">
        {formatPrice(base, currency)}
      </span>
      {current && current > 0 ? (
        <>
          <span className="text-[10px] text-[var(--text-muted)]">→</span>
          {!compact && <span className="text-[10px] text-[var(--text-muted)]">현재</span>}
          <span
            className={cn(
              "font-mono text-xs font-medium tabular-nums",
              toneClass(pct),
            )}
          >
            {formatPrice(current, currency)}
          </span>
          {pct !== null && (
            <span
              className={cn(
                "rounded-md px-1.5 py-0.5 font-mono text-[10px] font-semibold tabular-nums",
                pct > 0
                  ? "bg-[var(--profit)]/12 text-[var(--profit)]"
                  : pct < 0
                    ? "bg-[var(--loss)]/12 text-[var(--loss)]"
                    : "bg-[var(--surface)] text-[var(--text-secondary)]",
              )}
            >
              {pct > 0 ? "+" : ""}
              {pct.toFixed(1)}%
            </span>
          )}
        </>
      ) : (
        <span className="text-[10px] text-[var(--text-muted)]">현재가 확인 중</span>
      )}
      {note.priceDate && !compact && (
        <span className="ml-auto text-[10px] text-[var(--text-muted)]">{note.priceDate} 기준</span>
      )}
    </div>
  );
}

/** 목록 행·모달 헤더가 함께 쓰는 종목 칩. */
export function JournalSymbolChip({ note }: { note: JournalNote }) {
  if (!note.symbol) return null;
  return (
    <span className="inline-flex shrink-0 items-center gap-1 rounded-full bg-[var(--surface-elevated)] px-2 py-0.5 text-[11px] font-medium text-[var(--text-secondary)]">
      <StockAvatar symbol={note.symbol} name={note.symbolName} size="xs" />
      {note.symbolName || note.symbol}
    </span>
  );
}
