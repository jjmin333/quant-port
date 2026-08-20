"use client";

import { useEffect, useState } from "react";
import { Header } from "@/components/layout/header";
import { Sidebar } from "@/components/layout/sidebar";
import { HeaderSearchProvider } from "@/components/providers/header-search-provider";
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
     */
    const stored = readStoredSidebarOpen();
    const syncSidebar = (matches: boolean) =>
      setSidebarOpen(matches ? (stored ?? true) : false);
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
      <div className="flex h-[100dvh] flex-col overflow-hidden bg-[var(--background)]">
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

export function AppShell({ children }: { children: React.ReactNode }) {
  return (
    <ShellModeProvider>
      <AppShellFrame>{children}</AppShellFrame>
    </ShellModeProvider>
  );
}
