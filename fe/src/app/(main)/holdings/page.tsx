import { AuthRequiredShell } from "@/components/auth/auth-required-shell";
import { HoldingsData } from "./holdings-data";

export default function HoldingsPage() {
  return (
    <AuthRequiredShell
      title="보유종목 통합"
      subtitle="계좌별 · 종목별 통합 보유 현황"
      loginNext="/holdings"
      brokerRequired
    >
      <HoldingsData />
    </AuthRequiredShell>
  );
}
