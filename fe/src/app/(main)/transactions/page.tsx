import { Suspense } from "react";
import { AuthRequiredShell } from "@/components/auth/auth-required-shell";
import { TransactionsClient } from "@/components/transactions/transactions-client";

export default function TransactionsPage() {
  return (
    <AuthRequiredShell
      title="거래내역"
      subtitle="체결 · 취소 내역"
      loginNext="/transactions"
      brokerRequired
    >
      <Suspense fallback={<div className="p-6 text-sm text-[var(--text-muted)]">로딩 중…</div>}>
        <TransactionsClient />
      </Suspense>
    </AuthRequiredShell>
  );
}
