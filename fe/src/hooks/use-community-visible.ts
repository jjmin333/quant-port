"use client";

import { useAuth } from "@/components/providers/auth-provider";
import { isAdminSession } from "@/lib/admin-features";
import { isAdminOnlyRoute } from "@/lib/hidden-features";

/**
 * 커뮤니티·투자 리포트 입구(종목 화면 탭·카드, 기사 공유 링크 등)를 그릴지 —
 * `lib/hidden-features` ADMIN_ONLY_ROUTE_PREFIXES에 있으면 관리자에게만(10-13부터 모두에게 공개).
 */
export function useCommunityVisible(): boolean {
  const { session } = useAuth();
  return !isAdminOnlyRoute("/community") || isAdminSession(session);
}
