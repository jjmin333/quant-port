"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import {
  Globe,
  Layers,
  LineChart,
  Link2,
  Star,
  FileText,
  Newspaper,
  PieChart,
  ShieldAlert,
  Receipt,
  BookOpen,
  Target,
  TrendingUp,
  Settings,
} from "lucide-react";
import { isSidebarNavVisible } from "@/lib/account-features";
import { cn } from "@/lib/utils";

const navGroups = [
  {
    label: "핵심",
    items: [
      { href: "/quotes", label: "실시간 시세", icon: LineChart },
      { href: "/watchlist", label: "관심종목", icon: Star },
    ],
  },
  {
    label: "계좌",
    items: [
      { href: "/onboarding/link-broker", label: "계좌 연동", icon: Link2 },
      { href: "/holdings", label: "보유종목 통합", icon: Layers },
      { href: "/transactions", label: "거래내역", icon: Receipt },
    ],
  },
  {
    label: "분석",
    items: [
      { href: "/analysis", label: "종목 분석", icon: FileText },
      { href: "/portfolio", label: "포트폴리오 분석", icon: PieChart },
      { href: "/risk", label: "리스크 분석", icon: ShieldAlert },
    ],
  },
  {
    label: "뉴스",
    items: [
      { href: "/news/domestic", label: "국내 주식", icon: Newspaper },
      { href: "/news/us", label: "미국 주식", icon: Globe },
    ],
  },
  {
    label: "기록",
    items: [
      { href: "/journal", label: "AI 투자일지", icon: BookOpen },
      { href: "/targets", label: "목표가 관리", icon: Target },
      { href: "/backtest", label: "백테스트", icon: TrendingUp },
    ],
  },
];

type SidebarNavProps = {
  onNavigate?: () => void;
  collapsed?: boolean;
};

function SidebarNav({ onNavigate, collapsed = false }: SidebarNavProps) {
  const pathname = usePathname();

  return (
    <>
      <nav className={cn("flex-1 overflow-y-auto py-4", collapsed ? "px-1" : "px-3")}>
        {navGroups
          .map((group) => ({
            ...group,
            items: group.items.filter((item) => isSidebarNavVisible(item.href)),
          }))
          .filter((group) => group.items.length > 0)
          .map((group) => (
          <div key={group.label} className={cn(collapsed ? "mb-4" : "mb-5")}>
            <p
              className={cn(
                "font-semibold text-[var(--text-muted)]",
                collapsed
                  ? "mb-1.5 text-center text-[9px] tracking-tight"
                  : "mb-2 whitespace-nowrap px-2 text-[10px] uppercase tracking-wider",
              )}
            >
              {group.label}
            </p>
            <ul className="space-y-0.5">
              {group.items.map((item) => {
                const Icon = item.icon;
                const active =
                  pathname === item.href || pathname.startsWith(`${item.href}/`);

                return (
                  <li key={item.href}>
                    <Link
                      href={item.href}
                      onClick={onNavigate}
                      title={item.label}
                      className={cn(
                        "flex items-center rounded-lg text-sm transition-colors",
                        collapsed
                          ? "justify-center px-1 py-1.5"
                          : "gap-2.5 whitespace-nowrap px-2.5 py-2",
                        active
                          ? "bg-[var(--accent-muted)] text-[var(--accent)]"
                          : "text-[var(--text-secondary)] hover:bg-[var(--surface-elevated)] hover:text-[var(--text-primary)]",
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

      <div className={cn("shrink-0 border-t border-[var(--border)]", collapsed ? "p-1" : "p-3")}>
        <Link
          href="/settings"
          onClick={onNavigate}
          title="설정"
          className={cn(
            "flex items-center rounded-lg py-2 text-sm text-[var(--text-secondary)] hover:bg-[var(--surface-elevated)]",
            collapsed ? "justify-center px-1 py-1.5" : "gap-2.5 whitespace-nowrap px-2.5",
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
          "hidden h-full shrink-0 flex-col border-r border-[var(--border)] bg-[var(--surface)] transition-[width] duration-300 ease-in-out lg:flex",
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
          "fixed inset-y-0 left-0 z-50 flex w-[var(--sidebar-width)] min-w-[var(--sidebar-width)] flex-col border-r border-[var(--border)] bg-[var(--surface)] transition-transform duration-300 ease-in-out lg:hidden",
          open ? "translate-x-0" : "-translate-x-full",
        )}
      >
        <div className="flex h-14 shrink-0 items-center border-b border-[var(--border)] px-4">
          <span className="whitespace-nowrap text-sm font-bold tracking-[0.18em] text-[var(--text-primary)]">
            QUANT PORT
          </span>
        </div>
        <SidebarNav onNavigate={handleNavigate} />
      </aside>
    </>
  );
}
