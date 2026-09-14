"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
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
import { createJournalNoteClient, fetchJournalNotesClient } from "@/lib/journal-api";
import type { JournalNote, JournalNoteInput } from "@/lib/journal-types";
import type { EnrichedRankingItem } from "@/lib/market-radar-types";
import { parseRankingNumber } from "@/lib/market-radar-config";
import type { QuoteSummary } from "@/lib/quotes-types";
import { useModalAutoFocus } from "@/hooks/use-modal-autofocus";

/**
 * 접힌 상태에서 보여 줄 지난 메모 수.
 *
 * 셋은 「최근에 뭐라고 썼더라」를 떠올리기에 충분하고, 그 이상은 이 창의 일이
 * 아니다 — 다 훑어보는 것은 메모장(`/journal`)이 한다.
 */
const MEMO_PREVIEW_COUNT = 3;

/** '26.09.03 — 목록이 좁아 연·월·일만. */
function shortDate(iso: string): string {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "";
  return d.toLocaleDateString("ko-KR", {
    timeZone: "Asia/Seoul",
    year: "2-digit",
    month: "2-digit",
    day: "2-digit",
  });
}

/**
 * 이미 쓴 메모 한 건.
 *
 * 목록 화면의 카드를 그대로 가져오지 않고 줄여서 그린다 — 여기서 할 일은 「전에
 * 뭐라고 썼더라」를 떠올리는 것이지 편집이 아니다. 고치는 것은 메모장에서 한다.
 */
function ExistingNote({ note, quote }: { note: JournalNote; quote?: QuoteSummary }) {
  return (
    <li className="rounded-lg border border-[var(--border)] bg-[var(--surface-elevated)]/40 px-2.5 py-2">
      <div className="flex items-baseline justify-between gap-2">
        <p className="min-w-0 flex-1 truncate text-cell font-medium text-[var(--text-primary)]">
          {note.title || "제목 없음"}
        </p>
        <span className="shrink-0 font-num text-micro tabular-nums text-[var(--text-muted)]">
          {shortDate(note.createdAt)}
        </span>
      </div>
      <p className="mt-1 line-clamp-3 whitespace-pre-wrap text-cell leading-relaxed text-[var(--text-secondary)]">
        {note.body}
      </p>
      <JournalTagChips tags={note.tags} className="mt-1.5" />
      <JournalPriceStrip note={note} quote={quote} compact className="mt-1.5" />
    </li>
  );
}

/**
 * 랭킹 행의 메모 아이콘이 여는 창.
 *
 * **폼은 메모장 화면의 것을 그대로 쓴다.** 같은 일을 하는 입력이 화면마다 다르면
 * 태그 프리셋 같은 것이 여기만 빠지고, 그러면 사용자는 같은 기능을 두 번 배워야 한다.
 *
 * 종목은 이미 정해져 있으므로 폼에 채워서 연다 — 랭킹에서 누른 그 종목이다.
 *
 * **이미 쓴 메모를 먼저 보여 준다.** 예전에는 빈 폼만 떴다. 그러면 같은 종목에
 * 지난주에 뭐라고 썼는지 확인하려면 메모장으로 나갔다 돌아와야 했고, 그러다
 * 보던 랭킹 자리를 잃는다. 「그때와 비교한다」는 것이 이 기능의 요점인데 정작
 * 그때 쓴 것이 화면에 없었던 셈이다.
 */
/*
메모 창이 실제로 필요한 것 — **종목 하나와 그 행의 자리**뿐이다.

예전에는 `EnrichedRankingItem`(실시간 차트 행)을 통째로 받았다. 그래서 AI 추천 랭킹의
행은 같은 창을 열 수 없었다 — 그쪽 행은 가격을 문자열이 아니라 숫자로 들고 있고
`price.basePrice` 같은 시세 전용 칸이 없다. 창이 쓰는 여섯 칸만 요구하면 두 목록이
같은 창을 쓴다.
*/
export type RankingMemoTarget = {
  symbol: string;
  name: string;
  /**
   * 목록에서 열었을 때의 순위. **없을 수 있다** — 종목 퀵룩 모달에서 열면 「몇 위」가
   * 없는 자리이고(2026-09-05 사용자 요청 7-4), 없는 값을 0으로 채우면 부제가
   * 「종목 0위」가 된다.
   */
  rank?: number;
  currency?: "KRW" | "USD";
  assetType?: EnrichedRankingItem["assetType"];
  logoUrl?: string;
  /** 「작성 당시 → 현재」 줄에 쓸 현재가. 0이면 그 줄을 그리지 않는다. */
  price: number;
  /** 부제의 출처 이름 — 「실시간 차트 3위」 / 「AI 추천 랭킹 3위」. */
  originLabel: string;
};

