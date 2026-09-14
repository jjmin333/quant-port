"use client";

import Link from "next/link";
import { useMemo, useState } from "react";
import { ArrowRight, Pencil, Pin, Plus, Trash2, X } from "lucide-react";
import { PageColumns } from "@/components/layout/page-columns";
import { PageActions, PageScrollShell } from "@/components/layout/page-content-shell";
import { JournalNoteForm } from "@/components/journal/journal-note-form";
import { JournalNoteModal } from "@/components/journal/journal-note-modal";
import {
  JournalPriceStrip,
  JournalSymbolChip,
  JournalTagChips,
} from "@/components/journal/journal-shared";
import { JournalSideRail } from "@/components/journal/journal-side-rail";
import { Card, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { MenuEmptyState } from "@/components/common/menu-empty-state";
import { menuIcon } from "@/components/common/menu-icon";
import {
  SampleModeButton,
  SampleRibbon,
  SampleWrap,
  useAutoSample,
  useSampleMode,
} from "@/components/common/sample-mode";
import { useAuth } from "@/components/providers/auth-provider";
import { useConfirm } from "@/components/providers/confirm-provider";
import { useJournalNotes, type JournalSort } from "@/hooks/use-journal-notes";
import { analysisSymbolHref } from "@/lib/analysis-tabs";
import { formatDateTimeKST, formatDateKST } from "@/lib/format";
import { noteReturnPct, type JournalNote, type JournalNoteInput } from "@/lib/journal-types";
import { MENU_COPY } from "@/lib/menu-copy";
import type { QuoteSummary } from "@/lib/quotes-types";
import { cn } from "@/lib/utils";
import { buttonVariants } from "@/components/ui/button";

const COPY = MENU_COPY.journal;

const SORT_OPTIONS: { id: JournalSort; label: string }[] = [
  { id: "updated", label: "최신순" },
  { id: "created", label: "작성순" },
];

/**
 * 메모장 — 2단 인박스(목록 | 본문).
 *
 * 예전에는 한 컬럼에 작성 폼이 **항상 펼쳐진 채** 맨 위에 있었고, 그 아래 목록의
 * 본문은 클램프 없이 전부 나왔다. 긴 메모 하나가 화면을 다 먹고, 수정을 누르면
 * 폼이 목록 위에 하나 더 떠서 고치는 중인 행은 저 아래 남았다.
 *
 * 왼쪽은 제목·종목·등락만 한 줄로, 오른쪽이 고른 메모 하나를 온전히 보여준다.
 * 좁은 화면(<lg)에서는 목록만 남고, 행을 누르면 모달이 뜬다 — 두 컬럼을 억지로
 * 유지하면 둘 다 못 읽는다.
 */
export function JournalClient() {
  const { session } = useAuth();
  const confirm = useConfirm();
  const confirmDelete = () =>
    confirm({
      title: "이 메모를 삭제할까요?",
      description: "메모와 함께 저장된 작성 시점 가격도 사라집니다.",
      confirmLabel: "삭제",
      tone: "danger",
    });
  const { active: sample, setActive: setSample } = useSampleMode();
  const {
    notes,
    allNotes,
    totalCount,
    loading,
    error,
    query,
    setQuery,
    symbolFilter,
    setSymbolFilter,
    tagFilter,
    setTagFilter,
    sort,
    setSort,
    symbols,
    tags,
    quotes,
    createNote,
    updateNote,
    togglePin,
    removeNote,
  } = useJournalNotes({ sample });

  /** 오른쪽 본문에 세운 메모. `"new"`면 작성 폼. */
  const [selectedId, setSelectedId] = useState<number | "new">("new");
  /** 좁은 화면에서만 쓰는 모달. 넓은 화면은 오른쪽 칸이 이미 상세다. */
  const [modalId, setModalId] = useState<number | null>(null);

  /*
   * 0건 안내를 접고 **작성 화면으로 넘어갔는지**.
   *
   * 안내 카드의 「첫 메모 쓰기」는 `setSelectedId("new")`만 했는데, `selectedId`는
   * 애초에 `"new"`가 기본값이고 편집기는 `isEmpty`의 **반대쪽 분기**에 있다. 즉
   * 그 버튼은 아무 것도 하지 않았다 — 정확히 그 버튼이 필요한 사람(메모가 0건인
   * 사람)에게 눌러도 화면이 그대로였다(실측). 안내를 접는 플래그를 따로 둬서
   * 눌렀을 때 실제로 편집기가 있는 평소 레이아웃으로 넘어가게 한다.
   */
  const [composing, setComposing] = useState(false);
  /*
    「내 것이 아직 없다」 — **`sample`을 빼고** 잰다 (2026-09-14 사용자 요청 4).
    이 값이 `useAutoSample`로 가서 예시를 켠다. `!sample`이 섞이면 켜짐 → 0건 아님 →
    꺼짐이 무한히 반복된다.

    `composing`도 빼 둔다. 그건 「쓸 것이 없다」가 아니라 「지금 쓰고 있다」이고,
    쓰는 중에 예시가 꺼지는 것은 맞지만(아래 `isEmpty`가 아니라 아예 예시를 끄는
    것은 사용자 몫이다) 그것 때문에 예시가 켜졌다 꺼졌다 하면 안 된다.
  */
  const hasNothingYet = !loading && !error && totalCount === 0;
  useAutoSample(hasNothingYet);
  const isEmpty = !sample && hasNothingYet && !composing;

  // 목록이 필터로 줄어 고른 메모가 사라지면 첫 행으로 내린다. 선택이 허공을
  // 가리키면 오른쪽 칸이 아무 이유 없이 비어 보인다.
  const selectedNote = useMemo(
    () => (typeof selectedId === "number" ? notes.find((n) => n.id === selectedId) ?? null : null),
    [selectedId, notes],
  );
  const readerNote = selectedNote ?? (selectedId === "new" ? null : notes[0] ?? null);
  const modalNote = useMemo(
    () => (modalId === null ? null : allNotes.find((n) => n.id === modalId) ?? null),
    [modalId, allNotes],
  );

  const activeFilterLabel = symbolFilter
    ? symbols.find((s) => s.symbol === symbolFilter)?.name || symbolFilter
    : tagFilter;

  return (
    <>
      <PageScrollShell>
        {/*
          0건 안내 카드가 이미 같은 버튼을 들고 있다 (§R-2). 두 곳에 다 두면 한 화면에
          「예시로 둘러보기」가 둘이 되어 무엇이 다른지 묻게 만든다 — 실측으로 빈
          보유 화면의 보이는 버튼이 그 문구 ×2 였다.
        */}
        <PageActions>
          {isEmpty ? null : (
            <SampleModeButton active={sample} onToggle={setSample} emphasize={!session} />
          )}
        </PageActions>
        {sample && (
          <div className="mb-3">
            <SampleRibbon
              loggedIn={Boolean(session)}
              loginHref="/login?next=%2Fjournal"
              onDismiss={() => setSample(false)}
              /*
                안내 카드가 들고 있던 첫 동작 — 카드가 예시 뒤로 물러났으니 여기 선다.
                예시를 함께 끈다: 메모를 쓰러 가는 사람에게 남의 메모가 옆에 서 있으면
                자기 것과 섞여 보인다.
              */
              action={{
                label: "첫 메모 쓰기",
                onClick: () => {
                  setSample(false);
                  setSelectedId("new");
                  setComposing(true);
                },
              }}
            />
          </div>
        )}

        {isEmpty ? (
          <MenuEmptyState
            icon={menuIcon("journal")}
            headline={COPY.emptyHeadline}
            description={COPY.emptyDescription}
            steps={COPY.steps}
            primary={{
              label: "첫 메모 쓰기",
              onClick: () => {
                setSelectedId("new");
                setComposing(true);
              },
            }}
            secondary={
              <SampleModeButton
                active={false}
                onToggle={() => setSample(true)}
                emphasize
                className="border border-[var(--sample-border)] px-3.5 py-1.5"
              />
            }
            footnote={COPY.emptyFootnote}
          />
        ) : (
          <PageColumns
            rail={
              <JournalSideRail
                query={query}
                onQueryChange={setQuery}
                symbols={symbols}
                symbolFilter={symbolFilter}
                onSymbolFilter={setSymbolFilter}
                tags={tags}
                tagFilter={tagFilter}
                onTagFilter={setTagFilter}
                allNotes={allNotes}
                quotes={quotes}
                onOpenNote={setSelectedId}
              />
            }
          >
            <SampleWrap active={sample}>
              <div className="grid min-w-0 gap-3 lg:grid-cols-[290px_minmax(0,1fr)] xl:grid-cols-[320px_minmax(0,1fr)]">
                {/* ── 왼쪽: 목록 ─────────────────────────────────────────── */}
                <Card className="min-w-0 p-0">
                  <CardHeader className="mb-0 gap-2 border-b border-[var(--border)] px-3 py-2.5">
                    <CardTitle className="text-cell">메모 목록</CardTitle>
                    <div className="flex items-center gap-1.5">
                      {!loading && (
                        <Badge variant="info">
                          {notes.length}
                          {totalCount !== notes.length && ` / ${totalCount}`}
                        </Badge>
                      )}
                      <div className="flex gap-0.5 rounded-lg bg-[var(--surface-elevated)] p-0.5">
                        {SORT_OPTIONS.map((opt) => (
                          <button
                            key={opt.id}
                            type="button"
                            onClick={() => setSort(opt.id)}
                            className={cn(
                              "rounded-md px-2 py-0.5 text-cell font-medium transition-colors",
                              sort === opt.id
                                ? "bg-[var(--surface)] text-[var(--text-primary)] shadow-sm"
                                : "text-[var(--text-muted)] hover:text-[var(--text-secondary)]",
                            )}
                          >
                            {opt.label}
                          </button>
                        ))}
                      </div>
                    </div>
                  </CardHeader>

                  {/* 예시 데이터는 서버에 없는 행이라 작성 버튼을 그리지 않는다. */}
                  {!sample && (
                    <div className="border-b border-[var(--border)] p-2">
                      <button
                        type="button"
                        onClick={() => setSelectedId("new")}
                        className={cn(
                          "flex w-full items-center justify-center gap-1.5 rounded-lg border border-dashed px-3 py-2 text-cell font-medium transition-colors",
                          selectedId === "new"
                            ? "border-[var(--brand)] bg-[var(--brand-muted)] text-[var(--brand)]"
                            : "border-[var(--border)] text-[var(--text-muted)] hover:border-[var(--brand)] hover:text-[var(--text-secondary)]",
                        )}
                      >
                        <Plus className="size-3.5" />새 메모 작성
                      </button>
                    </div>
                  )}

                  {activeFilterLabel && (
                    <div className="flex items-center gap-1.5 border-b border-[var(--border)] px-3 py-1.5 text-cell text-[var(--text-muted)]">
                      <span className="truncate">「{activeFilterLabel}」로 필터 중</span>
                      <button
                        type="button"
                        onClick={() => {
                          setSymbolFilter(null);
                          setTagFilter(null);
                        }}
                        className="ml-auto shrink-0 rounded-md p-0.5 hover:text-[var(--text-secondary)]"
                        aria-label="필터 해제"
                      >
                        <X className="size-3" />
                      </button>
                    </div>
                  )}

                  {loading ? (
                    <div className="space-y-2 p-2">
                      <div className="h-12 animate-pulse rounded-lg bg-[var(--surface-elevated)]" />
                      <div className="h-12 animate-pulse rounded-lg bg-[var(--surface-elevated)]" />
                      <div className="h-12 animate-pulse rounded-lg bg-[var(--surface-elevated)]" />
                    </div>
                  ) : error ? (
                    <p className="p-4 text-center text-sm text-[var(--text-muted)]">{error}</p>
                  ) : notes.length === 0 ? (
                    <p className="p-4 text-center text-sm text-[var(--text-muted)]">
                      {totalCount === 0 ? "작성된 메모가 없습니다." : "조건에 맞는 메모가 없습니다."}
                    </p>
                  ) : (
                    <ul className="max-h-[calc(100dvh-17rem)] divide-y divide-[var(--border)] overflow-y-auto">
                      {notes.map((note) => (
                        <NoteListRow
                          key={note.id}
                          note={note}
                          quote={note.symbol ? quotes[note.symbol] : undefined}
                          active={readerNote?.id === note.id && selectedId !== "new"}
                          onSelect={() => setSelectedId(note.id)}
                          onOpenModal={() => setModalId(note.id)}
                        />
                      ))}
                    </ul>
                  )}
                </Card>

                {/* ── 오른쪽: 본문 ───────────────────────────────────────── */}
                <Card className="hidden min-w-0 lg:block">
                  {selectedId === "new" && !sample ? (
                    <>
                      <CardHeader>
                        <CardTitle>새 메모</CardTitle>
                        {notes.length > 0 && (
                          <button
                            type="button"
                            onClick={() => setSelectedId(notes[0].id)}
                            className="text-cell text-[var(--text-muted)] hover:text-[var(--text-secondary)]"
                          >
                            취소
                          </button>
                        )}
                      </CardHeader>
                      <JournalNoteForm
                        key="new"
                        compact
                        onCancel={null}
                        onSubmit={async (input) => {
                          const created = await createNote(input);
                          setSelectedId(created.id);
                        }}
                      />
                    </>
                  ) : readerNote ? (
                    <NoteReader
                      note={readerNote}
                      quote={readerNote.symbol ? quotes[readerNote.symbol] : undefined}
                      readOnly={sample}
                      onTagSelect={setTagFilter}
                      onSave={async (input) => {
                        await updateNote(readerNote.id, input);
                      }}
                      onTogglePin={() => togglePin(readerNote)}
                      onDelete={async () => {
                        if (await confirmDelete()) {
                          removeNote(readerNote.id);
                          setSelectedId("new");
                        }
                      }}
                    />
                  ) : (
                    <p className="p-8 text-center text-sm text-[var(--text-muted)]">
                      왼쪽에서 메모를 고르면 여기에 펼쳐집니다.
                    </p>
                  )}
                </Card>
              </div>
            </SampleWrap>
          </PageColumns>
        )}

        {/* 좁은 화면 전용. 넓은 화면에서는 오른쪽 칸이 같은 내용을 이미 보여준다. */}
        <JournalNoteModal
          note={modalNote}
          quote={modalNote?.symbol ? quotes[modalNote.symbol] : undefined}
          open={modalId !== null}
          readOnly={sample}
          onClose={() => setModalId(null)}
          onSave={async (input) => {
            if (modalNote) await updateNote(modalNote.id, input);
          }}
          onTogglePin={() => {
            if (modalNote) togglePin(modalNote);
          }}
          onDelete={async () => {
            if (modalNote && (await confirmDelete())) {
              removeNote(modalNote.id);
              setModalId(null);
            }
          }}
        />
      </PageScrollShell>
    </>
  );
}

/* ── 목록 한 줄 ──────────────────────────────────────────────────────────── */

function NoteListRow({
  note,
  quote,
  active,
  onSelect,
  onOpenModal,
}: {
  note: JournalNote;
  quote?: QuoteSummary;
  active: boolean;
  onSelect: () => void;
  onOpenModal: () => void;
}) {
  const pct = noteReturnPct(note, quote?.currentPrice);

  return (
    <li>
      <button
        type="button"
        onClick={() => {
          onSelect();
          // <lg 에서는 오른쪽 칸이 없다. 같은 클릭이 그쪽에서는 모달을 열어야 한다.
          if (window.matchMedia("(max-width: 1023px)").matches) onOpenModal();
        }}
        className={cn(
          "w-full min-w-0 px-3 py-2.5 text-left transition-colors",
          active
            ? "bg-[var(--brand-muted)] shadow-[inset_2px_0_0_var(--brand)]"
            : "hover:bg-[var(--surface-elevated)]",
        )}
      >
        <div className="flex min-w-0 items-center gap-1.5">
          {note.pinned && (
            <Pin className="size-3 shrink-0 fill-current text-[var(--brand)]" aria-label="고정됨" />
          )}
          <span className="min-w-0 flex-1 truncate text-cell font-medium text-[var(--text-primary)]">
            {note.title || "제목 없음"}
          </span>
          {pct !== null && (
            <span
              className={cn(
                "shrink-0 font-num text-micro font-semibold tabular-nums",
                pct > 0
                  ? "text-[var(--profit)]"
                  : pct < 0
                    ? "text-[var(--loss)]"
                    : "text-[var(--text-muted)]",
              )}
            >
              {pct > 0 ? "+" : ""}
              {pct.toFixed(1)}%
            </span>
          )}
        </div>
        <p className="mt-0.5 line-clamp-1 text-cell text-[var(--text-muted)]">{note.body}</p>
        <div className="mt-1 flex min-w-0 items-center gap-1.5">
          <span className="shrink-0 text-micro text-[var(--text-muted)]">
            {note.symbolName || note.symbol || "종목 미지정"}
          </span>
          <span className="shrink-0 text-micro text-[var(--text-muted)]">·</span>
          <span className="shrink-0 text-micro text-[var(--text-muted)]">
            {formatDateKST(note.updatedAt)}
          </span>
          <JournalTagChips tags={note.tags?.slice(0, 2)} className="ml-auto" />
        </div>
      </button>
    </li>
  );
}

/* ── 오른쪽 본문 ─────────────────────────────────────────────────────────── */

function NoteReader({
  note,
  quote,
  readOnly,
  onTagSelect,
  onSave,
  onTogglePin,
  onDelete,
}: {
  note: JournalNote;
  quote?: QuoteSummary;
  readOnly?: boolean;
  onTagSelect: (tag: string) => void;
  onSave: (input: JournalNoteInput) => Promise<void>;
  onTogglePin: () => void;
  onDelete: () => void;
}) {
  const [editing, setEditing] = useState(false);

  // 다른 메모로 넘어가면 편집을 접는다. 렌더 중 조정 — 이펙트로 하면 이전 메모의
  // 폼이 새 메모 위에 한 프레임 그려진다.
  const [lastId, setLastId] = useState(note.id);
  if (lastId !== note.id) {
    setLastId(note.id);
    if (editing) setEditing(false);
  }

  if (editing) {
    return (
      <>
        <CardHeader>
          <CardTitle>메모 수정</CardTitle>
        </CardHeader>
        <JournalNoteForm
          key={note.id}
          initial={note}
          compact
          autoFocus
          onCancel={() => setEditing(false)}
          onSubmit={async (input) => {
            await onSave(input);
            setEditing(false);
          }}
        />
      </>
    );
  }

  return (
    <div className="flex min-h-[280px] flex-col">
      <div className="flex min-w-0 items-start gap-2 border-b border-[var(--border)] pb-3">
        <div className="min-w-0 flex-1">
          <div className="flex min-w-0 flex-wrap items-center gap-1.5">
            {note.pinned && <Pin className="size-3.5 fill-current text-[var(--brand)]" />}
            <h2 className="min-w-0 text-base font-semibold text-[var(--text-primary)]">
              {note.title || "제목 없음"}
            </h2>
            <JournalSymbolChip note={note} />
          </div>
          <time className="mt-0.5 block text-cell text-[var(--text-muted)]">
            {note.updatedAt !== note.createdAt ? "수정됨 " : "작성 "}
            {formatDateTimeKST(note.updatedAt)}
          </time>
        </div>
        {!readOnly && (
          <div className="flex shrink-0 items-center gap-1">
            <button
              type="button"
              onClick={() => setEditing(true)}
              aria-label="수정"
              className="rounded-md p-1.5 text-[var(--text-muted)] hover:text-[var(--text-secondary)]"
            >
              <Pencil className="size-3.5" />
            </button>
            <button
              type="button"
              onClick={onTogglePin}
              aria-pressed={note.pinned}
              aria-label={note.pinned ? "고정 해제" : "고정"}
              className={cn(
                "rounded-md p-1.5 transition-colors",
                note.pinned
                  ? "text-[var(--brand)]"
                  : "text-[var(--text-muted)] hover:text-[var(--text-secondary)]",
              )}
            >
              <Pin className={cn("size-3.5", note.pinned && "fill-current")} />
            </button>
            <button
              type="button"
              onClick={onDelete}
              aria-label="삭제"
              className="rounded-md p-1.5 text-[var(--text-muted)] hover:text-rose-400"
            >
              <Trash2 className="size-3.5" />
            </button>
          </div>
        )}
      </div>

      <div className="space-y-3 pt-3">
        <JournalPriceStrip note={note} quote={quote} />
        <JournalTagChips tags={note.tags} onSelect={onTagSelect} />
        <p className="whitespace-pre-wrap text-read text-[var(--text-secondary)]">
          {note.body}
        </p>
      </div>

      {note.symbol && (
        <div className="mt-auto flex items-center gap-2 border-t border-[var(--border)] pt-3">
          <Link
            href={analysisSymbolHref(note.symbol)}
            className={cn(buttonVariants({ variant: "outline", size: "sm" }), "border-[var(--border)] text-[var(--text-secondary)] hover:border-[var(--brand)]")}
          >
            종목 분석 보기
            <ArrowRight className="size-3" />
          </Link>
        </div>
      )}
    </div>
  );
}
