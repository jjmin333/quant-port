"use client";

import { useMemo } from "react";
import { History, Pin, Trophy } from "lucide-react";
import { StatRow } from "@/components/layout/page-columns";
import { StockAvatar } from "@/components/stocks/stock-avatar";
import { Card } from "@/components/ui/card";
import { SectionCard, SectionEmpty, SectionKeyValue } from "@/components/ui/section-card";
import { formatDateKST } from "@/lib/format";
import { JOURNAL_TAG_TONE, type JournalNote } from "@/lib/journal-types";
import { cn } from "@/lib/utils";
import {
  LabComposer,
  LabFilterNote,
  LabNoteCard,
  LabSearchField,
  LabTagFilter,
  OutcomePill,
  OutcomeTrack,
  agoLabel,
  daysSince,
  toneClass,
  useLab,
  useLabStats,
} from "./jl-shared";

/**
 * 시안 C · **복기 보드** — 「내가 쓴 판단이 실제로 어땠나」를 화면의 주제로 삼는다.
 *
 * # 왜 이런 화면을 후보에 넣나
 *
 * 사용자가 말한 「크게 사용할 만한 메리트를 모르겠다」에 정면으로 답하는 쪽이다.
 * 메모 자체는 어떤 앱으로도 쓸 수 있다. 이 앱에서만 되는 것은 **저장하는 순간 그때
 * 주가가 함께 박제된다**는 것이고(`journal_notes.price_at_write` — 이후 UPDATE 금지),
 * 그래서 몇 달 뒤 「그 판단이 맞았나」에 계산 없이 답할 수 있다.
 *
 * 그 값이 지금 화면에서는 목록 행 끝의 작은 % 하나와 레일 맨 아래 두 줄로만 쓰인다.
 * 여기서는 그것을 첫 화면으로 끌어올린다 — 성적표, 크게 맞은 순서, 태그별 성적,
 * 그리고 아직 답을 확인하지 않은 옛 메모.
 *
 * # 대신 잃는 것
 *
 * 「쓰는 화면」의 느낌은 A(기록장)가 가장 강하고 여기는 가장 약하다. 그래서 작성 바는
 * 여기서도 맨 위에 그대로 둔다 — 성적표가 먼저 보이더라도 쓰는 길이 한 클릭이어야
 * 메모장이다.
 */

const STALE_DAYS = 30;

export function JournalReviewConcept({ onOpen }: { onOpen: (note: JournalNote) => void }) {
  const { notes, all, pctFor } = useLab();
  const stats = useLabStats();

  /** 30일 넘게 손대지 않은, 아직 답을 안 본 메모. */
  const stale = useMemo(
    () =>
      all
        .filter((n) => daysSince(n.updatedAt) >= STALE_DAYS)
        .sort((a, b) => new Date(a.updatedAt).getTime() - new Date(b.updatedAt).getTime())
        .slice(0, 5),
    [all],
  );

  return (
    <div className="min-w-0 space-y-3">
      <LabComposer />

      {/* ── 성적표 ─────────────────────────────────────────────────────── */}
      <StatRow>
        <StatTile label="기록" value={`${stats.total}건`} hint={`고정 ${stats.pinned}건`} />
        <StatTile
          label="가격이 박제된 메모"
          value={`${stats.scored}건`}
          hint={`종목 연결 ${stats.linked}건`}
        />
        <StatTile
          label="작성 후 평균"
          value={stats.avg === null ? "—" : `${stats.avg > 0 ? "+" : ""}${stats.avg.toFixed(1)}%`}
          valueClass={toneClass(stats.avg)}
          hint="그때 값 대비 지금"
        />
        <StatTile
          label="맞은 판단"
          value={`${stats.up} / ${stats.scored}`}
          hint={`틀린 판단 ${stats.down}건`}
        />
      </StatRow>

      {/* ── 크게 맞은 순서 ─────────────────────────────────────────────── */}
      <Card className="min-w-0">
        <div className="mb-2 flex items-center gap-1.5">
          <Trophy className="size-4 text-[var(--brand)]" />
          <h2 className="text-note font-semibold text-[var(--text-primary)]">판단의 성적표</h2>
          <span className="text-micro text-[var(--text-muted)]">
            쓴 날의 가격 대비 지금 · 큰 순서
          </span>
        </div>
        {stats.ranked.length === 0 ? (
          <p className="py-6 text-center text-cell text-[var(--text-muted)]">
            종목을 연결한 메모가 아직 없습니다.
          </p>
        ) : (
          <ul className="divide-y divide-[var(--border)]">
            {stats.ranked.map(({ note }) => (
              <li key={note.id}>
                <button
                  type="button"
                  onClick={() => onOpen(note)}
                  className="w-full min-w-0 rounded-md px-1 py-2 text-left transition-colors hover:bg-[var(--surface-elevated)]"
                >
                  <span className="flex min-w-0 items-center gap-1.5">
                    <StockAvatar
                      symbol={note.symbol!}
                      name={note.symbolName}
                      size="xs"
                      className="shrink-0"
                    />
                    <span className="min-w-0 flex-1 truncate text-cell font-medium text-[var(--text-primary)]">
                      {note.title || "제목 없음"}
                    </span>
                    <span className="shrink-0 text-micro text-[var(--text-muted)]">
                      {agoLabel(note.createdAt)}
                    </span>
                  </span>
                  <OutcomeTrack note={note} className="mt-1.5" />
                </button>
              </li>
            ))}
          </ul>
        )}
      </Card>

      {/* ── 아직 답을 안 본 메모 ───────────────────────────────────────── */}
      {stale.length > 0 && (
        <Card className="min-w-0">
          <div className="mb-2 flex items-center gap-1.5">
            <History className="size-4 text-[var(--text-muted)]" />
            <h2 className="text-note font-semibold text-[var(--text-primary)]">다시 볼 메모</h2>
            <span className="text-micro text-[var(--text-muted)]">
              {STALE_DAYS}일 넘게 안 열어본 것
            </span>
          </div>
          <ul className="divide-y divide-[var(--border)]">
            {stale.map((note) => (
              <li key={note.id}>
                <button
                  type="button"
                  onClick={() => onOpen(note)}
                  className="flex w-full min-w-0 items-center gap-2 rounded-md px-1 py-2 text-left transition-colors hover:bg-[var(--surface-elevated)]"
                >
                  {note.symbol ? (
                    <StockAvatar
                      symbol={note.symbol}
                      name={note.symbolName}
                      size="xs"
                      className="shrink-0"
                    />
                  ) : (
                    <span className="size-4 shrink-0 rounded-full bg-[var(--surface-elevated)]" />
                  )}
                  <span className="min-w-0 flex-1">
                    <span className="block truncate text-cell font-medium text-[var(--text-primary)]">
                      {note.title || "제목 없음"}
                    </span>
                    <span className="block truncate text-micro text-[var(--text-muted)]">
                      {formatDateKST(note.createdAt)} · {daysSince(note.updatedAt)}일 지남
                    </span>
                  </span>
                  <OutcomePill pct={pctFor(note)} />
                </button>
              </li>
            ))}
          </ul>
        </Card>
      )}

      {/* ── 전체 기록 ──────────────────────────────────────────────────── */}
      <section className="space-y-2">
        <div className="flex items-center gap-2 pt-1">
          <h2 className="text-note font-semibold text-[var(--text-secondary)]">전체 기록</h2>
          <span className="font-num text-micro tabular-nums text-[var(--text-muted)]">
            {notes.length}건
          </span>
          <span className="h-px min-w-0 flex-1 bg-[var(--border)]" />
        </div>
        <LabFilterNote />
        {notes.length === 0 ? (
          <p className="rounded-xl border border-[var(--border)] p-8 text-center text-sm text-[var(--text-muted)]">
            조건에 맞는 메모가 없습니다.
          </p>
        ) : (
          notes.map((note) => (
            <LabNoteCard key={note.id} note={note} onOpen={() => onOpen(note)} compact />
          ))
        )}
      </section>
    </div>
  );
}

