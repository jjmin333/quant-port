import { Suspense } from "react";
import { AuthRequiredShell } from "@/components/auth/auth-required-shell";
import { TargetsClient } from "@/components/targets/targets-client";
import { MENU_COPY } from "@/lib/menu-copy";

export default function TargetsPage() {
  return (
    <Suspense
      fallback={
        <main className="flex flex-1 items-center justify-center p-8 text-sm text-[var(--text-muted)]">
          목표가를 불러오는 중…
        </main>
      }
    >
      <AuthRequiredShell
        title={MENU_COPY.targets.title}
        subtitle={MENU_COPY.targets.subtitle}
        loginNext="/targets"
        menu="targets"
      >
        <TargetsClient />
      </AuthRequiredShell>
    </Suspense>
  );
}
