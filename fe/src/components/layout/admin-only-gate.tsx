"use client";

import { useEffect } from "react";
import { usePathname, useRouter } from "next/navigation";
import { useAuth } from "@/components/providers/auth-provider";
import { isAdminSession } from "@/lib/admin-features";
import { isAdminOnlyRoute } from "@/lib/hidden-features";

/**
 * 관리자 전용 화면의 문(2026-10-12, `lib/hidden-features` ADMIN_ONLY_ROUTE_PREFIXES).
 * 세션을 확인하는 동안은 아무것도 그리지 않고, 관리자가 아니면 홈으로 보낸다.
 * 데이터 차단은 BE가 한다 — 이 문은 빈 화면 대신 길을 돌려주는 일만 한다.
 */
export function AdminOnlyGate({ children }: { children: React.ReactNode }) {
  const { session, loading } = useAuth();
  const router = useRouter();
  const allowed = !isAdminOnlyRoute(usePathname()) || isAdminSession(session);

  useEffect(() => {
    if (!loading && !allowed) router.replace("/");
  }, [loading, allowed, router]);

  return allowed ? <>{children}</> : null;
}
