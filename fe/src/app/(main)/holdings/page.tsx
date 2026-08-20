import { Suspense } from "react";
import { AuthRequiredShell } from "@/components/auth/auth-required-shell";
import { MENU_COPY } from "@/lib/menu-copy";
import { HoldingsData } from "./holdings-data";

export default function HoldingsPage() {
  return (
    <Suspense
      fallback={
        <main className="flex flex-1 items-center justify-center p-8 text-sm text-[var(--text-muted)]">
          포트폴리오를 불러오는 중…
        </main>
      }
    >
      <AuthRequiredShell
        title={MENU_COPY.holdings.title}
        subtitle={MENU_COPY.holdings.subtitle}
        loginNext="/holdings"
        menu="holdings"
      >
        <HoldingsData />
      </AuthRequiredShell>
    </Suspense>
  );
}