/** 실시간 차트 행 → 메모 창이 쓰는 최소 모양. */
export function memoTargetFromRankingItem(item: EnrichedRankingItem): RankingMemoTarget {
  return {
    symbol: item.symbol,
    name: item.name,
    rank: item.rank,
    currency: item.currency,
    assetType: item.assetType,
    logoUrl: item.logoUrl,
    price: parseRankingNumber(item.price.lastPrice),
    originLabel: "실시간 차트",
  };
}

export function RankingMemoModal({
  item,
  onClose,
}: {
  item: RankingMemoTarget | null;
  onClose: () => void;
}) {
  /* 폰·태블릿에서는 모달이 열려도 자판을 올리지 않는다 — lib/modal-autofocus.ts */
  const canAutoFocus = useModalAutoFocus();
  const [saved, setSaved] = useState(false);
  const [notes, setNotes] = useState<JournalNote[]>([]);
  const [notesLoading, setNotesLoading] = useState(false);
  const [showAllNotes, setShowAllNotes] = useState(false);

  // 열림/닫힘에 딸린 초기화는 이펙트가 아니라 렌더 중 조정 — 이펙트로 하면
  // 저장 완료 문구가 남은 프레임이 한 번 더 그려진다(메모 상세 모달과 같은 방식).
  const [lastSymbol, setLastSymbol] = useState<string | null>(null);
  const symbol = item?.symbol ?? null;
  if (lastSymbol !== symbol) {
    setLastSymbol(symbol);
    if (saved) setSaved(false);
    if (notes.length > 0) setNotes([]);
    // 종목이 바뀌면 펼침도 되돌린다 — 앞 종목에서 펼쳐 둔 채로 스무 건짜리
    // 다음 종목을 열면 창이 통째로 길어진 상태로 뜬다.
    if (showAllNotes) setShowAllNotes(false);
  }

  const loadNotes = useCallback(async (sym: string) => {
    setNotesLoading(true);
    try {
      setNotes(await fetchJournalNotesClient(sym));
    } catch {
      // 기존 메모를 못 불러온 것으로 **작성을 막지는 않는다.** 이 창의 본래 일은
      // 새 메모를 쓰는 것이고, 지난 메모는 거들 뿐이다.
      setNotes([]);
    } finally {
      setNotesLoading(false);
    }
  }, []);

  useEffect(() => {
    if (!symbol) return;
    void loadNotes(symbol);
  }, [symbol, loadNotes]);

  async function handleSubmit(input: JournalNoteInput) {
    await createJournalNoteClient(input);
    setSaved(true);
    if (symbol) void loadNotes(symbol);
  }

  /*
    「작성 당시 → 현재」 줄에 쓸 현재가. 랭킹 행이 이미 들고 있는 값을 쓴다 —
    이 창을 열자고 시세를 한 번 더 부를 이유가 없다.
  */
  const livePrice = item?.price ?? 0;
  const quote =
    item && livePrice > 0
      ? ({
          symbol: item.symbol,
          name: item.name,
          currentPrice: livePrice,
          currency: item.currency,
        } as unknown as QuoteSummary)
      : undefined;

  return (
    <Dialog open={Boolean(item)} onOpenChange={(next) => !next && onClose()}>
      <DialogContent className="flex max-h-[88dvh] flex-col gap-0 overflow-hidden p-0 sm:max-w-lg">
        {item && (
          <>
            <DialogHeader className="shrink-0 border-b px-4 py-3 pr-10 text-left">
              <DialogTitle className="flex min-w-0 items-center gap-2 text-sm">
                <StockAvatar
                  symbol={item.symbol}
                  name={item.name}
                  currency={item.currency}
                  assetType={item.assetType}
                  logoUrl={item.logoUrl}
                  size="sm"
                  className="shrink-0"
                />
                <span className="min-w-0 flex-1 truncate">{item.name} 메모</span>
              </DialogTitle>
              <DialogDescription className="text-cell">
                {item.symbol} · {item.originLabel}
                {item.rank != null ? ` ${item.rank}위` : ""}
              </DialogDescription>
            </DialogHeader>

            <div className="min-h-0 flex-1 space-y-3 overflow-y-auto px-4 py-3">
              {/*
                # 쓰는 자리가 먼저다 (2026-09-03 사용자 결정)

                예전에는 지난 메모를 전부 위에 깔고 그 아래에 폼을 뒀다. 메모가 쌓인
                종목에서는 **정작 쓰려는 칸이 스크롤 한참 아래**로 내려가, 이 창을 연
                이유(지금 떠오른 것을 바로 적는다)와 순서가 반대였다. 지난 메모는
                「그때 뭐라고 썼더라」를 거드는 재료라 아래에서 받는다.
              */}
              {saved ? (
                <div className="space-y-3 py-6 text-center">
                  <p className="text-sm text-[var(--text-primary)]">메모를 저장했습니다.</p>
                  <button
                    type="button"
                    onClick={() => setSaved(false)}
                    className="text-cell text-[var(--text-secondary)] underline"
                  >
                    이 종목에 메모 더 쓰기
                  </button>
                </div>
              ) : (
                <JournalNoteForm
                  compact
                  /* 폰·태블릿에서는 자판을 올리지 않는다 — lib/modal-autofocus.ts */
                  autoFocus={canAutoFocus}
                  initialStock={{ symbol: item.symbol, name: item.name }}
                  onCancel={onClose}
                  onSubmit={handleSubmit}
                />
              )}

              {notesLoading && notes.length === 0 ? (
                <p className="text-cell text-[var(--text-muted)]">기존 메모 확인 중…</p>
              ) : notes.length > 0 ? (
                <section className="border-t border-[var(--border)] pt-3">
                  <div className="mb-1.5 flex items-baseline justify-between gap-2">
                    <h3 className="flex items-baseline gap-1.5 text-cell font-semibold text-[var(--text-primary)]">
                      이 종목에 쓴 메모 {notes.length}건
                      {/*
                        「전체 보기 +N」은 제목 오른쪽 — 홈 카드의 접힘과 같은 규약이다
                        (`docs/UI_SPEC.md`). 최근 셋만 두는 이유는 스크롤 길이다: 메모가
                        스무 건인 종목에서 이 창이 화면 몇 배로 길어졌다.
                      */}
                      {notes.length > MEMO_PREVIEW_COUNT && (
                        <button
                          type="button"
                          onClick={() => setShowAllNotes((v) => !v)}
                          className="shrink-0 whitespace-nowrap text-cell font-medium text-[var(--brand)] hover:underline"
                        >
                          {showAllNotes ? "접기" : `전체 보기 +${notes.length - MEMO_PREVIEW_COUNT}`}
                        </button>
                      )}
                    </h3>
                    {/* 고치고 지우는 것은 메모장에서 — 이 창은 읽기와 새로 쓰기만 한다. */}
                    <Link
                      href="/journal"
                      className="shrink-0 text-micro text-[var(--brand)] hover:underline"
                    >
                      메모장에서 보기 →
                    </Link>
                  </div>
                  <ul className="space-y-1.5">
                    {(showAllNotes ? notes : notes.slice(0, MEMO_PREVIEW_COUNT)).map((note) => (
                      <ExistingNote key={note.id} note={note} quote={quote} />
                    ))}
                  </ul>
                </section>
              ) : null}
            </div>
          </>
        )}
      </DialogContent>
    </Dialog>
  );
}
