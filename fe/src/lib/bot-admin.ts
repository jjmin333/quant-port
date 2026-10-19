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
  /** template — 템플릿 그대로 · llm — 배정한 게시 계정 말투로 AI가 다시 썼다. */
  source: "template" | "llm";
  reason: string;
  title: string;
  body: string;
  articles: { id: string; title: string }[];
  /** 초안을 만들 때 배정한 게시 계정(그 말투로 썼다). 게시 화면이 먼저 고른다. */
  personaId?: number;
  /** AI 다시 쓰기를 버린 이유(숫자 검증 등). */
  note?: string;
  status: BotDraftStatus;
  publishedRef?: number;
  publishedBy?: number;
  createdAt: string;
};

export type BotPersona = {
  id: number;
  displayName: string;
  /** 뱃지 키 — `fetchPersonaBadges` 목록 중에서. */
  badges: string[];
  /** 말투 설명 — AI가 이 계정 이름으로 쓸 때 따른다. */
  style: string;
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

export type PersonaInput = { displayName: string; badges: string[]; style: string };

export function createPersona(input: PersonaInput): Promise<{ id: number }> {
  return apiRequest<{ id: number }>("/api/v1/admin/personas", {
    method: "POST",
    headers: JSON_HEADERS,
    body: JSON.stringify(input),
  });
}

export function updatePersona(id: number, input: PersonaInput): Promise<unknown> {
  return apiRequest(`/api/v1/admin/personas/${id}`, {
    method: "PUT",
    headers: JSON_HEADERS,
    body: JSON.stringify(input),
  });
}

/** 서비스에 있는 뱃지 전부(BE `model.CommunityBadgeCatalog`) — 게시 계정에 고른다. */
export type PersonaBadgeOption = { key: string; emoji: string; label: string };

export function fetchPersonaBadges(): Promise<PersonaBadgeOption[]> {
  return apiRequest<PersonaBadgeOption[] | null>("/api/v1/admin/personas/badges").then((d) => d ?? []);
}

export type PersonaPost = {
  kind: BotDraftKind;
  id: number;
  title: string;
  personaName: string;
  views: number;
  createdAt: string;
};

export function fetchPersonaPosts(): Promise<PersonaPost[]> {
  return apiRequest<PersonaPost[] | null>("/api/v1/admin/personas/posts").then((d) => d ?? []);
}

/** 게시 계정 글의 조회수만 바뀐다(실제 사용자 글은 404). 이후 실제 조회는 이 값에서 더해진다. */
export function setPersonaPostViews(kind: BotDraftKind, id: number, views: number): Promise<unknown> {
  return apiRequest(`/api/v1/admin/personas/posts/${kind}/${id}/views`, {
    method: "PUT",
    headers: JSON_HEADERS,
    body: JSON.stringify({ views }),
  });
}

export function fetchPersonaActivity(): Promise<BotActivity[]> {
  return apiRequest<BotActivity[] | null>("/api/v1/admin/personas/activity").then((d) => d ?? []);
}
