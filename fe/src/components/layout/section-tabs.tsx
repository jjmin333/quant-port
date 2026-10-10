"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { navSectionOf } from "@/lib/nav-sections";
import { cn } from "@/lib/utils";

/**
 * 묶인 메뉴의 공통 탭 줄 (2026-10-10 사용자 결정 A, `lib/nav-sections.ts`).
 *
 * 앱 셸의 세로 flex 맨 위에 `shrink-0`으로 선다 — 아래 화면은 원래대로 남은 높이를 채운다.
 * 묶음에 속하지 않는 주소(홈·종목 화면·내 투자)에서는 아무것도 그리지 않는다.
 */
export function SectionTabs() {
  const pathname = usePathname();
  const hit = navSectionOf(pathname);
  if (!hit) return null;
  return (
    <nav
      aria-label={hit.section.label}
      className="flex shrink-0 items-center gap-1 overflow-x-auto border-b border-[var(--border)] px-3 py-1.5"
    >
      <span className="mr-1 shrink-0 text-micro font-semibold text-[var(--text-muted)]">{hit.section.label}</span>
      {hit.section.tabs.map((t) => {
        const active = t.href === hit.tab.href;
        return (
          <Link
            key={t.href}
            href={t.href}
            aria-current={active ? "page" : undefined}
            className={cn(
              "shrink-0 rounded-full px-2.5 py-0.5 text-cell",
              active
                ? "bg-[var(--brand)] font-semibold text-white"
                : "text-[var(--text-secondary)] hover:bg-[var(--surface-elevated)]",
            )}
          >
            {t.label}
          </Link>
        );
      })}
    </nav>
  );
}
