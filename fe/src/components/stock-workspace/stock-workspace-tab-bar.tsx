"use client";

import { useEffect, useRef } from "react";
import {
  STOCK_WORKSPACE_TABS,
  type StockWorkspaceTabId,
} from "@/lib/stock-workspace-tabs";
import { cn } from "@/lib/utils";

type StockWorkspaceTabBarProps = {
  active: StockWorkspaceTabId;
  onChange: (tab: StockWorkspaceTabId) => void;
  tabs?: readonly { id: StockWorkspaceTabId; label: string }[];
  className?: string;
};

export function StockWorkspaceTabBar({
  active,
  onChange,
  tabs = STOCK_WORKSPACE_TABS,
  className,
}: StockWorkspaceTabBarProps) {
  const navRef = useRef<HTMLElement>(null);

  // Nine tabs overflow a phone-width bar, so the active one can sit off-screen
  // after a programmatic tab change (e.g. the 개요 shortcut cards). Keep it in
  // view so the user can always see where they are.
  useEffect(() => {
    const nav = navRef.current;
    const el = nav?.querySelector<HTMLElement>(`[data-tab-id="${active}"]`);
    if (!nav || !el) return;
    if (nav.scrollWidth <= nav.clientWidth) return;
    const left = el.offsetLeft - (nav.clientWidth - el.offsetWidth) / 2;
    const reduceMotion = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
    nav.scrollTo({ left: Math.max(0, left), behavior: reduceMotion ? "auto" : "smooth" });
  }, [active]);

  return (
    /*
     * 우측 페이드는 "오른쪽에 탭이 더 있다"는 유일한 신호다 — 스크롤바가 `scrollbar-thin`
     * 이라 눈에 띄지 않아서, 좁은 화면에서 뒤쪽 탭(뉴스·내부자 거래)의 존재를 알 방법이
     * 없었다 (docs/UI_SPEC.md §8.1).
     * 페이드는 배경색과 같은 그라디언트라 스크롤이 끝까지 간 상태에서도 해가 없다.
     */
    <div className={cn("relative min-w-0", className)}>
      <nav
        ref={navRef}
        className="scrollbar-thin flex gap-0.5 overflow-x-auto border-b bg-card"
        aria-label="종목 탭"
      >
        {tabs.map((tab) => (
          <button
            key={tab.id}
            type="button"
            data-tab-id={tab.id}
            onClick={() => onChange(tab.id)}
            aria-current={active === tab.id ? "page" : undefined}
            className={cn(
              // whitespace-nowrap keeps two-word labels (내부자 거래, 차트·호가)
              // on one line instead of wrapping and doubling the bar height.
              "shrink-0 whitespace-nowrap border-b-2 px-3 py-2 text-xs font-medium transition-colors",
              // 활성 표시는 중립. 브랜드 파랑은 이 앱에서 하락을 뜻한다 (UI_SPEC §5)
              active === tab.id
                ? "border-foreground text-foreground"
                : "border-transparent text-muted-foreground hover:text-foreground",
            )}
          >
            {tab.label}
          </button>
        ))}
      </nav>
      <div
        aria-hidden
        className="pointer-events-none absolute inset-y-0 right-0 w-8 bg-gradient-to-l from-card to-transparent"
      />
    </div>
  );
}
