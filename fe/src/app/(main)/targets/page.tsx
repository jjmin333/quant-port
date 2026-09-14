import { Suspense } from "react";
import { AuthRequiredShell } from "@/components/auth/auth-required-shell";
import { TargetsClient } from "@/components/targets/targets-client";
import { pageMetadata } from "@/lib/seo";

export const metadata = pageMetadata({
  title: "목표가 관리",
  description: "종목별 목표가와 도달 현황입니다.",
  noindex: true,
});

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
        loginNext="/targets"
        menu="targets"
      >
        <TargetsClient />
      </AuthRequiredShell>
    </Suspense>
  );
}
