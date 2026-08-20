"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import {
  Globe,
  Briefcase,
  Star,
  Newspaper,
  NotebookPen,
  Target,
  Settings,
  Shield,
  Stamp,
  PenLine,
  Database,
  Table2,
  Gauge,
  Sparkles,
  Calculator,
  ListChecks,
  Trophy,
  LineChart,
  CalendarClock,
  CalendarRange,
} from "lucide-react";
import { isAdminNavVisible } from "@/lib/admin-features";
import { useAuth } from "@/components/providers/auth-provider";
import { cn } from "@/lib/utils";

const navGroups = [
  {
    label: "순위",
    items: [
      { href: "/market", label: "실시간 차트", icon: LineChart },
      { href: "/analysis", label: "AI 추천 랭킹", icon: Trophy },
    ],
  },
  {
    label: "뉴스",
    items: [
      { href: "/news/domestic", label: "국내 주식", icon: Newspaper },
      { href: "/news/us", label: "해외 주식", icon: Globe },
    ],
  },
  {
    // 뉴스 그룹과 같은 결로 국내·해외를 가른다. 두 시장은 지표(연결/개별 vs
    // 장전/장후)와 데이터 성질이 달라, 한 목록에 섞으면 한쪽이 항상 이상해 보인다 —
    // 국내는 예정일이 없고 해외는 발표 시각이 있다.
    label: "실적",
    items: [
      { href: "/earnings/domestic", label: "국내 실적 일정", icon: CalendarClock },
      { href: "/earnings/us", label: "해외 실적 일정", icon: CalendarRange },
    ],
  },
  {
    label: "내 투자",
    items: [
      { href: "/holdings", label: "내 포트폴리오", icon: Briefcase },
      { href: "/watchlist", label: "관심 종목", icon: Star },
      { href: "/stamps", label: "스탬프", icon: Stamp },
      { href: "/journal", label: "메모장", icon: NotebookPen },
      { href: "/targets", label: "목표가 관리", icon: Target },
    ],
  },
  {
    // 리포트는 공개 발행물이라 사적 기록과 그룹을 분리한다.
    // 순위표(리더보드)는 두지 않는다 — 작성자를 줄 세우면 리포트가 대회
    // 참가작이 되고, 대회는 보상 없이는 유지되지 않는다.
    label: "커뮤니티",
    items: [{ href: "/reports", label: "투자 리포트", icon: PenLine }],
  },
  {
    label: "관리",
    items: [
      { href: "/admin", label: "뉴스 관리", icon: Shield },
      { href: "/admin/api", label: "API 관리", icon: Gauge },
      { href: "/admin/llm", label: "LLM 모델 관리", icon: Sparkles },
      { href: "/admin/score", label: "점수 관리", icon: Calculator },
      { href: "/admin/batch", label: "배치 관리", icon: ListChecks },
      { href: "/admin/cache", label: "캐시 관리", icon: Database },
      { href: "/admin/database", label: "데이터베이스 관리", icon: Table2 },
    ],
  },
];

const allNavHrefs = navGroups.flatMap((group) => group.items.map((item) => item.href));

/**
 * The single nav href that should render as active — the *longest* one that
 * matches the current path, so a parent route never lights up alongside its
 * child. "/admin" (뉴스 관리) is a prefix of every other admin page, so a plain
 * exact-or-child test marked it active on /admin/api, /admin/llm, … as well;
 * longest-match makes the child win and leaves only one item highlighted.
 */
function activeNavHref(pathname: string): string {
  let best = "";
  for (const href of allNavHrefs) {
    if (pathname !== href && !pathname.startsWith(`${href}/`)) continue;
    if (href.length > best.length) best = href;
  }
  return best;
}

type SidebarNavProps = {
  onNavigate?: () => void;
  collapsed?: boolean;
};

