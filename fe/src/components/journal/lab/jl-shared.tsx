"use client";

import { createContext, useContext, useMemo, useState } from "react";
import { Pencil, Pin, Search, X } from "lucide-react";
import { JournalNoteForm } from "@/components/journal/journal-note-form";
import { JournalTagChips } from "@/components/journal/journal-shared";
import { StockAvatar } from "@/components/stocks/stock-avatar";
import { useQuickLook } from "@/components/providers/quick-look-provider";
import { Card } from "@/components/ui/card";
import { PriceDisplay } from "@/components/ui/price-display";
import { formatDateKST } from "@/lib/format";
import { JOURNAL_TAG_TONE, noteReturnPct, type JournalNote } from "@/lib/journal-types";
import type { QuoteSummary } from "@/lib/quotes-types";
import { cn } from "@/lib/utils";
import { useLabJournalData } from "./fixture";

/**
 * 세 시안이 함께 쓰는 조각 (2026-09-07 · 메모장 재설계).
 *
 * 배치는 시안마다 다르지만 **한 메모를 말하는 방식**은 같아야 한다 — 성과 막대의
 * 방향, 태그 칩, 종목 칩이 시안마다 다르면 사용자가 비교하는 것이 배치가 아니라
 * 디테일이 된다. 그래서 그 셋은 여기 한 벌만 둔다.
 *
 * 태그 칩·가격 줄·상세 모달·작성 폼은 **실서비스 컴포넌트를 그대로** 부른다.
 */

/* ── 공유 상태 ───────────────────────────────────────────────────────────── */

type LabStore = {
  all: JournalNote[];
  notes: JournalNote[];
  quotes: Record<string, QuoteSummary>;
  tags: { tag: string; count: number }[];
  symbols: { symbol: string; name: string; count: number }[];
  query: string;
  setQuery: (v: string) => void;
  tag: string | null;
  setTag: (v: string | null) => void;
  symbol: string | null;
  setSymbol: (v: string | null) => void;
  filtering: boolean;
  quoteFor: (note: JournalNote) => QuoteSummary | undefined;
  pctFor: (note: JournalNote) => number | null;
};

const LabContext = createContext<LabStore | null>(null);

export function useLab(): LabStore {
  const ctx = useContext(LabContext);
  if (!ctx) throw new Error("useLab must be used within JournalLabProvider");
  return ctx;
}

export function JournalLabProvider({ children }: { children: React.ReactNode }) {
  const { notes: all, quotes } = useLabJournalData();
  const [query, setQuery] = useState("");
  const [tag, setTag] = useState<string | null>(null);
  const [symbol, setSymbol] = useState<string | null>(null);

  const value = useMemo<LabStore>(() => {
    const quoteFor = (note: JournalNote) => (note.symbol ? quotes[note.symbol] : undefined);
    const pctFor = (note: JournalNote) => noteReturnPct(note, quoteFor(note)?.currentPrice);

    const tagCounts = new Map<string, number>();
    for (const n of all) for (const t of n.tags ?? []) tagCounts.set(t, (tagCounts.get(t) ?? 0) + 1);

    const symbolMap = new Map<string, { symbol: string; name: string; count: number }>();
    for (const n of all) {
      if (!n.symbol) continue;
      const prev = symbolMap.get(n.symbol);
      symbolMap.set(n.symbol, {
        symbol: n.symbol,
        name: n.symbolName || n.symbol,
        count: (prev?.count ?? 0) + 1,
      });
    }

    const q = query.trim().toLowerCase();
    const notes = all
      .filter((n) => {
        if (tag && !(n.tags ?? []).includes(tag)) return false;
        if (symbol && n.symbol !== symbol) return false;
        if (!q) return true;
        return (
          n.title.toLowerCase().includes(q) ||
          n.body.toLowerCase().includes(q) ||
          (n.tags ?? []).some((t) => t.toLowerCase().includes(q))
        );
      })
      .sort((a, b) => {
        if (a.pinned !== b.pinned) return a.pinned ? -1 : 1;
        return new Date(b.createdAt).getTime() - new Date(a.createdAt).getTime();
      });

    return {
      all,
      notes,
      quotes,
      tags: Array.from(tagCounts, ([t, count]) => ({ tag: t, count })).sort(
        (a, b) => b.count - a.count || a.tag.localeCompare(b.tag),
      ),
      symbols: Array.from(symbolMap.values()).sort((a, b) => b.count - a.count),
      query,
      setQuery,
      tag,
      setTag,
      symbol,
      setSymbol,
      filtering: Boolean(q || tag || symbol),
      quoteFor,
      pctFor,
    };
  }, [all, quotes, query, tag, symbol]);

  return <LabContext.Provider value={value}>{children}</LabContext.Provider>;
}

