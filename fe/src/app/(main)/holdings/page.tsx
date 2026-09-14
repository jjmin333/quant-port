import { Suspense } from "react";
import { AuthRequiredShell } from "@/components/auth/auth-required-shell";
import { HoldingsData } from "./holdings-data";
import { pageMetadata } from "@/lib/seo";

export const metadata = pageMetadata({
  title: "내 포트폴리오",
  description: "보유 종목의 손익과 비중을 한 화면에서 봅니다.",
  noindex: true,
});

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
        loginNext="/holdings"
        menu="holdings"
      >
        <HoldingsData />
      </AuthRequiredShell>
    </Suspense>
  );
}
