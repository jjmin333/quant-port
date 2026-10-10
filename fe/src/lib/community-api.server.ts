import { getApiBaseUrl } from "@/lib/api-client";
import type { ApiResponse } from "@/lib/api-types";
import type { CommunityPost, CommunityPostPage } from "@/lib/community-types";
import { fetchWithDeadline } from "@/lib/fetch-timeout";

/**
 * 커뮤니티 첫 화면을 **서버에서** 받아 HTML에 싣는다 — 검색 색인 허용(2026-09-27 결정).
 * 네이버 Yeti는 사실상 HTML에 있는 것만 본다(`research-api.server.ts` 머리글과 같은 이유).
 *
 * 쿠키를 보내지 않는다 — 비로그인 방문자가 보는 화면 그대로라 캐시를 모두가 나눠 써도
 * 안전하다. 좋아요 여부·내 글 표시는 클라이언트가 마운트 뒤 자기 세션으로 다시 받아 덮는다.
 */

const LIST_REVALIDATE = 60;
const DETAIL_REVALIDATE = 120;

async function publicGet<T>(path: string, revalidate: number): Promise<T | null> {
  try {
    const res = await fetchWithDeadline(`${getApiBaseUrl()}${path}`, { next: { revalidate } });
    if (!res.ok) return null;
    const body = (await res.json()) as ApiResponse<T>;
    return body.data ?? null;
  } catch {
    return null;
  }
}

export async function fetchCommunityPostsServer(symbol?: string): Promise<CommunityPostPage | null> {
  const qs = symbol ? `?symbol=${encodeURIComponent(symbol)}` : "";
  return publicGet<CommunityPostPage>(`/api/v1/community/posts${qs}`, LIST_REVALIDATE);
}

export async function fetchCommunityPostServer(id: number): Promise<CommunityPost | null> {
  return publicGet<CommunityPost>(`/api/v1/community/posts/${id}`, DETAIL_REVALIDATE);
}

export async function fetchCommunitySitemapServer(): Promise<{ id: number; updatedAt: string }[]> {
  return (
    (await publicGet<{ id: number; updatedAt: string }[]>(`/api/v1/community/sitemap`, 600)) ?? []
  );
}