function SidebarNav({ onNavigate, collapsed = false }: SidebarNavProps) {
  const pathname = usePathname();
  const { session } = useAuth();
  const activeHref = activeNavHref(pathname);

  return (
    <>
      <nav className={cn("flex-1 overflow-y-auto py-3", collapsed ? "px-1" : "px-2")}>
        {navGroups
          .map((group) => ({
            ...group,
            items: group.items.filter((item) => isAdminNavVisible(item.href, session)),
          }))
          .filter((group) => group.items.length > 0)
          .map((group) => (
          <div key={group.label} className={cn(collapsed ? "mb-3" : "mb-4")}>
            <p
              className={cn(
                "font-semibold text-muted-foreground",
                collapsed
                  ? "mb-1 text-center text-[9px] tracking-tight"
                  : "mb-1.5 whitespace-nowrap px-1.5 text-[10px] uppercase tracking-wider",
              )}
            >
              {group.label}
            </p>
            <ul className="space-y-0.5">
              {group.items.map((item) => {
                const Icon = item.icon;
                const active = item.href === activeHref;

                return (
                  <li key={item.href}>
                    <Link
                      href={item.href}
                      onClick={(e) => {
                        if (active) {
                          e.preventDefault();
                          window.location.reload();
                          return;
                        }
                        onNavigate?.();
                      }}
                      title={item.label}
                      className={cn(
                        "flex items-center rounded-lg text-sm transition-colors",
                        collapsed
                          ? "justify-center px-1 py-1.5"
                          : "gap-2 whitespace-nowrap px-2 py-1.5",
                        active
                          ? "bg-sidebar-accent font-medium text-sidebar-accent-foreground"
                          : "text-muted-foreground hover:bg-sidebar-accent/60 hover:text-sidebar-accent-foreground",
                      )}
                    >
                      <Icon className="h-4 w-4 shrink-0" />
                      {!collapsed && item.label}
                    </Link>
                  </li>
                );
              })}
            </ul>
          </div>
        ))}
      </nav>

      <div className={cn("shrink-0 border-t border-sidebar-border", collapsed ? "p-1" : "p-2")}>
        <Link
          href="/settings"
          onClick={onNavigate}
          title="설정"
          className={cn(
            "flex items-center rounded-lg py-2 text-xs text-muted-foreground hover:bg-sidebar-accent/60 hover:text-sidebar-accent-foreground",
            collapsed ? "justify-center px-1 py-1.5" : "gap-2 whitespace-nowrap px-2",
          )}
        >
          <Settings className="h-4 w-4 shrink-0" />
          {!collapsed && "설정"}
        </Link>
      </div>
    </>
  );
}

type SidebarProps = {
  open: boolean;
  onClose: () => void;
};

export function Sidebar({ open, onClose }: SidebarProps) {
  const handleNavigate = () => {
    if (window.matchMedia("(max-width: 1023px)").matches) {
      onClose();
    }
  };

  return (
    <>
      <div
        className={cn(
          "hidden h-full shrink-0 flex-col border-r border-sidebar-border bg-sidebar text-sidebar-foreground transition-[width] duration-300 ease-in-out lg:flex",
          open ? "w-[var(--sidebar-width)]" : "w-[var(--sidebar-collapsed-width)]",
        )}
      >
        <div className="flex min-h-0 flex-1 flex-col overflow-hidden">
          <SidebarNav collapsed={!open} />
        </div>
      </div>

      {open && (
        <button
          type="button"
          aria-label="메뉴 닫기"
          className="fixed inset-0 z-40 bg-black/50 lg:hidden"
          onClick={onClose}
        />
      )}

      <aside
        className={cn(
          "fixed inset-y-0 left-0 z-50 flex w-[var(--sidebar-width)] min-w-[var(--sidebar-width)] flex-col border-r border-sidebar-border bg-sidebar text-sidebar-foreground transition-transform duration-300 ease-in-out lg:hidden",
          open ? "translate-x-0" : "-translate-x-full",
        )}
      >
        <div className="flex h-14 shrink-0 items-center border-b border-sidebar-border px-4">
          <span className="whitespace-nowrap text-sm font-bold tracking-[0.18em] text-sidebar-foreground">
            QUANT PORT
          </span>
        </div>
        <SidebarNav onNavigate={handleNavigate} />
      </aside>
    </>
  );
}
