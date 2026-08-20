"use client";

import { useMemo } from "react";
import { Pin, Search } from "lucide-react";
import { StockAvatar } from "@/components/stocks/stock-avatar";
import {
  SectionCard,
  SectionEmpty,
  SectionKeyValue,
} from "@/components/ui/section-card";
import { formatDateKST } from "@/lib/format";
import {
  JOURNAL_TAG_TONE,
  noteReturnPct,
  type JournalNote,
} from "@/lib/journal-types";
import type { QuoteSummary } from "@/lib/quotes-types";
import { cn } from "@/lib/utils";

/**
 * 메모장 화면의 우측 레일.
 *
 * 찾는 도구(검색·태그·종목)는 옆에 두고 본문은 메모 자체가 차지한다.
 *
 * 「다시 볼 메모」가 이 레일의 핵심이다 — 30일 넘게 손대지 않은 메모를 **그동안의
 * 등락과 함께** 위로 올린다. 메모장을 다시 열게 만드는 건 새 메모를 쓰고 싶은
 * 마음이 아니라 옛 메모가 맞았는지 궁금한 마음이다.
 */

const STALE_DAYS = 30;

/** 이 시각보다 오래된 메모를 「다시 볼」 것으로 본다. 렌더 중 시계를 직접 읽지 않도록 함수로 뺀다. */
function staleCutoff(): number {
  return Date.now() - STALE_DAYS * 24 * 60 * 60 * 1000;
}

