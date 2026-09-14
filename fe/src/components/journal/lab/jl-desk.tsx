"use client";

import { useMemo } from "react";
import Link from "next/link";
import { ArrowRight, FileText } from "lucide-react";
import { MarketRadarSparkline } from "@/components/market-radar/market-radar-sparkline";
import { JournalTagChips } from "@/components/journal/journal-shared";
import { StockAvatar } from "@/components/stocks/stock-avatar";
import { useQuickLook } from "@/components/providers/quick-look-provider";
import { Card } from "@/components/ui/card";
import { PriceDisplay } from "@/components/ui/price-display";
import { SectionCard, SectionEmpty, SectionKeyValue } from "@/components/ui/section-card";
import { buttonVariants } from "@/components/ui/button";
import { analysisSymbolHref } from "@/lib/analysis-tabs";
import { formatPercent } from "@/lib/format";
import type { JournalNote } from "@/lib/journal-types";
import type { QuoteSummary } from "@/lib/quotes-types";
import { cn } from "@/lib/utils";
import {
  LabComposer,
  LabFilterNote,
  LabNoteCard,
  LabSearchField,
  LabTagFilter,
  OutcomePill,
  agoLabel,
  toneClass,
  useLab,
  useLabStats,
} from "./jl-shared";

/**
 * 시안 B · **종목 노트북** — 메모를 종목 아래에 묶는다.
 *
 * # 무엇을 고치려는 배치인가
 *
 * 메모를 다시 찾는 이유는 거의 언제나 종목에서 시작한다 — 「삼성전자를 또 사려는데,
 * 저번에 내가 뭐라고 썼더라」. 지금 화면에서 그 질문에 답하려면 우측 레일 다섯 번째
 * 카드(「종목별 메모」)까지 내려가 필터를 걸어야 하고, 걸고 나면 화면은 여전히 제목만
 * 늘어놓은 목록이다. **그 종목이 지금 얼마인지는 어디에도 안 나온다.**
 *
 * 여기서는 종목이 곧 묶음이다. 카드 하나가 한 종목이고, 머리에 현재가·등락·최근
 * 흐름(실서비스 스파크라인 그대로)·그 종목에 쓴 메모의 평균 성과가 서고, 그 아래에
 * 내가 쓴 글이 시간순으로 붙는다. 종목 화면으로 가는 길과 퀵룩도 같은 카드에 있다.
 *
 * 종목을 안 붙인 메모(시장 전체 · 반성 · 환율)는 맨 아래 한 묶음으로 모은다 — 없는
 * 척하면 그 메모들이 화면에서 사라진다.
 */

type Group = {
  symbol: string;
  name: string;
  quote?: QuoteSummary;
  notes: JournalNote[];
  avg: number | null;
  /** 가장 최근 메모 시각 — 묶음 순서를 정한다. */
  latest: number;
};

export function JournalDeskConcept({ onOpen }: { onOpen: (note: JournalNote) => void }) {
  const { notes, quoteFor, pctFor, filtering } = useLab();

  const { groups, loose } = useMemo(() => {
    const map = new Map<string, Group>();
    const loose: JournalNote[] = [];

    for (const note of notes) {
      if (!note.symbol) {
        loose.push(note);
        continue;
      }
      const prev = map.get(note.symbol);
      const group: Group = prev ?? {
        symbol: note.symbol,
        name: note.symbolName || note.symbol,
        quote: quoteFor(note),
        notes: [],
        avg: null,
        latest: 0,
      };
      group.notes.push(note);
      group.latest = Math.max(group.latest, new Date(note.createdAt).getTime());
      map.set(note.symbol, group);
    }

    for (const group of map.values()) {
      const pcts = group.notes.map((n) => pctFor(n)).filter((p): p is number => p !== null);
      group.avg = pcts.length ? pcts.reduce((a, b) => a + b, 0) / pcts.length : null;
    }

    return {
      groups: Array.from(map.values()).sort((a, b) => b.latest - a.latest),
      loose,
    };
  }, [notes, quoteFor, pctFor]);

  return (
    <div className="min-w-0 space-y-3">
      <LabComposer />
      {!filtering && <SymbolStrip />}
      <LabFilterNote />

      {groups.length === 0 && loose.length === 0 && (
        <p className="rounded-xl border border-[var(--border)] p-8 text-center text-sm text-[var(--text-muted)]">
          조건에 맞는 메모가 없습니다.
        </p>
      )}

      {groups.map((group) => (
        <SymbolGroupCard key={group.symbol} group={group} onOpen={onOpen} />
      ))}

      {loose.length > 0 && (
        <Card className="min-w-0">
          <div className="mb-2 flex items-center gap-1.5 border-b border-[var(--border)] pb-2">
            <FileText className="size-4 text-[var(--text-muted)]" />
            <span className="text-note font-semibold text-[var(--text-primary)]">
              종목을 붙이지 않은 메모
            </span>
            <span className="font-num text-micro tabular-nums text-[var(--text-muted)]">
              {loose.length}건
            </span>
          </div>
          <div className="space-y-2">
            {loose.map((note) => (
              <LabNoteCard key={note.id} note={note} onOpen={() => onOpen(note)} compact />
            ))}
          </div>
        </Card>
      )}
    </div>
  );
}

