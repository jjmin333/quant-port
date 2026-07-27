"use client";

import type { ReactNode } from "react";
import { PageHeader } from "@/components/layout/header";
import { BrokerLinkRequiredPanel } from "@/components/auth/broker-link-required-panel";
import { LoginRequiredPanel } from "@/components/auth/login-required-panel";
import { useAuth } from "@/components/providers/auth-provider";

type AuthRequiredShellProps = {
  title: string;
  subtitle?: string;
  loginNext: string;
  brokerRequired?: boolean;
  children: ReactNode;
};

export function AuthRequiredShell({
  title,
  subtitle,
  loginNext,
  brokerRequired = false,
  children,
}: AuthRequiredShellProps) {
  const { session, loading } = useAuth();

  if (loading) {
    return (
      <>
        <PageHeader title={title} subtitle={subtitle} />
        <main className="flex-1 overflow-y-auto p-6">
          <p className="text-sm text-[var(--text-muted)]">로딩 중…</p>
        </main>
      </>
    );
  }

  if (!session) {
    return (
      <>
        <PageHeader title={title} subtitle={subtitle} />
        <main className="flex-1 overflow-y-auto p-6">
          <div className="mx-auto max-w-5xl">
            <LoginRequiredPanel
              loginNext={loginNext}
              description="로그인 후 이용할 수 있습니다. 계좌 연동은 선택 사항이며, 연동하지 않아도 시세·종목 분석 등은 이용 가능합니다."
            />
          </div>
        </main>
      </>
    );
  }

  if (brokerRequired && !session.brokerLinked) {
    return (
      <>
        <PageHeader title={title} subtitle={subtitle} />
        <main className="flex-1 overflow-y-auto p-6">
          <div className="mx-auto max-w-5xl">
            <BrokerLinkRequiredPanel />
          </div>
        </main>
      </>
    );
  }

  return children;
}
