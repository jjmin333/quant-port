import { AuthRequiredShell } from "@/components/auth/auth-required-shell";
import { WatchlistClient } from "@/components/watchlist/watchlist-client";

export default function WatchlistPage() {
  return (
    <AuthRequiredShell
      title="관심종목"
      subtitle="워치리스트 · 실시간 시세"
      loginNext="/watchlist"
    >
      <WatchlistClient />
    </AuthRequiredShell>
  );
}
