"use client";

import { useState } from "react";
import Link from "next/link";
import { ArrowRight, Pencil, Pin, Trash2 } from "lucide-react";
import { PriceHistoryChart } from "@/components/charts/price-history-chart";
import { JournalNoteForm } from "@/components/journal/journal-note-form";
import { JournalPriceStrip, JournalTagChips } from "@/components/journal/journal-shared";
import { StockAvatar } from "@/components/stocks/stock-avatar";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { useSparklineCandles } from "@/hooks/use-sparkline-candles";
import { analysisSymbolHref } from "@/lib/analysis-tabs";
import { formatDateTimeKST, formatPrice } from "@/lib/format";
import type { JournalNote, JournalNoteInput } from "@/lib/journal-types";
import { usePriceTargets } from "@/lib/price-targets";
import type { QuoteSummary } from "@/lib/quotes-types";
import { cn } from "@/lib/utils";

/**
 * 메모 상세 모달.
 *
 * 관심·보유·스탬프·뉴스가 이미 전부 모달이다. 메모만 인라인 폼이라 같은 종목을
 * 어디서 눌렀느냐에 따라 다른 화면이 떴다.
 *
 * 차트에는 **작성한 날의 가격**을 기준선으로 얹는다(`markerPrice`). 보유 화면이
 * 평단을 얹는 것과 같은 장치로, "이 글을 쓴 자리"가 어디였는지가 선 하나로 보인다.
 */
