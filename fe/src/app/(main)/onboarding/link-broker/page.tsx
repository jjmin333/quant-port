import { AuthRequiredShell } from "@/components/auth/auth-required-shell";
import LinkBrokerPage from "./link-broker-client";

export default function Page() {
  return (
    <AuthRequiredShell
      title="토스증권 계좌 연동"
      subtitle="Open API 키 등록"
      loginNext="/onboarding/link-broker"
    >
      <LinkBrokerPage />
    </AuthRequiredShell>
  );
}
