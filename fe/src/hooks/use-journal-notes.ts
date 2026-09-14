"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { isSampleModeActive } from "@/components/common/sample-mode";
import {
  createJournalNoteClient,
  deleteJournalNoteClient,
  fetchJournalNotesClient,
  updateJournalNoteClient,
} from "@/lib/journal-api";
import { fetchQuotesBatchClient } from "@/lib/market-api";
import { fetchSampleJournal } from "@/lib/sample-api";
import type { JournalNote, JournalNoteInput } from "@/lib/journal-types";
import type { QuoteSummary } from "@/lib/quotes-types";

export type JournalSort = "updated" | "created";

function sortNotes(notes: JournalNote[], sort: JournalSort): JournalNote[] {
  const key = sort === "updated" ? "updatedAt" : "createdAt";
  return [...notes].sort((a, b) => {
    if (a.pinned !== b.pinned) return a.pinned ? -1 : 1;
    return new Date(b[key]).getTime() - new Date(a[key]).getTime();
  });
}

export function useJournalNotes({ sample = false }: { sample?: boolean } = {}) {
  const [notes, setNotes] = useState<JournalNote[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [query, setQuery] = useState("");
  const [symbolFilter, setSymbolFilter] = useState<string | null>(null);
  const [tagFilter, setTagFilter] = useState<string | null>(null);
  const [sort, setSort] = useState<JournalSort>("updated");
  const [quotes, setQuotes] = useState<Record<string, QuoteSummary>>({});

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    const isSample = isSampleModeActive(sample);
    try {
      if (isSample) {
        // 예시 모드는 공개 픽스처를 읽는다 — 로그인 여부와 무관하게 같은 화면이 뜬다.
        // 현재가도 픽스처가 들고 온다: 실시간 시세를 섞으면 "샀던 근거가 맞았다"고
        // 쓰인 메모의 등락률이 방문할 때마다 뒤집힌다.
        const rows = await fetchSampleJournal();
        setNotes(rows);
        setQuotes(
          Object.fromEntries(
            rows
              .filter((r) => r.symbol && r.price && r.price > 0)
              .map((r) => [
                r.symbol as string,
                {
                  symbol: r.symbol,
                  name: r.symbolName,
                  currentPrice: r.price,
                  currency: r.priceCurrency ?? "KRW",
                } as unknown as QuoteSummary,
              ]),
          ),
        );
      } else {
        setNotes(await fetchJournalNotesClient());
      }
    } catch {
      setError("메모를 불러오지 못했습니다.");
    } finally {
      setLoading(false);
    }
  }, [sample]);

  useEffect(() => {
    load();
  }, [load]);

  const symbols = useMemo(() => {
    const map = new Map<string, string>();
    for (const note of notes) {
      if (note.symbol) map.set(note.symbol, note.symbolName || note.symbol);
    }
    return Array.from(map.entries()).map(([symbol, name]) => ({ symbol, name }));
  }, [notes]);

  const symbolKey = useMemo(
    () => symbols.map((s) => s.symbol).sort().join(","),
    [symbols],
  );

  /*
   * 작성 당시 가격만으로는 아무 말도 못 한다 — "184,500원이었다"는 지금 얼마인지를
   * 알아야 의미가 생긴다. 연결된 종목의 현재가를 한 번에 받아 목록·모달이 함께 쓴다.
   * 실패해도 메모는 그대로 뜬다: 시세는 덧붙는 정보이지 메모의 전제가 아니다.
   *
   * 예시에서도 부른다. 예전에는 픽스처의 작성 당시 가격과 실시간 시세가 다른 세계라
   * 막아뒀지만, 지금은 작성 당시 가격이 **그 날짜의 실제 종가**다.
   */
  useEffect(() => {
    if (!symbolKey) return;
    let alive = true;
    fetchQuotesBatchClient(symbolKey.split(","))
      .then((rows) => {
        if (alive) setQuotes(Object.fromEntries(rows.map((q) => [q.symbol, q])));
      })
      .catch(() => {
        // 시세 없이도 메모 목록은 유효하다
      });
    return () => {
      alive = false;
    };
  }, [symbolKey]);

  const tags = useMemo(() => {
    const counts = new Map<string, number>();
    for (const note of notes) {
      for (const tag of note.tags ?? []) counts.set(tag, (counts.get(tag) ?? 0) + 1);
    }
    return Array.from(counts.entries())
      .map(([tag, count]) => ({ tag, count }))
      .sort((a, b) => b.count - a.count || a.tag.localeCompare(b.tag));
  }, [notes]);

  const visibleNotes = useMemo(() => {
    const q = query.trim().toLowerCase();
    const filtered = notes.filter((note) => {
      if (symbolFilter && note.symbol !== symbolFilter) return false;
      if (tagFilter && !(note.tags ?? []).includes(tagFilter)) return false;
      if (!q) return true;
      return (
        note.title.toLowerCase().includes(q) ||
        note.body.toLowerCase().includes(q) ||
        (note.tags ?? []).some((t) => t.toLowerCase().includes(q))
      );
    });
    return sortNotes(filtered, sort);
  }, [notes, query, symbolFilter, tagFilter, sort]);

  const createNote = useCallback(async (input: JournalNoteInput) => {
    const note = await createJournalNoteClient(input);
    setNotes((prev) => [note, ...prev]);
    return note;
  }, []);

  const updateNote = useCallback(async (id: number, input: JournalNoteInput) => {
    const note = await updateJournalNoteClient(id, input);
    setNotes((prev) => prev.map((n) => (n.id === id ? note : n)));
    return note;
  }, []);

  const togglePin = useCallback(
    async (note: JournalNote) => {
      await updateNote(note.id, {
        title: note.title,
        body: note.body,
        symbol: note.symbol,
        symbolName: note.symbolName,
        tags: note.tags,
        pinned: !note.pinned,
      });
    },
    [updateNote],
  );

  const removeNote = useCallback(async (id: number) => {
    await deleteJournalNoteClient(id);
    setNotes((prev) => prev.filter((n) => n.id !== id));
  }, []);

  return {
    notes: visibleNotes,
    /** 필터 전 원본. 우측 레일의 집계(고정 메모·통계)가 쓴다 — 요청을 늘리지 않는다. */
    allNotes: notes,
    totalCount: notes.length,
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
    /** 연결된 종목의 현재 시세. 작성 당시 가격과 짝을 이룰 때만 의미가 있다. */
    quotes,
    createNote,
    updateNote,
    togglePin,
    removeNote,
  };
}
