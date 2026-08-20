import { Suspense } from "react";
import { AuthRequiredShell } from "@/components/auth/auth-required-shell";
import { WatchlistClient } from "@/components/watchlist/watchlist-client";
import { MENU_COPY } from "@/lib/menu-copy";

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
        title={MENU_COPY.watchlist.title}
        subtitle={MENU_COPY.watchlist.subtitle}
        loginNext="/watchlist"
        menu="watchlist"
      >
        <WatchlistClient />
      </AuthRequiredShell>
    </Suspense>
  );
}
