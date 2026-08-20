import { apiRequest } from "@/lib/api-client";
import type { JournalNote, JournalNoteInput } from "@/lib/journal-types";

function jsonBody(payload?: unknown): RequestInit {
  return {
    headers: { "Content-Type": "application/json" },
    body: payload ? JSON.stringify(payload) : undefined,
  };
}

export async function fetchJournalNotesClient(): Promise<JournalNote[]> {
  const data = await apiRequest<JournalNote[] | null>("/api/v1/journal/notes");
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
