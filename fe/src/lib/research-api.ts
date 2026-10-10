import type { ContentTopic } from "@/lib/topics";
import { apiRequest } from "@/lib/api-client";
import type {
  ResearchAuthorProfile,
  ResearchCall,
  ResearchComment,
  ResearchPreview,
  ResearchPreviewInput,
  ResearchPublishInput,
  ResearchRevision,
  ResearchSummary,
  ResearchSymbolSummary,
  ResearchUpdateInput,
} from "@/lib/research-types";

function jsonBody(payload?: unknown): RequestInit {
  return {
    headers: { "Content-Type": "application/json" },
    body: payload ? JSON.stringify(payload) : undefined,
  };
}

export type ResearchListParams = {
  topic?: ContentTopic | "";
  symbol?: string;
  status?: string;
  direction?: string;
  contrarian?: string;
  limit?: number;
  offset?: number;
};

function queryOf(params: Record<string, unknown>): string {
  const query = new URLSearchParams();
  for (const [key, value] of Object.entries(params)) {
    if (value !== undefined && value !== null && value !== "") query.set(key, String(value));
  }
  return query.toString() ? `?${query}` : "";
}

/** Public feed. Readable without an account. */
export async function fetchResearchCallsClient(params: ResearchListParams = {}): Promise<ResearchCall[]> {
  const data = await apiRequest<ResearchCall[] | null>(`/api/v1/research/calls${queryOf(params)}`);
  return data ?? [];
}

/** The caller's own calls — retracted and hidden ones included. */
export async function fetchMyResearchCallsClient(params: ResearchListParams = {}): Promise<ResearchCall[]> {
  const data = await apiRequest<ResearchCall[] | null>(`/api/v1/research/mine${queryOf(params)}`);
  return data ?? [];
}

export async function fetchResearchCallClient(id: number): Promise<ResearchCall> {
  return apiRequest<ResearchCall>(`/api/v1/research/calls/${id}`);
}

export async function fetchResearchRevisionsClient(id: number): Promise<ResearchRevision[]> {
  const data = await apiRequest<ResearchRevision[] | null>(`/api/v1/research/calls/${id}/revisions`);
  return data ?? [];
}

export async function fetchResearchSummaryClient(): Promise<ResearchSummary> {
  return apiRequest<ResearchSummary>("/api/v1/research/summary");
}

export async function fetchResearchSymbolSummaryClient(symbol: string): Promise<ResearchSymbolSummary> {
  return apiRequest<ResearchSymbolSummary>(
    `/api/v1/research/symbols/${encodeURIComponent(symbol)}/summary`,
  );
}

/**
 * Preview assembles the evidence card, the auto-generated draft and the target
 * odds without storing anything — the compose screen calls it on every change
 * to the call values.
 */
export async function previewResearchClient(input: ResearchPreviewInput): Promise<ResearchPreview> {
  return apiRequest<ResearchPreview>("/api/v1/research/preview", { method: "POST", ...jsonBody(input) });
}

export async function publishResearchClient(input: ResearchPublishInput): Promise<ResearchCall> {
  return apiRequest<ResearchCall>("/api/v1/research/calls", { method: "POST", ...jsonBody(input) });
}

export async function updateResearchClient(id: number, input: ResearchUpdateInput): Promise<ResearchCall> {
  return apiRequest<ResearchCall>(`/api/v1/research/calls/${id}`, { method: "PATCH", ...jsonBody(input) });
}

/**
 * Retract, not delete. There is no delete endpoint on purpose — a removable
 * call would make every hit rate on the service unfalsifiable.
 */
export async function retractResearchClient(id: number): Promise<ResearchCall> {
  return apiRequest<ResearchCall>(`/api/v1/research/calls/${id}/retract`, { method: "POST" });
}

/** 완전 삭제 — 작성자·관리자. 성적 집계에서도 빠진다. */
export async function deleteResearchClient(id: number): Promise<{ ok: boolean }> {
  return apiRequest<{ ok: boolean }>(`/api/v1/research/calls/${id}`, { method: "DELETE" });
}

/** Toggle a like/bookmark. Returns whether it is now on. */
export async function toggleResearchReactionClient(
  id: number,
  kind: "like" | "bookmark",
): Promise<boolean> {
  const data = await apiRequest<{ id: number; kind: string; active: boolean }>(
    `/api/v1/research/calls/${id}/reactions`,
    { method: "POST", ...jsonBody({ kind }) },
  );
  return data.active;
}

export async function fetchResearchCommentsClient(id: number): Promise<ResearchComment[]> {
  const data = await apiRequest<ResearchComment[] | null>(`/api/v1/research/calls/${id}/comments`);
  return data ?? [];
}

export async function addResearchCommentClient(
  id: number,
  body: string,
  parentId?: number,
): Promise<ResearchComment> {
  return apiRequest<ResearchComment>(`/api/v1/research/calls/${id}/comments`, {
    method: "POST",
    ...jsonBody({ body, parentId: parentId ?? null }),
  });
}

export async function updateResearchCommentClient(
  commentId: number,
  body: string,
): Promise<ResearchComment> {
  return apiRequest<ResearchComment>(`/api/v1/research/comments/${commentId}`, {
    method: "PUT",
    ...jsonBody({ body }),
  });
}

export async function deleteResearchCommentClient(commentId: number): Promise<void> {
  await apiRequest<{ id: number }>(`/api/v1/research/comments/${commentId}`, { method: "DELETE" });
}

/**
 * 댓글 추천을 켜고 끈다. 돌려주는 값은 **누른 뒤의 상태**다 (2026-09-14 요청 3).
 *
 * 글 추천(`toggleResearchReactionClient`)과 같은 모양이라 화면이 같은 방식으로
 * 쓴다 — 낙관적으로 먼저 그리고 이 값으로 바로잡는다.
 */
export async function toggleResearchCommentLikeClient(commentId: number): Promise<boolean> {
  const data = await apiRequest<{ id: number; active: boolean }>(
    `/api/v1/research/comments/${commentId}/like`,
    { method: "POST" },
  );
  return data.active;
}

/** Rate the *argument*. Refused by the server once the verdict is in. */
export async function rateResearchClient(
  id: number,
  rating: { clarity: number; evidence: number; risk: number },
): Promise<void> {
  await apiRequest<{ id: number }>(`/api/v1/research/calls/${id}/ratings`, {
    method: "POST",
    ...jsonBody(rating),
  });
}

/**
 * Views are recorded explicitly rather than as a side effect of the detail
 * fetch — a counter that moved with every re-render would count renders.
 */
export async function recordResearchViewClient(id: number): Promise<void> {
  await apiRequest<{ id: number }>(`/api/v1/research/calls/${id}/view`, { method: "POST" });
}

export async function fetchResearchAuthorClient(userId: number): Promise<ResearchAuthorProfile> {
  return apiRequest<ResearchAuthorProfile>(`/api/v1/research/authors/${userId}`);
}

export async function reportResearchClient(
  id: number,
  reason: string,
  detail = "",
): Promise<void> {
  await apiRequest<{ id: number }>(`/api/v1/research/calls/${id}/report`, {
    method: "POST",
    ...jsonBody({ reason, detail }),
  });
}
