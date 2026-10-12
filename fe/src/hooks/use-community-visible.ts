"use client";

import { useAuth } from "@/components/providers/auth-provider";
import { isAdminSession } from "@/lib/admin-features";

/**
 * 커뮤니티·투자 리포트 입구(종목 화면 탭·카드, 기사 공유 링크 등)를 그릴지 — 2026-10-12부터
 * 관리자에게만(`lib/hidden-features` ADMIN_ONLY_ROUTE_PREFIXES). BE도 같은 경로를 관리자만 받는다.
 */
export function useCommunityVisible(): boolean {
  const { session } = useAuth();
  return isAdminSession(session);
}