function SymbolGroupCard({ group, onOpen }: { group: Group; onOpen: (n: JournalNote) => void }) {
  const quickLook = useQuickLook();
  const { pctFor } = useLab();
  const quote = group.quote;
  const currency = quote?.currency ?? group.notes[0]?.priceCurrency ?? "KRW";

  return (
    <Card className="min-w-0 p-0">
      {/* ── 머리: 이 종목이 지금 어떤 상태인가 ─────────────────────────── */}
      <button
        type="button"
        onClick={() => quickLook?.openSymbol(group.symbol, quote ?? null)}
        className="flex w-full min-w-0 items-center gap-2 border-b border-[var(--border)] px-2.5 py-2 text-left transition-colors hover:bg-[var(--surface-elevated)]"
      >
        <StockAvatar symbol={group.symbol} name={group.name} size="sm" className="shrink-0" />
        <span className="min-w-0 flex-1">
          <span className="block truncate text-sm font-semibold text-[var(--text-primary)]">
            {group.name}
          </span>
          <span className="block truncate font-num text-micro text-[var(--text-muted)]">
            {group.symbol} · 메모 {group.notes.length}건
          </span>
        </span>

        {quote && (
          <MarketRadarSparkline
            symbol={group.symbol}
            changeRate={quote.changeRate}
            className="hidden sm:flex"
          />
        )}

        <span className="shrink-0 text-right">
          <span className="block font-num text-sm font-semibold tabular-nums text-[var(--text-primary)]">
            <PriceDisplay value={quote?.currentPrice ?? 0} currency={currency} size="inherit" />
          </span>
          {quote && (
            <span
              className={cn(
                "block font-num text-micro tabular-nums",
                toneClass(quote.changeRate),
              )}
            >
              {formatPercent(quote.changeRate)}
            </span>
          )}
        </span>
      </button>

      {/* ── 이 종목에 쓴 글의 성적 ─────────────────────────────────────── */}
      {group.avg !== null && (
        <div className="flex items-center gap-2 border-b border-[var(--border)] bg-[var(--surface-elevated)]/50 px-2.5 py-1.5">
          <span className="text-micro text-[var(--text-muted)]">이 종목에 쓴 글의 작성 후 평균</span>
          <OutcomePill pct={group.avg} className="ml-auto" />
        </div>
      )}

      {/* ── 글 ─────────────────────────────────────────────────────────── */}
      <ul className="divide-y divide-[var(--border)]">
        {group.notes.map((note) => (
          <li key={note.id}>
            <button
              type="button"
              onClick={() => onOpen(note)}
              className="w-full min-w-0 px-2.5 py-2 text-left transition-colors hover:bg-[var(--surface-elevated)]"
            >
              <span className="flex min-w-0 items-center gap-2">
                <span className="min-w-0 flex-1 truncate text-cell font-medium text-[var(--text-primary)]">
                  {note.title || "제목 없음"}
                </span>
                <span className="shrink-0 text-micro text-[var(--text-muted)]">
                  {agoLabel(note.createdAt)}
                </span>
                <OutcomePill pct={pctFor(note)} />
              </span>
              <span className="mt-0.5 line-clamp-2 block text-cell text-[var(--text-muted)]">
                {note.body}
              </span>
              {(note.tags?.length ?? 0) > 0 && (
                <span className="mt-1 block">
                  <JournalTagChips tags={note.tags} />
                </span>
              )}
            </button>
          </li>
        ))}
      </ul>

      <div className="flex items-center gap-2 border-t border-[var(--border)] px-2.5 py-2">
        <Link
          href={analysisSymbolHref(group.symbol)}
          className={cn(
            buttonVariants({ variant: "outline", size: "sm" }),
            "border-[var(--border)] text-[var(--text-secondary)] hover:border-[var(--brand)]",
          )}
        >
          종목 분석 보기
          <ArrowRight className="size-3" />
        </Link>
      </div>
    </Card>
  );
}