/** 종목이 붙은 메모의 성적. 세 시안이 같은 숫자를 말하도록 한 곳에서 센다. */
export function useLabStats() {
  const { all, pctFor } = useLab();
  return useMemo(() => {
    const scored = all
      .map((note) => ({ note, pct: pctFor(note) }))
      .filter((r): r is { note: JournalNote; pct: number } => r.pct !== null)
      .sort((a, b) => b.pct - a.pct);
    const avg = scored.length ? scored.reduce((a, r) => a + r.pct, 0) / scored.length : null;
    const up = scored.filter((r) => r.pct > 0).length;

    const tagMap = new Map<string, number[]>();
    for (const { note, pct } of scored) {
      for (const t of note.tags ?? []) {
        const arr = tagMap.get(t) ?? [];
        arr.push(pct);
        tagMap.set(t, arr);
      }
    }

    return {
      total: all.length,
      linked: all.filter((n) => n.symbol).length,
      pinned: all.filter((n) => n.pinned).length,
      scored: scored.length,
      avg,
      up,
      down: scored.length - up,
      ranked: scored,
      best: scored[0] ?? null,
      worst: scored[scored.length - 1] ?? null,
      byTag: Array.from(tagMap, ([tag, pcts]) => ({
        tag,
        count: pcts.length,
        avg: pcts.reduce((a, b) => a + b, 0) / pcts.length,
      })).sort((a, b) => b.avg - a.avg),
    };
  }, [all, pctFor]);
}

export function daysSince(iso: string): number {
  return Math.max(0, Math.floor((Date.now() - new Date(iso).getTime()) / 86_400_000));
}

/** 「n일 전」. 오늘·어제는 숫자보다 말이 빠르게 읽힌다. */
export function agoLabel(iso: string): string {
  const d = daysSince(iso);
  if (d === 0) return "오늘";
  if (d === 1) return "어제";
  return `${d}일 전`;
}

/* ── 성과 표시 ───────────────────────────────────────────────────────────── */

/** 상승/하락 색. 이 앱은 빨강=상승, 파랑=하락이다. */
export function toneClass(pct: number | null): string {
  if (pct === null) return "text-[var(--text-muted)]";
  if (pct > 0) return "text-[var(--profit)]";
  if (pct < 0) return "text-[var(--loss)]";
  return "text-[var(--text-secondary)]";
}

/** 「+26.2%」 알약 하나. 목록 어디에 놓여도 같은 크기·같은 색이다. */
export function OutcomePill({
  pct,
  className,
  size = "sm",
}: {
  pct: number | null;
  className?: string;
  size?: "sm" | "lg";
}) {
  if (pct === null) return null;
  return (
    <span
      className={cn(
        "shrink-0 rounded-md font-num font-semibold tabular-nums",
        size === "lg" ? "px-2 py-1 text-sm" : "px-1.5 py-0.5 text-micro",
        pct > 0
          ? "bg-[var(--profit)]/12 text-[var(--profit)]"
          : pct < 0
            ? "bg-[var(--loss)]/12 text-[var(--loss)]"
            : "bg-[var(--surface-elevated)] text-[var(--text-secondary)]",
        className,
      )}
    >
      {pct > 0 ? "+" : ""}
      {pct.toFixed(1)}%
    </span>
  );
}

