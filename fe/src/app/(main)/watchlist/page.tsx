import { Suspense } from "react";
import { AuthRequiredShell } from "@/components/auth/auth-required-shell";
import { WatchlistClient } from "@/components/watchlist/watchlist-client";
import { pageMetadata } from "@/lib/seo";

export const metadata = pageMetadata({
  title: "관심 종목",
  description: "담은 가격과 함께 기록한 관심 종목 목록입니다.",
  noindex: true,
});

export default function WatchlistPage() {
  return (
    <Suspense
      fallback={
        <main className="flex flex-1 items-center justify-center p-8 text-sm text-[var(--text-muted)]">
          관심 종목을 불러오는 중…
        </main>
      }
    >
      <AuthRequiredShell
        loginNext="/watchlist"
        menu="watchlist"
      >
        <WatchlistClient />
      </AuthRequiredShell>
    </Suspense>
  );
}