function StatTile({
  label,
  value,
  hint,
  valueClass,
}: {
  label: string;
  value: string;
  hint?: string;
  valueClass?: string;
}) {
  return (
    <Card className="min-w-0">
      <p className="truncate text-micro text-[var(--text-muted)]">{label}</p>
      <p className={cn("mt-0.5 font-num text-base font-semibold tabular-nums", valueClass)}>
        {value}
      </p>
      {hint && <p className="mt-0.5 truncate text-micro text-[var(--text-muted)]">{hint}</p>}
    </Card>
  );
}

export function JournalReviewRail() {
  const { all } = useLab();
  const stats = useLabStats();
  const pinned = all.filter((n) => n.pinned);

  return (
    <>
      <SectionCard title="메모 검색">
        <LabSearchField />
        <div className="mt-2">
          <LabTagFilter />
        </div>
      </SectionCard>

      {/*
        태그별 성적 — 이 화면에서만 나오는 숫자다. 「매수근거」라고 적어 둔 판단과
        「리스크」라고 적어 둔 판단이 실제로 어떻게 갈렸는지는 태그를 쓰는 사람만
        받을 수 있는 되먹임이고, 지금 화면에는 이 자리가 없다.
      */}
      <SectionCard title="태그별 성적" action={<span className="text-micro text-muted-foreground">작성 후 평균</span>}>
        {stats.byTag.length === 0 ? (
          <SectionEmpty text="태그가 붙은 메모가 없습니다" />
        ) : (
          <ul className="min-w-0 space-y-1">
            {stats.byTag.map(({ tag, count, avg }) => (
              <li key={tag} className="flex min-w-0 items-center gap-2">
                <span
                  className={cn(
                    "shrink-0 rounded-full px-2 py-0.5 text-micro font-medium",
                    JOURNAL_TAG_TONE[tag] ??
                      "bg-[var(--surface-elevated)] text-[var(--text-secondary)]",
                  )}
                >
                  {tag}
                </span>
                <span className="font-num text-micro tabular-nums text-[var(--text-muted)]">
                  {count}건
                </span>
                <span
                  className={cn(
                    "ml-auto shrink-0 font-num text-cell font-semibold tabular-nums",
                    toneClass(avg),
                  )}
                >
                  {avg > 0 ? "+" : ""}
                  {avg.toFixed(1)}%
                </span>
              </li>
            ))}
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
              <li key={note.id} className="min-w-0 px-1 py-1.5">
                <span className="line-clamp-1 block text-cell">{note.title || "제목 없음"}</span>
                <span className="mt-0.5 block truncate text-micro text-muted-foreground">
                  {note.symbolName || note.symbol || "종목 미지정"} · {agoLabel(note.createdAt)}
                </span>
              </li>
            ))}
          </ul>
        )}
      </SectionCard>

      <SectionCard title="기록 현황">
        <SectionKeyValue label="전체" value={`${stats.total}건`} />
        <SectionKeyValue label="종목 연결" value={`${stats.linked}건`} />
        <SectionKeyValue label="플러스 / 마이너스" value={`${stats.up} / ${stats.down}`} />
      </SectionCard>
    </>
  );
}
