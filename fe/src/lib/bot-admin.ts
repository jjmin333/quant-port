import { apiRequest } from "@/lib/api-client";

/**
 * 봇 초안 · 게시 계정 — BE `handler/admin_bots.go` (2026-10-16 사용자 요청).
 *
 * 초안은 worker-stock이 3시간마다 만든다. 관리자가 고쳐서 게시 계정으로 올린다 — 올리기는 일반 글쓰기 API를
 * `/api/v1/admin/personas/{id}/as/...` 아래로 보낸다(`asPersona`). 검증·쿨다운·오류 응답이 일반 글과 같다.
 */

export type BotDraftKind = "community" | "research";
export type BotDraftStatus = "draft" | "published" | "discarded";

export type BotDraft = {
  id: number;
  kind: BotDraftKind;
  slotAt: string;
  symbol: string;
  symbolName: string;
  market: string;
  length: "short" | "long";
  reason: string;
  title: string;
  body: string;
  articles: { id: string; title: string }[];
  status: BotDraftStatus;
  publishedRef?: number;
  publishedBy?: number;
  createdAt: string;
};

export type BotPersona = {
  id: number;
  displayName: string;
  badge: string;
  posts: number;
  calls: number;
  createdAt: string;
};

export type BotActivity = {
  kind: BotDraftKind;
  commentId: number;
  targetId: number;
  targetTitle: string;
  personaId: number;
  personaName: string;
  authorName: string;
  body: string;
  createdAt: string;
};

const JSON_HEADERS = { "Content-Type": "application/json" };

/** 일반 쓰기 경로(`/api/v1/...`)를 게시 계정 경로로 바꾼다. personaId가 없으면 그대로. */
export function asPersona(path: string, personaId?: number | null): string {
  return personaId ? path.replace(/^\/api\/v1\//, `/api/v1/admin/personas/${personaId}/as/`) : path;
}

export function fetchBotDrafts(kind: BotDraftKind, status: BotDraftStatus | "" = "draft"): Promise<BotDraft[]> {
  const q = new URLSearchParams({ kind });
  if (status) q.set("status", status);
  return apiRequest<BotDraft[] | null>(`/api/v1/admin/bot-drafts?${q}`).then((d) => d ?? []);
}

export function fetchBotDraft(id: number): Promise<BotDraft> {
  return apiRequest<BotDraft>(`/api/v1/admin/bot-drafts/${id}`);
}

export function discardBotDraft(id: number): Promise<unknown> {
  return apiRequest(`/api/v1/admin/bot-drafts/${id}/discard`, { method: "POST" });
}

export function markBotDraftPublished(id: number, ref: number, personaId: number): Promise<unknown> {
  return apiRequest(`/api/v1/admin/bot-drafts/${id}/published`, {
    method: "POST",
    headers: JSON_HEADERS,
    body: JSON.stringify({ ref, personaId }),
  });
}

export function fetchPersonas(): Promise<BotPersona[]> {
  return apiRequest<BotPersona[] | null>("/api/v1/admin/personas").then((d) => d ?? []);
}

export function createPersona(displayName: string, badge: string): Promise<{ id: number }> {
  return apiRequest<{ id: number }>("/api/v1/admin/personas", {
    method: "POST",
    headers: JSON_HEADERS,
    body: JSON.stringify({ displayName, badge }),
  });
}

export function updatePersona(id: number, displayName: string, badge: string): Promise<unknown> {
  return apiRequest(`/api/v1/admin/personas/${id}`, {
    method: "PUT",
    headers: JSON_HEADERS,
    body: JSON.stringify({ displayName, badge }),
  });
}

export function fetchPersonaActivity(): Promise<BotActivity[]> {
  return apiRequest<BotActivity[] | null>("/api/v1/admin/personas/activity").then((d) => d ?? []);
}
