import { Suspense } from "react";
import { AuthRequiredShell } from "@/components/auth/auth-required-shell";
import { JournalClient } from "@/components/journal/journal-client";
import { pageMetadata } from "@/lib/seo";

export const metadata = pageMetadata({
  title: "메모장",
  description: "종목과 함께 남긴 투자 메모입니다.",
  noindex: true,
});

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
        loginNext="/journal"
        menu="journal"
      >
        <JournalClient />
      </AuthRequiredShell>
    </Suspense>
  );
}
