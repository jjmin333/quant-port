import { Suspense } from "react";
import { AuthRequiredShell } from "@/components/auth/auth-required-shell";
import { JournalClient } from "@/components/journal/journal-client";
import { MENU_COPY } from "@/lib/menu-copy";

export default function JournalPage() {
  return (
    <Suspense
      fallback={
        <main className="flex flex-1 items-center justify-center p-8 text-sm text-[var(--text-muted)]">
          메모장을 불러오는 중…
        </main>
      }
    >
      <AuthRequiredShell
        title={MENU_COPY.journal.title}
        subtitle={MENU_COPY.journal.subtitle}
        loginNext="/journal"
        menu="journal"
      >
        <JournalClient />
      </AuthRequiredShell>
    </Suspense>
  );
}