/**
 * 「작성 당시 → 지금」 한 줄.
 *
 * 지금 화면은 이 값을 행 끝의 작은 % 하나로만 말한다. 그런데 이 앱의 메모가 다른
 * 메모장과 다른 지점이 정확히 여기다 — **글이 스스로 답을 갖고 온다.** 그래서
 * 가운데를 0으로 놓고 좌우로 자라는 막대를 준다: 메모를 세로로 쌓았을 때 어느 판단이
 * 크게 맞았고 어느 것이 크게 틀렸는지가 글을 읽기 전에 훑힌다.
 *
 * 막대는 ±30%에서 포화한다. 비례로만 그리면 +150% 하나가 나머지를 전부 점 하나로
 * 눌러 버린다.
 */
const TRACK_CAP = 30;

export function OutcomeTrack({
  note,
  className,
  showPrices = true,
}: {
  note: JournalNote;
  className?: string;
  showPrices?: boolean;
}) {
  const { quoteFor, pctFor } = useLab();
  const pct = pctFor(note);
  const base = note.priceAtWrite;
  if (pct === null || !base) return null;

  const currency = note.priceCurrency ?? "KRW";
  const width = `${(Math.min(Math.abs(pct) / TRACK_CAP, 1) * 50).toFixed(1)}%`;

  return (
    <div className={cn("flex min-w-0 items-center gap-2", className)}>
      {/*
        두 가격 칸은 폭을 고정한다. 글자 수에 맡기면 행마다 막대의 0점이 좌우로
        밀려서, 여러 메모를 세로로 쌓았을 때 길이를 견줄 수 없다 — 이 막대를 넣은
        이유가 정확히 그 비교다(실측: 원화 「1,787,000원」과 달러 「$172.27」이 같은
        목록에 있으면 0점이 20px 넘게 어긋난다).
      */}
      {showPrices && (
        <span className="w-[4.75rem] shrink-0 text-right font-num text-micro tabular-nums text-[var(--text-muted)]">
          <PriceDisplay value={base} currency={currency} size="inherit" />
        </span>
      )}
      <div className="relative h-1.5 min-w-0 flex-1 rounded-full bg-[var(--surface-elevated)]">
        {/* 0 기준선. 막대가 짧아도 「가운데가 기준」이라는 사실은 남아 있어야 한다. */}
        <span className="absolute left-1/2 top-1/2 h-3 w-px -translate-x-1/2 -translate-y-1/2 bg-[var(--border)]" />
        <span
          className={cn(
            "absolute top-0 h-full rounded-full",
            pct > 0 ? "left-1/2 bg-[var(--profit)]" : "right-1/2 bg-[var(--loss)]",
          )}
          style={{ width }}
        />
      </div>
      {showPrices && (
        <span
          className={cn(
            "w-[4.75rem] shrink-0 text-left font-num text-micro font-medium tabular-nums",
            toneClass(pct),
          )}
        >
          <PriceDisplay
            value={quoteFor(note)?.currentPrice ?? 0}
            currency={currency}
            size="inherit"
          />
        </span>
      )}
      <OutcomePill pct={pct} />
    </div>
  );
}

/* ── 종목 칩 ─────────────────────────────────────────────────────────────── */

/**
 * 눌러서 **퀵룩이 열리는** 종목 칩.
 *
 * 지금 화면의 종목 칩은 글자다 — 메모에 종목을 붙여 둔 보람이 「이름이 적혀 있다」로
 * 끝난다. 앱은 이미 종목 퀵룩 모달을 갖고 있으므로, 메모에서도 같은 한 번의 클릭으로
 * 시세·차트·점수·뉴스에 닿게 한다.
 */
