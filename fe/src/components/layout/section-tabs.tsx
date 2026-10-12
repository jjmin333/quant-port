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
 *
 * 크기(2026-10-12 사용자 요청 4): 국내/해외가 이 줄로만 갈리므로 화면에서 가장 먼저 읽혀야 한다 —
 * 12px·높이 20px 알약이던 것을 14px·높이 36px(터치 목표)로 키웠다.
 */
export function SectionTabs() {
  const pathname = usePathname();
  const hit = navSectionOf(pathname);
  if (!hit) return null;
  return (
    <nav
      aria-label={hit.section.label}
      className="flex shrink-0 items-center gap-1.5 overflow-x-auto border-b border-[var(--border)] px-3 py-2"
    >
      <span className="mr-1.5 shrink-0 text-sm font-bold text-[var(--text-primary)]">{hit.section.label}</span>
      {hit.section.tabs.map((t) => {
        const active = t.href === hit.tab.href;
        return (
          <Link
            key={t.href}
            href={t.href}
            aria-current={active ? "page" : undefined}
            className={cn(
              "inline-flex h-9 shrink-0 items-center rounded-full border px-4 text-sm font-medium transition-colors",
              active
                ? "border-[var(--brand)] bg-[var(--brand)] font-semibold text-white"
                : "border-[var(--border)] text-[var(--text-secondary)] hover:bg-[var(--surface-elevated)] hover:text-[var(--text-primary)]",
            )}
          >
            {t.label}
          </Link>
        );
      })}
    </nav>
  );
}
