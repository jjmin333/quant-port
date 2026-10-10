"use client";

import { useEffect, useState } from "react";
import { OfflineBanner } from "@/components/common/offline-banner";
import { Header } from "@/components/layout/header";
import { QuoteTabTitle } from "@/components/layout/quote-tab-title";
import { Sidebar } from "@/components/layout/sidebar";
import { HeaderSearchProvider } from "@/components/providers/header-search-provider";
import { QuickLookProvider } from "@/components/providers/quick-look-provider";
import { ShellModeProvider, useShellMode } from "@/components/providers/shell-mode-provider";
import { cn } from "@/lib/utils";

const SIDEBAR_STORAGE_KEY = "quant-port-sidebar-open";

/** 사용자가 직접 토글한 값만 저장한다. 없으면 화면 폭으로 판단. */
function readStoredSidebarOpen(): boolean | null {
  try {
    const raw = localStorage.getItem(SIDEBAR_STORAGE_KEY);
    if (raw === "1") return true;
    if (raw === "0") return false;
  } catch {
    // localStorage 접근 불가(사생활 보호 모드 등) — 화면 폭 판단으로 넘어간다
  }
  return null;
}

function AppShellFrame({ children }: { children: React.ReactNode }) {
  const { isWebView, isCompact } = useShellMode();
  const [sidebarOpen, setSidebarOpen] = useState(false);

  useEffect(() => {
    if (isWebView) {
      setSidebarOpen(false);
      return;
    }
    const media = window.matchMedia("(min-width: 1024px)");
    /*
     * 저장된 선택이 있으면 그것이 이긴다. 예전에는 화면 폭만 보고 매번 초기화해서
     * **새로고침하면 접어둔 사이드바가 다시 펼쳐졌다**.
     * 좁은 화면에서는 사이드바가 오버레이라 "열림"을 복원하면 앱을 가린 채로 뜬다 —
     * 그래서 넓은 화면에서만 저장값을 적용한다.
     *
     * **처음 오는 사람에게는 접힌 채로 준다.** 펼친 사이드바는 220px, 접으면 64px라
     * 본문이 156px을 더 쓴다 — 홈의 카드 넉 줄이 한 칸씩 넓어지는 폭이다. 접힌
     * 상태에서도 아이콘 밑에 이름이 붙어(sidebar.tsx의 rail) 무슨 메뉴인지 읽히므로,
     * 예전처럼 "접으면 아이콘만 남아 못 찾는" 대가를 치르지 않는다.
     */
    const stored = readStoredSidebarOpen();
    const syncSidebar = (matches: boolean) =>
      setSidebarOpen(matches ? (stored ?? false) : false);
    syncSidebar(media.matches);

    const onChange = (event: MediaQueryListEvent) => syncSidebar(event.matches);
    media.addEventListener("change", onChange);
    return () => media.removeEventListener("change", onChange);
  }, [isWebView]);

  const toggleSidebar = () =>
    setSidebarOpen((prev) => {
      const next = !prev;
      try {
        localStorage.setItem(SIDEBAR_STORAGE_KEY, next ? "1" : "0");
      } catch {
        // 저장 실패는 무시 — 이번 세션에서만 유지된다
      }
      return next;
    });
  const closeSidebar = () => setSidebarOpen(false);

  return (
    <HeaderSearchProvider>
      <div className="relative flex h-[100dvh] flex-col overflow-hidden bg-[var(--background)]">
        {/* 끊긴 동안 화면마다 다른 말을 하지 않도록 앱 전체가 한 번만 알린다. */}
        <OfflineBanner />
        <div className="relative z-50 shrink-0">
          <Header
            sidebarOpen={sidebarOpen}
            onSidebarToggle={toggleSidebar}
            showSidebarToggle={!isWebView}
          />
        </div>

        <div
          className={cn(
            "flex min-h-0 flex-1 overflow-hidden",
            !isCompact && "overflow-x-auto",
          )}
        >
          <div
            className={cn(
              "flex min-w-0 flex-1",
              !isCompact && sidebarOpen && "lg:min-w-[var(--shell-min-width)]",
              !isCompact && !sidebarOpen && "lg:min-w-[var(--dashboard-content-min)]",
            )}
          >
            {!isWebView && <Sidebar open={sidebarOpen} onClose={closeSidebar} />}

            <div className="flex min-w-0 flex-1 flex-col overflow-hidden">{children}</div>
          </div>
        </div>
      </div>
    </HeaderSearchProvider>
  );
}

export function AppShell({
  children,
  /** 서버가 요청 UA로 찍은 「좁은 화면일 것」 — `(main)/layout.tsx` 참조. */
  initialMobile = false,
}: {
  children: React.ReactNode;
  initialMobile?: boolean;
}) {
  return (
    <ShellModeProvider initialMobile={initialMobile}>
      {/*
        종목·지수 퀵룩은 **껍데기 전체가 하나를 공유한다**. 목록 → 카드 → 기사 모달까지
        여러 겹 안쪽에 있는 칩이 창을 열 수 있어야 하고, 그 창은 한 번에 하나뿐이다
        (`quick-look-provider.tsx` 머리말).
      */}
      {/*
        브라우저 탭 제목이 마지막으로 연 종목의 시세를 말한다 (2026-09-15 사용자 요청 4).
        아무것도 그리지 않는다 — `document.title`만 쥔다. 껍데기에 **하나만** 있어야
        하므로(제목을 쥐는 곳이 둘이면 서로를 덮는다) 여기 둔다.

        이 컴포넌트가 그 종목의 **구독자**이기도 하다. 안 보이는 탭이 목록을 다 놓고도
        이 한 줄을 살려 두는 근거가 그 구독이다(`lib/pinned-symbols` 머리글).
      */}
      <QuoteTabTitle />
      <QuickLookProvider>
        <AppShellFrame>{children}</AppShellFrame>
      </QuickLookProvider>
    </ShellModeProvider>
  );
}