export function LabSymbolChip({ note, size = "xs" }: { note: JournalNote; size?: "xs" | "sm" }) {
  const quickLook = useQuickLook();
  const { quoteFor } = useLab();
  if (!note.symbol) return null;
  const symbol = note.symbol;
  return (
    <button
      type="button"
      onClick={(e) => {
        e.stopPropagation();
        quickLook?.openSymbol(symbol, quoteFor(note) ?? null);
      }}
      className={cn(
        "inline-flex shrink-0 items-center gap-1 rounded-full bg-[var(--surface-elevated)] px-2 py-0.5 font-medium text-[var(--text-secondary)] transition-colors hover:bg-[var(--brand-muted)] hover:text-[var(--brand)]",
        size === "sm" ? "text-cell" : "text-micro",
      )}
    >
      <StockAvatar symbol={symbol} name={note.symbolName} size="xs" />
      {note.symbolName || symbol}
    </button>
  );
}

/* ── 작성 바 ─────────────────────────────────────────────────────────────── */

/**
 * 쉬고 있을 때는 한 줄, 누르면 실서비스 폼이 그대로 펼쳐지는 작성 자리.
 *
 * 지금 화면에서 「새 메모 작성」은 목록 카드 **안쪽**의 점선 버튼이고, 좁은 화면에서는
 * 그 버튼이 목록 헤더 아래에 묻힌다 — 메모장에서 가장 자주 하는 일이 가장 찾기 어려운
 * 자리에 있다. 세 시안 모두 이 바를 본문 맨 위에 세운다.
 *
 * 저장은 하지 않는다. 시안이 진짜 메모를 만들면 곤란하다.
 */
export function LabComposer({ className }: { className?: string }) {
  const [open, setOpen] = useState(false);

  if (!open) {
    return (
      <button
        type="button"
        onClick={() => setOpen(true)}
        className={cn(
          "flex w-full items-center gap-2 rounded-xl border border-[var(--border)] bg-[var(--surface)] px-3 py-2.5 text-left transition-colors hover:border-[var(--brand)]",
          className,
        )}
      >
        <Pencil className="size-4 shrink-0 text-[var(--brand)]" />
        <span className="min-w-0 flex-1 truncate text-sm text-[var(--text-muted)]">
          오늘 어떤 판단을 했나요 — 근거 · 지켜볼 것 · 손절 기준
        </span>
        <span className="shrink-0 rounded-lg bg-[var(--brand)] px-2.5 py-1 text-cell font-medium text-[var(--brand-foreground)]">
          기록
        </span>
      </button>
    );
  }

  return (
    <Card className={cn("min-w-0", className)}>
      <div className="mb-2 flex items-center justify-between">
        <span className="text-note font-semibold text-[var(--text-primary)]">새 메모</span>
        <button
          type="button"
          onClick={() => setOpen(false)}
          aria-label="접기"
          className="rounded-md p-1 text-[var(--text-muted)] hover:text-[var(--text-secondary)]"
        >
          <X className="size-3.5" />
        </button>
      </div>
      <JournalNoteForm
        compact
        autoFocus
        onCancel={() => setOpen(false)}
        onSubmit={async () => {
          // 시안이다 — 저장하지 않는다.
          setOpen(false);
        }}
      />
    </Card>
  );
}

/* ── 검색 · 태그 ─────────────────────────────────────────────────────────── */

export function LabSearchField({ className }: { className?: string }) {
  const { query, setQuery } = useLab();
  return (
    <div className={cn("relative min-w-0", className)}>
      <Search className="pointer-events-none absolute left-2.5 top-1/2 size-3.5 -translate-y-1/2 text-[var(--text-muted)]" />
      <input
        type="text"
        value={query}
        onChange={(e) => setQuery(e.target.value)}
        placeholder="제목 · 내용 · 태그"
        className="h-8 w-full rounded-lg border border-[var(--border)] bg-[var(--surface)] pl-8 pr-2.5 text-cell text-[var(--text-primary)] placeholder:text-[var(--text-muted)] focus:border-[var(--brand)] focus:outline-none"
      />
    </div>
  );
}

