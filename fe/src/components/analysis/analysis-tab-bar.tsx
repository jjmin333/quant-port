"use client";

import { useEffect, useRef } from "react";
import { ANALYSIS_TABS, type AnalysisTabId } from "@/lib/analysis-tabs";
import { cn } from "@/lib/utils";

type AnalysisTabBarProps = {
  active: AnalysisTabId;
  onChange: (tab: AnalysisTabId) => void;
};

export function AnalysisTabBar({ active, onChange }: AnalysisTabBarProps) {
  const navRef = useRef<HTMLElement>(null);

  // Seven tabs can overflow a phone-width bar, so a deep link (e.g. ?tab=insider)
  // can land on a tab that's scrolled out of view. Keep the active one visible,
  // matching StockWorkspaceTabBar's behavior for the same problem.
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
    <nav
      ref={navRef}
      className="scrollbar-thin -mx-1 flex gap-1 overflow-x-auto border-b border-[var(--border)] pb-px"
      aria-label="종목 분석 메뉴"
    >
      {ANALYSIS_TABS.map((tab) => (
        <button
          key={tab.id}
          type="button"
          data-tab-id={tab.id}
          onClick={() => onChange(tab.id)}
          aria-current={active === tab.id ? "page" : undefined}
          className={cn(
            "shrink-0 whitespace-nowrap rounded-t-lg px-3 py-2 text-[13px] font-medium transition-colors",
            active === tab.id
              ? "border-b-2 border-[var(--brand)] text-[var(--brand)]"
              : "text-[var(--text-muted)] hover:text-[var(--text-secondary)]",
          )}
        >
          {tab.label}
        </button>
      ))}
    </nav>
  );
}
