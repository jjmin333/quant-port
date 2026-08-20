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
