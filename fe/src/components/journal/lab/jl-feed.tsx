"use client";

import { useMemo } from "react";
import { Pin, Sparkles } from "lucide-react";
import { StockAvatar } from "@/components/stocks/stock-avatar";
import { SectionCard, SectionEmpty, SectionKeyValue } from "@/components/ui/section-card";
import type { JournalNote } from "@/lib/journal-types";
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
 * 시안 A · **기록장** — 쓰는 곳이 맨 위, 그 아래로 한 줄짜리 연대기.
 *
 * # 무엇을 고치려는 배치인가
 *
 * 지금 화면은 1280px에서 목록(290px) · 본문(나머지) 2단인데, **들어오자마자 그 본문이
 * 비어 있다**(「왼쪽에서 메모를 고르면 여기에 펼쳐집니다」). 화면에서 가장 넓은 칸이
 * 가장 오래 아무 말도 하지 않고, 정작 메모장에서 가장 자주 하는 일(쓰기)은 왼쪽 카드
 * 안쪽의 점선 버튼이다.
 *
 * 여기서는 뒤집는다 — 본문은 **한 컬럼**이고, 맨 위가 작성 자리이며, 그 아래로 메모가
 * 시간순으로 쌓인다. 고를 필요가 없으니 빈 칸도 없다. 상세는 앱의 다른 화면과 같은
 * 방식, 즉 모달로 연다.
 *
 * # 「답이 오는 메모」
 *
 * 메모장을 다시 열게 만드는 것은 새로 쓰고 싶은 마음이 아니라 **옛 메모가 맞았는지
 * 궁금한 마음**이다. 그래서 맨 위에 그 답을 먼저 놓는다 — 작성 이후 ±10% 넘게 움직인
 * 메모만 골라 가로 띠 하나로. 지금 화면에서 이 정보는 우측 레일 세 번째 카드 안에
 * 있고, 좁은 화면에서는 목록 전체 아래로 내려간다.
 */

const MOVED = 10;

export function JournalFeedConcept({ onOpen }: { onOpen: (note: JournalNote) => void }) {
  const { notes, filtering } = useLab();

  /**
   * 고정한 메모는 **연대기 밖**에 따로 세운다.
   *
   * 처음엔 지금 화면처럼 고정을 목록 맨 위로 올렸는데, 그러면 8월에 쓴 고정 메모가
   * 「2026년 9월」 머리글 아래에 앉는다(실측). 시간순이 이 시안의 뼈대라 그 한 줄이
   * 뼈대를 거짓말로 만든다. 고정은 시간이 아니라 **내가 정한 우선순위**이므로 자기
   * 자리를 따로 갖는 게 맞다.
   */
  const pinned = useMemo(() => notes.filter((n) => n.pinned), [notes]);

  /** 월별 묶음. 「9월 · 4건」 같은 머리글이 스크롤의 눈금이 된다. */
  const groups = useMemo(() => {
    const map = new Map<string, JournalNote[]>();
    const chronological = [...notes].sort(
      (a, b) => new Date(b.createdAt).getTime() - new Date(a.createdAt).getTime(),
    );
    for (const note of chronological) {
      const d = new Date(note.createdAt);
      const key = `${d.getFullYear()}-${d.getMonth() + 1}`;
      const arr = map.get(key) ?? [];
      arr.push(note);
      map.set(key, arr);
    }
    return Array.from(map, ([key, rows]) => {
      const [y, m] = key.split("-");
      return { key, label: `${y}년 ${m}월`, rows };
    });
  }, [notes]);

  return (
    <div className="min-w-0 space-y-3">
      <LabComposer />
      {!filtering && <AnswerStrip onOpen={onOpen} />}
      <LabFilterNote />

      {pinned.length > 0 && (
        <section className="space-y-2">
          <div className="flex items-center gap-2 pt-1">
            <Pin className="size-3.5 fill-current text-[var(--brand)]" />
            <h2 className="text-note font-semibold text-[var(--text-secondary)]">고정한 메모</h2>
            <span className="font-num text-micro tabular-nums text-[var(--text-muted)]">
              {pinned.length}건
            </span>
            <span className="h-px min-w-0 flex-1 bg-[var(--border)]" />
          </div>
          {pinned.map((note) => (
            <LabNoteCard key={note.id} note={note} onOpen={() => onOpen(note)} compact />
          ))}
        </section>
      )}

      {notes.length === 0 ? (
        <p className="rounded-xl border border-[var(--border)] p-8 text-center text-sm text-[var(--text-muted)]">
          조건에 맞는 메모가 없습니다.
        </p>
      ) : (
        groups.map((group) => (
          <section key={group.key} className="space-y-2">
            <div className="flex items-center gap-2 pt-1">
              <h2 className="text-note font-semibold text-[var(--text-secondary)]">
                {group.label}
              </h2>
              <span className="font-num text-micro tabular-nums text-[var(--text-muted)]">
                {group.rows.length}건
              </span>
              <span className="h-px min-w-0 flex-1 bg-[var(--border)]" />
            </div>
            <div className="space-y-2">
              {group.rows.map((note) => (
                <LabNoteCard key={note.id} note={note} onOpen={() => onOpen(note)} />
              ))}
            </div>
          </section>
        ))
      )}
    </div>
  );
}

