import { apiRequest } from "@/lib/api-client";

/** 앱 안 알림 한 줄 (2026-10-10 요청 4 — 웹 푸시 아님). kind: 관심 종목 변화 · 관심 단지 새 실거래 · 관심 상품 금리 변동. */
export type AppNotification = {
  id: number;
  kind: "stock" | "re_trade" | "fin_rate";
  title: string;
  body: string;
  href: string;
  createdAt: string;
  read: boolean;
};

export function fetchNotifications(limit = 30): Promise<{ items: AppNotification[]; unread: number }> {
  return apiRequest<{ items: AppNotification[]; unread: number }>(`/api/v1/notifications?limit=${limit}`).then((d) => ({
    items: d?.items ?? [],
    unread: d?.unread ?? 0,
  }));
}

export function fetchUnreadNotifications(): Promise<number> {
  return apiRequest<{ unread: number }>("/api/v1/notifications/unread").then((d) => d?.unread ?? 0);
}

/** ids가 없으면 전부 읽음. */
export function markNotificationsRead(ids?: number[]): Promise<unknown> {
  return apiRequest("/api/v1/notifications/read", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ ids: ids ?? [] }),
  });
}

/** 단지·지역·상품·금리를 태그한 공개 글(커뮤니티 + 리포트). */
export type AssetPost = { type: "community" | "research"; id: number; title: string; authorName: string; createdAt: string };

export function fetchAssetPosts(kind: string, key: string): Promise<AssetPost[]> {
  const q = new URLSearchParams({ kind, key });
  return apiRequest<AssetPost[] | null>(`/api/v1/assets/posts?${q}`).then((d) => d ?? []);
}