export function JournalSideRail({
  query,
  onQueryChange,
  symbols,
  symbolFilter,
  onSymbolFilter,
  tags,
  tagFilter,
  onTagFilter,
  allNotes,
  quotes,
  onOpenNote,
}: {
  query: string;
  onQueryChange: (value: string) => void;
  symbols: { symbol: string; name: string }[];
  symbolFilter: string | null;
  onSymbolFilter: (symbol: string | null) => void;
  tags: { tag: string; count: number }[];
  tagFilter: string | null;
  onTagFilter: (tag: string | null) => void;
  allNotes: JournalNote[];
  quotes: Record<string, QuoteSummary>;
  onOpenNote: (id: number) => void;
}) {
  const pinned = useMemo(() => allNotes.filter((n) => n.pinned), [allNotes]);

  const thisMonth = useMemo(() => {
    const now = new Date();
    return allNotes.filter((n) => {
      const d = new Date(n.createdAt);
      return d.getFullYear() === now.getFullYear() && d.getMonth() === now.getMonth();
    }).length;
  }, [allNotes]);

  const linkedCount = useMemo(() => allNotes.filter((n) => n.symbol).length, [allNotes]);

  /** 30일 넘게 안 건드린 메모, 오래된 순. */
  const stale = useMemo(() => {
    const cutoff = staleCutoff();
    return allNotes
      .filter((n) => new Date(n.updatedAt).getTime() < cutoff)
      .sort((a, b) => new Date(a.updatedAt).getTime() - new Date(b.updatedAt).getTime())
      .slice(0, 5);
  }, [allNotes]);

  /**
   * 종목을 연결한 메모의 작성 후 성적. 「매수근거라고 적어둔 것들이 실제로 어땠나」가
   * 이 화면에서 얻을 수 있는 유일한 자기 피드백이다.
   */
  const perf = useMemo(() => {
    const pcts: number[] = [];
    let up = 0;
    for (const note of allNotes) {
      const pct = noteReturnPct(note, note.symbol ? quotes[note.symbol]?.currentPrice : undefined);
      if (pct === null) continue;
      pcts.push(pct);
      if (pct > 0) up += 1;
    }
    if (pcts.length === 0) return null;
    return {
      count: pcts.length,
      avg: pcts.reduce((a, b) => a + b, 0) / pcts.length,
      up,
      down: pcts.length - up,
    };
  }, [allNotes, quotes]);

  return (
    <>
      <SectionCard title="메모 검색">
        <div className="relative">
          <Search className="pointer-events-none absolute left-2.5 top-1/2 size-3.5 -translate-y-1/2 text-muted-foreground" />
          <input
            type="text"
            value={query}
            onChange={(e) => onQueryChange(e.target.value)}
            placeholder="제목·내용·태그"
            className="h-8 w-full rounded-lg border bg-background pl-8 pr-2.5 text-xs placeholder:text-muted-foreground focus:border-[var(--brand)] focus:outline-none"
          />
        </div>
      </SectionCard>

      <SectionCard title="태그">
        {tags.length === 0 ? (
          <SectionEmpty text="태그가 붙은 메모가 없습니다" />
        ) : (
          <div className="flex flex-wrap gap-1.5">
            {tags.map(({ tag, count }) => {
              const active = tagFilter === tag;
              return (
                <button
                  key={tag}
                  type="button"
                  onClick={() => onTagFilter(active ? null : tag)}
                  aria-pressed={active}
                  className={cn(
                    "inline-flex items-center gap-1 rounded-full px-2.5 py-1 text-[11px] font-medium transition-opacity hover:opacity-80",
                    JOURNAL_TAG_TONE[tag] ?? "bg-accent text-muted-foreground",
                    active && "ring-1 ring-[var(--brand)]",
                  )}
                >
                  {tag}
                  <span className="font-mono tabular-nums opacity-70">{count}</span>
                </button>
              );
            })}
          </div>
        )}
      </SectionCard>

      <SectionCard title="다시 볼 메모" action={<span className="text-[10px] text-muted-foreground">{STALE_DAYS}일 지남</span>}>
        {stale.length === 0 ? (
          <SectionEmpty text="최근에 모두 살펴봤습니다" />
        ) : (
          <ul className="min-w-0">
            {stale.map((note) => {
              const pct = noteReturnPct(
                note,
                note.symbol ? quotes[note.symbol]?.currentPrice : undefined,
              );
              return (
                <li key={note.id}>
                  <button
                    type="button"
                    onClick={() => onOpenNote(note.id)}
                    className="flex w-full min-w-0 items-center gap-2 rounded-md px-1 py-1.5 text-left hover:bg-accent"
                  >
                    <span className="min-w-0 flex-1">
                      <span className="line-clamp-1 block text-xs">{note.title || "제목 없음"}</span>
                      <span className="mt-0.5 block truncate text-[11px] text-muted-foreground">
                        {note.symbolName || note.symbol || "종목 미지정"} ·{" "}
                        {formatDateKST(note.updatedAt)}
                      </span>
                    </span>
                    <span
                      className={cn(
                        "shrink-0 font-mono text-[11px] font-semibold tabular-nums",
                        pct === null
                          ? "text-muted-foreground"
                          : pct > 0
                            ? "text-[var(--profit)]"
                            : pct < 0
                              ? "text-[var(--loss)]"
                              : "text-muted-foreground",
                      )}
                    >
                      {pct === null ? "—" : `${pct > 0 ? "+" : ""}${pct.toFixed(1)}%`}
                    </span>
                  </button>
                </li>
              );
            })}
          </ul>
        )}
      </SectionCard>

      <SectionCard
        title={
          <span className="flex items-center gap-1.5">
            <Pin className="size-3.5" />
            고정한 메모
          </span>
        }
      >
        {pinned.length === 0 ? (
          <SectionEmpty text="고정한 메모가 없습니다" />
        ) : (
          <ul className="min-w-0">
            {pinned.slice(0, 6).map((note) => (
              <li key={note.id}>
                <button
                  type="button"
                  onClick={() => onOpenNote(note.id)}
                  className="w-full min-w-0 rounded-md px-1 py-1.5 text-left hover:bg-accent"
                >
                  <span className="line-clamp-1 block text-xs">{note.title || "제목 없음"}</span>
                  <span className="mt-0.5 block truncate text-[11px] text-muted-foreground">
                    {note.symbolName || note.symbol || "종목 미지정"} ·{" "}
                    {formatDateKST(note.updatedAt)}
                  </span>
                </button>
              </li>
            ))}
          </ul>
        )}
      </SectionCard>

      <SectionCard title="종목별 메모">
        {symbols.length === 0 ? (
          <SectionEmpty text="종목에 연결된 메모가 없습니다" />
        ) : (
          <ul className="min-w-0">
            <li>
              <FilterRow
                label="전체 종목"
                active={symbolFilter === null}
                onClick={() => onSymbolFilter(null)}
              />
            </li>
            {symbols.map((s) => (
              <li key={s.symbol}>
                <FilterRow
                  label={s.name || s.symbol}
                  symbol={s.symbol}
                  active={symbolFilter === s.symbol}
                  onClick={() => onSymbolFilter(symbolFilter === s.symbol ? null : s.symbol)}
                />
              </li>
            ))}
          </ul>
        )}
      </SectionCard>

      <SectionCard title="메모 현황">
        <SectionKeyValue label="전체" value={`${allNotes.length}건`} />
        <SectionKeyValue label="이번 달 작성" value={`${thisMonth}건`} />
        <SectionKeyValue label="종목 연결" value={`${linkedCount}건`} />
        <SectionKeyValue label="고정" value={`${pinned.length}건`} />
        {/* 가격이 박제된 메모가 하나도 없으면 이 두 줄은 아예 그리지 않는다 —
            "—" 두 개는 정보가 아니라 빈 칸이다. */}
        {perf && (
          <>
            <SectionKeyValue
              label="작성 후 평균"
              value={`${perf.avg > 0 ? "+" : ""}${perf.avg.toFixed(1)}%`}
              valueClassName={
                perf.avg > 0
                  ? "text-[var(--profit)]"
                  : perf.avg < 0
                    ? "text-[var(--loss)]"
                    : undefined
              }
            />
            <SectionKeyValue label="플러스 / 마이너스" value={`${perf.up} / ${perf.down}`} />
          </>
        )}
      </SectionCard>
    </>
  );
}

function FilterRow({
  label,
  symbol,
  active,
  onClick,
}: {
  label: string;
  symbol?: string;
  active: boolean;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      aria-pressed={active}
      className={cn(
        "flex h-[30px] w-full min-w-0 items-center gap-1.5 rounded-md px-1 text-xs",
        active ? "bg-accent font-medium" : "hover:bg-accent/60",
      )}
    >
      {symbol && <StockAvatar symbol={symbol} name={label} size="xs" className="shrink-0" />}
      <span className="min-w-0 flex-1 truncate text-left">{label}</span>
    </button>
  );
}