export function JournalNoteModal({
  note,
  quote,
  open,
  onClose,
  onSave,
  onTogglePin,
  onDelete,
  readOnly,
}: {
  note: JournalNote | null;
  quote?: QuoteSummary;
  open: boolean;
  onClose: () => void;
  onSave: (input: JournalNoteInput) => Promise<void>;
  onTogglePin: () => void;
  onDelete: () => void;
  /** 예시 모드 — 서버에 없는 행이라 수정·삭제 버튼을 그리지 않는다. */
  readOnly?: boolean;
}) {
  const [editing, setEditing] = useState(false);
  const targets = usePriceTargets();

  // 열림/닫힘에 딸린 초기화는 이펙트가 아니라 렌더 중 조정 — 이펙트로 하면 편집
  // 상태가 남은 프레임이 한 번 더 그려진다.
  const [wasOpen, setWasOpen] = useState(open);
  if (wasOpen !== open) {
    setWasOpen(open);
    if (!open && editing) setEditing(false);
  }

  const symbol = note?.symbol ?? "";
  /*
   * 예시 모드에서는 캔들을 부르지 않는다. 예시 메모의 「작성 당시 가격」은 픽스처라
   * 실제 캔들 범위 밖에 있고, 그러면 기준선이 차트 밖으로 밀려 아예 안 보인다 —
   * 선이 없는 게 아니라 **잘못된 자리에 있는 것처럼** 읽힌다.
   */
  const { points, times } = useSparklineCandles(symbol, open && Boolean(symbol) && !readOnly);
  const target = targets.find((t) => t.symbol === symbol);
  const currency = note?.priceCurrency ?? quote?.currency ?? "KRW";

  return (
    <Dialog open={open} onOpenChange={(next) => !next && onClose()}>
      <DialogContent className="flex max-h-[88dvh] flex-col gap-0 overflow-hidden p-0 sm:max-w-lg">
        {note ? (
          <>
            <DialogHeader className="shrink-0 border-b px-4 py-3 pr-10 text-left">
              <DialogTitle className="flex min-w-0 items-start gap-2 text-sm">
                {note.symbol && (
                  <StockAvatar
                    symbol={note.symbol}
                    name={note.symbolName}
                    size="sm"
                    className="mt-0.5 shrink-0"
                  />
                )}
                <span className="min-w-0 flex-1">{note.title || "제목 없음"}</span>
                {note.pinned && (
                  <Pin className="mt-0.5 size-3.5 shrink-0 fill-current text-[var(--brand)]" />
                )}
              </DialogTitle>
              <DialogDescription className="text-[11px]">
                {note.symbol ? `${note.symbolName || note.symbol} · ${note.symbol}` : "종목 미지정"}
                {" · "}
                {note.updatedAt !== note.createdAt ? "수정 " : "작성 "}
                {formatDateTimeKST(note.updatedAt)}
              </DialogDescription>
            </DialogHeader>

            <div className="min-h-0 flex-1 space-y-3 overflow-y-auto px-4 py-3">
              {editing ? (
                <JournalNoteForm
                  initial={note}
                  compact
                  autoFocus
                  onCancel={() => setEditing(false)}
                  onSubmit={async (input) => {
                    await onSave(input);
                    setEditing(false);
                  }}
                />
              ) : (
                <>
                  <JournalPriceStrip note={note} quote={quote} />

                  {note.symbol && points.length >= 2 && (
                    <div className="rounded-lg border border-[var(--border)] p-2.5">
                      <PriceHistoryChart
                        points={points}
                        times={times}
                        currency={currency}
                        markerPrice={note.priceAtWrite}
                        markerLabel="작성 시점"
                        title="최근 흐름"
                        height={104}
                      />
                    </div>
                  )}

                  <JournalTagChips tags={note.tags} />

                  <p className="whitespace-pre-wrap text-sm leading-relaxed text-[var(--text-secondary)]">
                    {note.body}
                  </p>

                  {/*
                    목표가는 앱 전체가 공유하는 스토어에서 읽는다 — 이 모달을 열자고
                    요청을 하나 더 내보내지 않는다. 없으면 줄 자체를 그리지 않는다.
                  */}
                  {target && (
                    <div className="rounded-lg border border-[var(--border)] px-3 py-2">
                      <p className="text-[11px] font-medium text-[var(--text-muted)]">이 종목의 목표가</p>
                      <p className="mt-1 flex flex-wrap items-baseline gap-2 text-sm">
                        <span className="font-mono font-semibold text-[var(--brand)]">
                          {formatPrice(target.targetPrice, target.currency)}
                        </span>
                        {quote?.currentPrice ? (
                          <span className="font-mono text-[11px] text-[var(--text-muted)]">
                            현재 대비{" "}
                            {(
                              ((target.targetPrice - quote.currentPrice) / quote.currentPrice) *
                              100
                            ).toFixed(1)}
                            %
                          </span>
                        ) : null}
                        {target.note && (
                          <span className="text-[11px] text-[var(--text-muted)]">{target.note}</span>
                        )}
                      </p>
                    </div>
                  )}
                </>
              )}
            </div>

            {!editing && (
              <div className="flex shrink-0 items-center gap-2 border-t px-4 py-2.5">
                {!readOnly && (
                  <>
                    <button
                      type="button"
                      onClick={() => setEditing(true)}
                      className="inline-flex items-center gap-1.5 rounded-lg border border-[var(--border)] px-3 py-1.5 text-xs font-medium text-[var(--text-secondary)] hover:border-[var(--brand)]"
                    >
                      <Pencil className="size-3.5" />
                      수정
                    </button>
                    <button
                      type="button"
                      onClick={onTogglePin}
                      className={cn(
                        "inline-flex items-center gap-1.5 rounded-lg border border-[var(--border)] px-3 py-1.5 text-xs font-medium hover:border-[var(--brand)]",
                        note.pinned ? "text-[var(--brand)]" : "text-[var(--text-secondary)]",
                      )}
                    >
                      <Pin className={cn("size-3.5", note.pinned && "fill-current")} />
                      {note.pinned ? "고정 해제" : "고정"}
                    </button>
                    <button
                      type="button"
                      onClick={onDelete}
                      aria-label="삭제"
                      className="rounded-lg p-1.5 text-[var(--text-muted)] hover:text-rose-400"
                    >
                      <Trash2 className="size-3.5" />
                    </button>
                  </>
                )}
                {note.symbol && (
                  <Link
                    href={analysisSymbolHref(note.symbol, "ai")}
                    className="ml-auto inline-flex items-center gap-1 rounded-lg bg-[var(--brand)] px-3 py-1.5 text-xs font-medium text-white hover:opacity-90"
                  >
                    종목 분석 보기
                    <ArrowRight className="size-3" />
                  </Link>
                )}
              </div>
            )}
          </>
        ) : (
          <div className="p-6 text-center text-sm text-[var(--text-muted)]">메모를 불러오는 중…</div>
        )}
      </DialogContent>
    </Dialog>
  );
}
