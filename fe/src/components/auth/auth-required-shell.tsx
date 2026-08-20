"use client";

import Link from "next/link";
import type { ReactNode } from "react";
import { PageHeader } from "@/components/layout/header";
import { PageScrollShell } from "@/components/layout/page-content-shell";
import { LoginRequiredPanel } from "@/components/auth/login-required-panel";
import { MenuEmptyState } from "@/components/common/menu-empty-state";
import { menuIcon } from "@/components/common/menu-icon";
import { SampleModeButton, useSampleMode } from "@/components/common/sample-mode";
import { useAuth } from "@/components/providers/auth-provider";
import { MENU_COPY, type MenuKey } from "@/lib/menu-copy";

type AuthRequiredShellProps = {
  title: string;
  subtitle?: string;
  loginNext: string;
  /**
   * 「내 투자」 메뉴 키. 주면 로그인하지 않은 방문자도 **데이터가 0건인 회원과 똑같은
   * 안내 화면**(무엇을 해주는 곳인지 · 3단계 · 예시 버튼)을 본다.
   *
   * 예전에는 여기서 로그인 패널만 띄웠다. 그런데 그 패널이 말해주는 것은 "로그인이
   * 필요하다"는 사실뿐이라, 방문자는 **이 메뉴가 무엇인지 모르는 채로** 가입 여부를
   * 정해야 했다. 설명이 필요한 쪽은 오히려 아직 안 써 본 사람이다.
   *
   * `?sample=1`이면 본문을 그대로 통과시킨다 — 본문 컴포넌트가 예시 모드에서 공개
   * 픽스처를 읽으므로 인증 없이도 안전하게 그려진다.
   */
  menu?: MenuKey;
  children: ReactNode;
};

export function AuthRequiredShell({
  title,
  subtitle,
  loginNext,
  menu,
  children,
}: AuthRequiredShellProps) {
  const { session, loading } = useAuth();
  const { active: sample, setActive: setSample } = useSampleMode();

  // 예시 모드는 세션을 기다리지 않는다 — 공개 데이터라 로딩 화면을 거칠 이유가 없다.
  if (menu && sample) {
    return children;
  }

  const actions = menu ? (
    <SampleModeButton active={false} onToggle={() => setSample(true)} emphasize />
  ) : undefined;

  if (loading) {
    return (
      <>
        <PageHeader title={title} subtitle={subtitle} actions={actions} />
        <PageScrollShell>
          <p className="text-sm text-[var(--text-muted)]">로딩 중…</p>
        </PageScrollShell>
      </>
    );
  }

  if (!session) {
    return (
      <>
        <PageHeader title={title} subtitle={subtitle} actions={actions} />
        <PageScrollShell>
          {menu ? (
            <LoggedOutIntro menu={menu} loginNext={loginNext} onSample={() => setSample(true)} />
          ) : (
            <LoginRequiredPanel
              loginNext={loginNext}
              description="로그인 후 이용할 수 있습니다."
            />
          )}
        </PageScrollShell>
      </>
    );
  }

  return children;
}

/**
 * 비로그인 안내 — 0건 회원이 보는 카드와 **같은 컴포넌트·같은 문구**를 쓰고
 * 버튼만 바뀐다(첫 동작 → 로그인). 두 상태에 서로 다른 화면을 두면 같은 메뉴를
 * 두 번 처음 만나게 된다.
 */
function LoggedOutIntro({
  menu,
  loginNext,
  onSample,
}: {
  menu: MenuKey;
  loginNext: string;
  onSample: () => void;
}) {
  const copy = MENU_COPY[menu];
  return (
    <MenuEmptyState
      icon={menuIcon(menu)}
      headline={copy.emptyHeadline}
      description={copy.emptyDescription}
      steps={copy.steps}
      primary={{ label: "로그인하고 시작하기", href: `/login?next=${encodeURIComponent(loginNext)}` }}
      secondary={
        <>
          <SampleModeButton
            active={false}
            onToggle={onSample}
            emphasize
            className="border border-[var(--sample-border)] px-3.5 py-1.5"
          />
          <Link
            href="/signup"
            className="rounded-lg border border-[var(--border)] px-3.5 py-1.5 text-[11px] text-[var(--text-secondary)] hover:bg-[var(--surface-elevated)]"
          >
            회원가입
          </Link>
        </>
      }
      footnote="기록은 내 계정에 저장됩니다"
    />
  );
}
