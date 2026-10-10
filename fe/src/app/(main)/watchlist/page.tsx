import { Suspense } from "react";
import { AuthRequiredShell } from "@/components/auth/auth-required-shell";
import { WatchlistTabs } from "@/components/watchlist/watchlist-tabs";
import { resolvePageMetadata } from "@/lib/page-meta.server";

/*
 * 제목·설명은 `lib/page-meta-routes`의 기본값이고, 관리자가 `/admin/seo`에서
 * 덮어쓸 수 있다 (2026-09-15 사용자 요청 2). `const metadata`가 아니라 함수인
 * 이유는 그 덮어쓰기를 **렌더 시점에** 읽어야 하기 때문이다.
 */
export function generateMetadata() {
  return resolvePageMetadata("/watchlist");
}

export default function WatchlistPage() {
  return (
    <Suspense
      fallback={
        <main className="flex flex-1 items-center justify-center p-8 text-sm text-[var(--text-muted)]">
          관심 목록을 불러오는 중…
        </main>
      }
    >
      <AuthRequiredShell
        loginNext="/watchlist"
        menu="watchlist"
      >
        <WatchlistTabs />
      </AuthRequiredShell>
    </Suspense>
  );
}
