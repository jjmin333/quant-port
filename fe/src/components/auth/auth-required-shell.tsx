"use client";

import Link from "next/link";
import type { ReactNode } from "react";
import { PageScrollShell } from "@/components/layout/page-content-shell";
import { LoginRequiredPanel } from "@/components/auth/login-required-panel";
import { MenuEmptyState } from "@/components/common/menu-empty-state";
import { menuIcon } from "@/components/common/menu-icon";
import {
  SampleModeButton,
  useAutoSample,
  useSampleMode,
} from "@/components/common/sample-mode";
import { useAuth } from "@/components/providers/auth-provider";
import { MENU_COPY, type MenuKey } from "@/lib/menu-copy";
import { buttonVariants } from "@/components/ui/button";
import { cn } from "@/lib/utils";

type AuthRequiredShellProps = {
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
  loginNext,
  menu,
  children,
}: AuthRequiredShellProps) {
  const { session, loading } = useAuth();
  const { active: sample, setActive: setSample } = useSampleMode();

  /*
    로그인하지 않은 방문자에게는 **묻지 않고 예시를 켠다** (2026-09-14 사용자 요청 4).

    이 사람에게는 「내 것」이 있을 수가 없다 — 목록을 받아 볼 필요도 없이 0건이
    확정이다. 그래서 여기가 `useAutoSample`을 부르기에 가장 이른 자리이고, 화면이
    자기 데이터를 한 번 받아 보는 왕복도 생기지 않는다.

    세션을 기다리는 동안(`loading`)은 켜지 않는다. 로그인한 사람에게 예시가 한 프레임
    번쩍이고 사라지면, 그건 안내가 아니라 깜빡임이다.
  */
  useAutoSample(Boolean(menu) && !loading && !session);

  // 예시 모드는 세션을 기다리지 않는다 — 공개 데이터라 로딩 화면을 거칠 이유가 없다.
  if (menu && sample) {
    return children;
  }

  /*
    비로그인 화면에는 「예시로 둘러보기」를 **한 번만** 둔다 (§R-2).

    예전에는 페이지 헤더 우측에도 같은 버튼이 있었다. 헤더가 사라져 그 버튼이
    본문 첫 줄로 내려오자, 안내 카드가 들고 있는 같은 버튼과 40px 간격으로 나란히
    서서 "둘이 뭐가 다르지"가 되었다 — 실측으로 한 화면에 같은 문구 ×2.
    카드 쪽이 문맥(무엇을 해주는 곳인지 · 3단계)을 함께 주므로 그쪽을 남긴다.
  */
  if (loading) {
    return (
      <PageScrollShell>
        <p className="text-sm text-[var(--text-muted)]">로딩 중…</p>
      </PageScrollShell>
    );
  }

  if (!session) {
    return (
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
            className={cn(buttonVariants({ variant: "outline", size: "sm" }), "border-[var(--border)] text-[var(--text-secondary)] hover:bg-[var(--surface-elevated)]")}
          >
            회원가입
          </Link>
        </>
      }
      footnote="기록은 내 계정에 저장됩니다"
    />
  );
}
