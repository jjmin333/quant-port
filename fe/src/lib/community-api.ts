import type { ContentTopic } from "@/lib/topics";
import { apiRequest } from "@/lib/api-client";
import { asPersona } from "@/lib/bot-admin";
import type {
  CommunityComment,
  CommunityCreateInput,
  CommunityImage,
  CommunityPost,
  CommunityPostPage,
  CommunitySort,
} from "@/lib/community-types";

/** 커뮤니티 API — BE `handler/community.go`. 읽기는 비로그인도, 쓰기는 로그인. */

const JSON_HEADERS = { "Content-Type": "application/json" };

export function fetchCommunityPosts(params: {
  sort?: CommunitySort;
  symbol?: string;
  topic?: ContentTopic | "";
  mine?: boolean;
  cursor?: string;
  limit?: number;
}): Promise<CommunityPostPage> {
  const q = new URLSearchParams();
  if (params.sort && params.sort !== "latest") q.set("sort", params.sort);
  if (params.symbol) q.set("symbol", params.symbol);
  if (params.topic) q.set("topic", params.topic);
  if (params.mine) q.set("mine", "1");
  if (params.cursor) q.set("cursor", params.cursor);
  if (params.limit) q.set("limit", String(params.limit));
  const qs = q.toString();
  return apiRequest<CommunityPostPage>(`/api/v1/community/posts${qs ? `?${qs}` : ""}`).then((p) => ({
    ...p,
    items: p.items ?? [],
  }));
}

export function fetchCommunityPost(id: number): Promise<CommunityPost> {
  return apiRequest<CommunityPost>(`/api/v1/community/posts/${id}`);
}

/** personaId — 관리자가 게시 계정으로 올릴 때(`lib/bot-admin.ts`). */
export function createCommunityPost(input: CommunityCreateInput, personaId?: number): Promise<CommunityPost> {
  return apiRequest<CommunityPost>(asPersona("/api/v1/community/posts", personaId), {
    method: "POST",
    headers: JSON_HEADERS,
    body: JSON.stringify(input),
  });
}

export function updateCommunityPost(id: number, title: string, body: string, topic?: ContentTopic): Promise<CommunityPost> {
  return apiRequest<CommunityPost>(`/api/v1/community/posts/${id}`, {
    method: "PATCH",
    headers: JSON_HEADERS,
    body: JSON.stringify({ title, body, topic }),
  });
}

export function deleteCommunityPost(id: number): Promise<{ ok: boolean }> {
  return apiRequest(`/api/v1/community/posts/${id}`, { method: "DELETE" });
}

export function toggleCommunityPostLike(id: number): Promise<{ liked: boolean; likeCount: number }> {
  return apiRequest(`/api/v1/community/posts/${id}/like`, { method: "POST" });
}

export function toggleCommunityCommentLike(id: number): Promise<{ liked: boolean; likeCount: number }> {
  return apiRequest(`/api/v1/community/comments/${id}/like`, { method: "POST" });
}

/** 조회 기록 — 실패해도 화면에는 아무 일도 없다. */
export function recordCommunityView(id: number): Promise<void> {
  return apiRequest(`/api/v1/community/posts/${id}/view`, { method: "POST" }).then(
    () => undefined,
    () => undefined,
  );
}

export function fetchCommunityComments(id: number): Promise<CommunityComment[]> {
  return apiRequest<CommunityComment[] | null>(`/api/v1/community/posts/${id}/comments`).then(
    (c) => c ?? [],
  );
}

export function addCommunityComment(
  id: number,
  body: string,
  parentId?: number,
  personaId?: number,
): Promise<CommunityComment> {
  return apiRequest<CommunityComment>(asPersona(`/api/v1/community/posts/${id}/comments`, personaId), {
    method: "POST",
    headers: JSON_HEADERS,
    body: JSON.stringify({ body, parentId }),
  });
}

export function deleteCommunityComment(id: number): Promise<{ ok: boolean }> {
  return apiRequest(`/api/v1/community/comments/${id}`, { method: "DELETE" });
}

export function reportCommunity(
  targetType: "post" | "comment",
  targetId: number,
  reason: string,
): Promise<{ ok: boolean }> {
  return apiRequest("/api/v1/community/reports", {
    method: "POST",
    headers: JSON_HEADERS,
    body: JSON.stringify({ targetType, targetId, reason }),
  });
}

/** 이미지 업로드 — 서버가 다시 구워 ≤1MB(평균 100~200KB)로 저장한다. */
export function uploadCommunityImage(
  file: Blob,
  filename = "image.jpg",
): Promise<CommunityImage & { size: number }> {
  const form = new FormData();
  form.append("file", file, filename);
  return apiRequest<CommunityImage & { size: number }>("/api/v1/community/images", {
    method: "POST",
    body: form,
  });
}

// ── 관리자 ──

export type CommunityModeration = { posts: CommunityPost[]; comments: CommunityComment[] };

export function fetchCommunityModeration(): Promise<CommunityModeration> {
  return apiRequest<CommunityModeration>("/api/v1/admin/community/moderation");
}

export function setCommunityStatus(
  target: "post" | "comment",
  id: number,
  status: "visible" | "hidden",
): Promise<{ ok: boolean }> {
  return apiRequest(`/api/v1/admin/community/${target}/${id}/status`, {
    method: "PUT",
    headers: JSON_HEADERS,
    body: JSON.stringify({ status }),
  });
}