/** 작성 이후 크게 움직인 메모만. 없으면 줄 자체를 그리지 않는다. */
function AnswerStrip({ onOpen }: { onOpen: (note: JournalNote) => void }) {
  const { all, pctFor } = useLab();

  const moved = useMemo(
    () =>
      all
        .map((note) => ({ note, pct: pctFor(note) }))
        .filter((r): r is { note: JournalNote; pct: number } => r.pct !== null && Math.abs(r.pct) >= MOVED)
        .sort((a, b) => Math.abs(b.pct) - Math.abs(a.pct))
        .slice(0, 6),
    [all, pctFor],
  );

  if (moved.length === 0) return null;

  return (
    <section className="min-w-0">
      <div className="mb-1.5 flex items-center gap-1.5">
        <Sparkles className="size-3.5 text-[var(--brand)]" />
        <h2 className="text-note font-semibold text-[var(--text-primary)]">답이 오고 있는 메모</h2>
        <span className="text-micro text-[var(--text-muted)]">
          쓴 뒤 {MOVED}% 넘게 움직인 것
        </span>
      </div>
      <div className="-mx-1 flex gap-2 overflow-x-auto px-1 pb-1">
        {moved.map(({ note, pct }) => (
          <button
            key={note.id}
            type="button"
            onClick={() => onOpen(note)}
            className={cn(
              "flex w-[172px] shrink-0 flex-col gap-1.5 rounded-xl border p-2.5 text-left transition-colors hover:border-[var(--brand)]",
              pct > 0
                ? "border-[var(--profit)]/25 bg-[var(--profit)]/[0.06]"
                : "border-[var(--loss)]/25 bg-[var(--loss)]/[0.06]",
            )}
          >
            <div className="flex min-w-0 items-center gap-1.5">
              {note.symbol && (
                <StockAvatar symbol={note.symbol} name={note.symbolName} size="xs" />
              )}
              <span className="min-w-0 flex-1 truncate text-cell font-medium text-[var(--text-secondary)]">
                {note.symbolName || note.symbol}
              </span>
              <OutcomePill pct={pct} />
            </div>
            <span className="line-clamp-2 text-cell font-medium text-[var(--text-primary)]">
              {note.title || "제목 없음"}
            </span>
            <span className="text-micro text-[var(--text-muted)]">
              {agoLabel(note.createdAt)} 기록
            </span>
          </button>
        ))}
      </div>
    </section>
  );
}

/**
 * 이 시안의 레일 — **세 칸만** 둔다.
 *
 * 지금 화면의 레일은 카드 여섯 장(검색 · 태그 · 다시 볼 메모 · 고정 · 종목별 · 현황)
 * 812px이고, `xl` 미만에서는 그게 통째로 본문 아래로 쏟아진다. 「다시 볼 메모」와
 * 「고정」은 본문이 시간순으로 다 보여 주므로 레일에서 뺀다 — 레일은 **찾는 도구**만
 * 갖는다.
 */
export function JournalFeedRail() {
  const { symbols, symbol, setSymbol } = useLab();
  const stats = useLabStats();

  return (
    <>
      <SectionCard title="메모 검색">
        <LabSearchField />
        <div className="mt-2">
          <LabTagFilter />
        </div>
      </SectionCard>

      <SectionCard title="종목별 메모">
        {symbols.length === 0 ? (
          <SectionEmpty text="종목에 연결된 메모가 없습니다" />
        ) : (
          <ul className="min-w-0">
            {symbols.map((s) => (
              <li key={s.symbol}>
                <button
                  type="button"
                  onClick={() => setSymbol(symbol === s.symbol ? null : s.symbol)}
                  aria-pressed={symbol === s.symbol}
                  className={cn(
                    "flex h-[30px] w-full min-w-0 items-center gap-1.5 rounded-md px-1 text-cell",
                    symbol === s.symbol ? "bg-accent font-medium" : "hover:bg-accent/60",
                  )}
                >
                  <StockAvatar symbol={s.symbol} name={s.name} size="xs" className="shrink-0" />
                  <span className="min-w-0 flex-1 truncate text-left">{s.name}</span>
                  <span className="font-num text-micro tabular-nums text-[var(--text-muted)]">
                    {s.count}
                  </span>
                </button>
              </li>
            ))}
          </ul>
        )}
      </SectionCard>

      <SectionCard title="기록 현황">
        <SectionKeyValue label="전체" value={`${stats.total}건`} />
        <SectionKeyValue label="종목 연결" value={`${stats.linked}건`} />
        <SectionKeyValue label="고정" value={`${stats.pinned}건`} />
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