export function LabTagFilter({ className }: { className?: string }) {
  const { tags, tag: active, setTag } = useLab();
  return (
    <div className={cn("flex flex-wrap gap-1.5", className)}>
      {tags.map(({ tag, count }) => (
        <button
          key={tag}
          type="button"
          onClick={() => setTag(active === tag ? null : tag)}
          aria-pressed={active === tag}
          className={cn(
            "inline-flex items-center gap-1 rounded-full px-2.5 py-1 text-cell font-medium transition-opacity hover:opacity-80",
            JOURNAL_TAG_TONE[tag] ?? "bg-[var(--surface-elevated)] text-[var(--text-secondary)]",
            active === tag && "ring-1 ring-[var(--brand)]",
          )}
        >
          {tag}
          <span className="font-num tabular-nums opacity-70">{count}</span>
        </button>
      ))}
    </div>
  );
}

/** 필터가 걸려 있을 때만 뜨는 해제 줄. 어느 시안에서나 같은 자리(본문 첫 줄)다. */
export function LabFilterNote() {
  const { filtering, query, tag, symbol, symbols, setQuery, setTag, setSymbol, notes } = useLab();
  if (!filtering) return null;
  const label =
    tag ?? (symbol ? symbols.find((s) => s.symbol === symbol)?.name || symbol : `"${query}"`);
  return (
    <div className="flex items-center gap-2 rounded-lg border border-[var(--brand)]/40 bg-[var(--brand-muted)] px-3 py-1.5 text-cell text-[var(--brand)]">
      <span className="min-w-0 truncate">
        「{label}」로 걸러 {notes.length}건
      </span>
      <button
        type="button"
        onClick={() => {
          setQuery("");
          setTag(null);
          setSymbol(null);
        }}
        className="ml-auto shrink-0 rounded-md p-0.5 hover:opacity-70"
        aria-label="필터 해제"
      >
        <X className="size-3.5" />
      </button>
    </div>
  );
}

/* ── 메모 카드 ───────────────────────────────────────────────────────────── */

/**
 * 피드 한 장. 지금 목록 행과 담는 정보는 같지만 **순서가 다르다** —
 * 제목 · 종목 · 본문 · 성과 · 태그. 지금은 성과가 제목 오른쪽 끝의 작은 숫자라
 * 본문을 다 읽고 나면 이미 시야 밖이다.
 */
export function LabNoteCard({
  note,
  onOpen,
  compact,
}: {
  note: JournalNote;
  onOpen: () => void;
  compact?: boolean;
}) {
  return (
    <Card
      role="button"
      tabIndex={0}
      onClick={onOpen}
      onKeyDown={(e) => {
        if (e.key === "Enter" || e.key === " ") {
          e.preventDefault();
          onOpen();
        }
      }}
      className="min-w-0 cursor-pointer transition-colors hover:border-[var(--brand)]"
    >
      <div className="flex min-w-0 items-start gap-2">
        {note.pinned && (
          <Pin
            className="mt-1 size-3.5 shrink-0 fill-current text-[var(--brand)]"
            aria-label="고정됨"
          />
        )}
        <div className="min-w-0 flex-1">
          <div className="flex min-w-0 flex-wrap items-center gap-x-2 gap-y-1">
            <h3 className="min-w-0 text-sm font-semibold text-[var(--text-primary)]">
              {note.title || "제목 없음"}
            </h3>
            <LabSymbolChip note={note} />
            <span className="shrink-0 text-micro text-[var(--text-muted)]">
              {formatDateKST(note.createdAt)} · {agoLabel(note.createdAt)}
            </span>
          </div>
          <p
            className={cn(
              "mt-1 whitespace-pre-wrap text-read text-[var(--text-secondary)]",
              compact ? "line-clamp-2" : "line-clamp-3",
            )}
          >
            {note.body}
          </p>
        </div>
      </div>

      {note.priceAtWrite && <OutcomeTrack note={note} className="mt-2.5" />}

      {(note.tags?.length ?? 0) > 0 && (
        <div className="mt-2">
          <JournalTagChips tags={note.tags} />
        </div>
      )}
    </Card>
  );
}