/** 종목 사이를 건너뛰는 가로 띠. 묶음이 길어지면 스크롤 대신 여기를 누른다. */
function SymbolStrip() {
  const { symbols, symbol, setSymbol, quotes } = useLab();
  if (symbols.length === 0) return null;
  return (
    <div className="-mx-1 flex gap-1.5 overflow-x-auto px-1 pb-1">
      <button
        type="button"
        onClick={() => setSymbol(null)}
        aria-pressed={symbol === null}
        className={cn(
          "shrink-0 rounded-full border px-3 py-1.5 text-cell font-medium transition-colors",
          symbol === null
            ? "border-[var(--brand)] bg-[var(--brand-muted)] text-[var(--brand)]"
            : "border-[var(--border)] text-[var(--text-secondary)] hover:border-[var(--brand)]",
        )}
      >
        전체
      </button>
      {symbols.map((s) => (
        <button
          key={s.symbol}
          type="button"
          onClick={() => setSymbol(symbol === s.symbol ? null : s.symbol)}
          aria-pressed={symbol === s.symbol}
          className={cn(
            "flex shrink-0 items-center gap-1.5 rounded-full border px-2.5 py-1.5 text-cell font-medium transition-colors",
            symbol === s.symbol
              ? "border-[var(--brand)] bg-[var(--brand-muted)] text-[var(--brand)]"
              : "border-[var(--border)] text-[var(--text-secondary)] hover:border-[var(--brand)]",
          )}
        >
          <StockAvatar symbol={s.symbol} name={s.name} size="xs" />
          {s.name}
          <span
            className={cn(
              "font-num text-micro tabular-nums",
              toneClass(quotes[s.symbol]?.changeRate ?? null),
            )}
          >
            {quotes[s.symbol] ? formatPercent(quotes[s.symbol].changeRate) : `${s.count}`}
          </span>
        </button>
      ))}
    </div>
  );
}

export function JournalDeskRail() {
  const stats = useLabStats();

  return (
    <>
      <SectionCard title="메모 검색">
        <LabSearchField />
        <div className="mt-2">
          <LabTagFilter />
        </div>
      </SectionCard>

      <SectionCard title="종목별 성적" action={<span className="text-micro text-muted-foreground">작성 후</span>}>
        {stats.ranked.length === 0 ? (
          <SectionEmpty text="종목에 연결된 메모가 없습니다" />
        ) : (
          <ul className="min-w-0">
            {stats.ranked.slice(0, 6).map(({ note, pct }) => (
              <li
                key={note.id}
                className="flex h-[30px] min-w-0 items-center gap-1.5 px-1 text-cell"
              >
                <StockAvatar
                  symbol={note.symbol!}
                  name={note.symbolName}
                  size="xs"
                  className="shrink-0"
                />
                <span className="min-w-0 flex-1 truncate">{note.symbolName || note.symbol}</span>
                <span className={cn("font-num tabular-nums", toneClass(pct))}>
                  {pct > 0 ? "+" : ""}
                  {pct.toFixed(1)}%
                </span>
              </li>
            ))}
          </ul>
        )}
      </SectionCard>

      <SectionCard title="기록 현황">
        <SectionKeyValue label="전체" value={`${stats.total}건`} />
        <SectionKeyValue label="종목 연결" value={`${stats.linked}건`} />
        {stats.avg !== null && (
          <SectionKeyValue
            label="작성 후 평균"
            value={`${stats.avg > 0 ? "+" : ""}${stats.avg.toFixed(1)}%`}
            valueClassName={toneClass(stats.avg)}
          />
        )}
      </SectionCard>
    </>
  );
}
