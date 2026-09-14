import { apiRequest } from "@/lib/api-client";
import type { JournalNote, JournalNoteInput } from "@/lib/journal-types";

function jsonBody(payload?: unknown): RequestInit {
  return {
    headers: { "Content-Type": "application/json" },
    body: payload ? JSON.stringify(payload) : undefined,
  };
}

/**
 * 내 메모 목록. `symbol`을 주면 그 종목에 연결된 메모만 받는다 — 필터는 서버가
 * 한다(핸들러가 이미 `?symbol=`을 읽는다). 전부 받아 와 화면에서 거르면, 메모가
 * 수백 건인 사용자가 종목 하나를 보려고 목록 전체를 내려받는다.
 */
export async function fetchJournalNotesClient(symbol?: string): Promise<JournalNote[]> {
  const sym = symbol?.trim();
  const path = sym
    ? `/api/v1/journal/notes?symbol=${encodeURIComponent(sym)}`
    : "/api/v1/journal/notes";
  const data = await apiRequest<JournalNote[] | null>(path);
  return data ?? [];
}

export async function createJournalNoteClient(input: JournalNoteInput): Promise<JournalNote> {
  return apiRequest<JournalNote>("/api/v1/journal/notes", { method: "POST", ...jsonBody(input) });
}

export async function updateJournalNoteClient(id: number, input: JournalNoteInput): Promise<JournalNote> {
  return apiRequest<JournalNote>(`/api/v1/journal/notes/${id}`, { method: "PUT", ...jsonBody(input) });
}

export async function deleteJournalNoteClient(id: number): Promise<void> {
  await apiRequest<{ id: number }>(`/api/v1/journal/notes/${id}`, { method: "DELETE" });
}
