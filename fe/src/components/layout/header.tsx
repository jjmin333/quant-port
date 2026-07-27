"use client";

import Link from "next/link";
import { Bell, LogOut, Search, Settings, X } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { SidebarToggle } from "@/components/layout/sidebar-toggle";
import { useAuth } from "@/components/providers/auth-provider";
import { useHeaderSearch } from "@/components/providers/header-search-provider";
import { ACCOUNT_FEATURES_VISIBLE } from "@/lib/account-features";
import { cn } from "@/lib/utils";

type HeaderProps = {
  sidebarOpen: boolean;
  onSidebarToggle: () => void;
};

export function Header({ sidebarOpen, onSidebarToggle }: HeaderProps) {
  const { session, logout, loading } = useAuth();
  const initial = session?.user.displayName?.[0]?.toUpperCase() ?? "U";
  const {
    config,
    inputValue,
    setInputValue,
    clearSearch,
    submitSearch,
    isSearchActive,
  } = useHeaderSearch();

  const showSearch = config.mode !== "hidden";

  return (
    <header className="flex h-16 min-w-[var(--header-min-width)] shrink-0 items-center gap-3 border-b border-[var(--border)] bg-[var(--background)] px-4 sm:px-6">
      <div className="flex shrink-0 items-center gap-2">
        <SidebarToggle open={sidebarOpen} onToggle={onSidebarToggle} />
        <Link href="/" className="flex shrink-0 items-center gap-2.5 whitespace-nowrap">
          <span className="text-base leading-none text-[var(--accent)]" aria-hidden>
            ◆
          </span>
          <span className="text-sm font-bold tracking-[0.18em] text-[var(--text-primary)]">
            QUANT PORT
          </span>
        </Link>
      </div>

      <div className="flex min-w-[12rem] flex-1 justify-center px-1 sm:px-2">
        {showSearch ? (
          <div className="relative w-full min-w-[12rem] max-w-md">
            <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-[var(--text-muted)]" />
            <input
              type="search"
              value={inputValue}
              onChange={(e) => setInputValue(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === "Enter") {
                  e.preventDefault();
                  submitSearch();
                }
                if (e.key === "Escape") {
                  clearSearch();
                }
              }}
              placeholder={config.placeholder}
              aria-label={config.placeholder}
              className="h-9 w-full min-w-[12rem] rounded-lg border border-[var(--border)] bg-[var(--surface)] pl-9 pr-9 text-sm text-[var(--text-primary)] placeholder:text-[var(--text-muted)] focus:border-[var(--accent)] focus:outline-none"
            />
            {isSearchActive && (
              <button
                type="button"
                onClick={clearSearch}
                aria-label="검색어 지우기"
                className="absolute right-2 top-1/2 flex h-6 w-6 -translate-y-1/2 items-center justify-center rounded text-[var(--text-muted)] hover:bg-[var(--surface-elevated)] hover:text-[var(--text-primary)]"
              >
                <X className="h-3.5 w-3.5" />
              </button>
            )}
          </div>
        ) : (
          <div className="h-9 w-full max-w-md" aria-hidden />
        )}
      </div>

      <div className="flex shrink-0 items-center gap-2 sm:gap-3">
        {!loading && !session && (
          <>
            <Link
              href="/login"
              className="hidden h-9 items-center rounded-lg border border-[var(--border)] px-3 text-sm text-[var(--text-secondary)] hover:text-[var(--text-primary)] sm:inline-flex"
            >
              로그인
            </Link>
            <Link
              href="/signup"
              className="inline-flex h-9 items-center rounded-lg bg-[var(--accent)] px-3 text-sm font-semibold text-white hover:opacity-90"
            >
              회원가입
            </Link>
          </>
        )}

        {!loading && session && ACCOUNT_FEATURES_VISIBLE && !session.brokerLinked && (
          <Link
            href="/onboarding/link-broker"
            className="hidden rounded-lg border border-amber-500/40 bg-amber-500/10 px-3 py-1.5 text-xs font-medium text-amber-300 sm:inline-flex"
          >
            계좌 연동
          </Link>
        )}

        {!loading && session && (
          <>
            <button
              type="button"
              className="relative flex h-9 w-9 shrink-0 items-center justify-center rounded-lg border border-[var(--border)] bg-[var(--surface)] text-[var(--text-secondary)] hover:text-[var(--text-primary)]"
              aria-label="알림"
            >
              <Bell className="h-4 w-4" />
              <span className="absolute -right-0.5 -top-0.5 flex h-4 w-4 items-center justify-center rounded-full bg-rose-500 text-[10px] font-bold text-white">
                3
              </span>
            </button>

            <Link
              href="/settings"
              className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg border border-[var(--border)] bg-[var(--surface)] text-[var(--text-secondary)] hover:text-[var(--text-primary)]"
              aria-label="설정"
            >
              <Settings className="h-4 w-4" />
            </Link>

            <div className="group relative">
              <div
                className="flex h-9 w-9 shrink-0 cursor-pointer items-center justify-center rounded-full bg-[var(--accent-muted)] text-sm font-semibold text-[var(--accent)]"
                title={session.user.email}
              >
                {initial}
              </div>
              <div className="invisible absolute right-0 top-full z-50 mt-2 w-48 rounded-lg border border-[var(--border)] bg-[var(--surface)] py-1 opacity-0 shadow-lg transition group-hover:visible group-hover:opacity-100">
                <div className="border-b border-[var(--border)] px-3 py-2">
                  <p className="truncate text-sm font-medium text-[var(--text-primary)]">
                    {session.user.displayName}
                  </p>
                  <p className="truncate text-xs text-[var(--text-muted)]">{session.user.email}</p>
                </div>
                <button
                  type="button"
                  onClick={() => void logout()}
                  className="flex w-full items-center gap-2 px-3 py-2 text-left text-sm text-[var(--text-secondary)] hover:bg-[var(--surface-elevated)] hover:text-[var(--text-primary)]"
                >
                  <LogOut className="h-4 w-4" />
                  로그아웃
                </button>
              </div>
            </div>
          </>
        )}
      </div>
    </header>
  );
}

export function PageHeader({ title, subtitle }: { title: string; subtitle?: string }) {
  return (
    <div className="min-w-[var(--dashboard-content-min)] shrink-0 border-b border-[var(--border)] bg-[var(--background)] px-4 py-4 sm:px-6">
      <h1 className="whitespace-nowrap text-lg font-semibold text-[var(--text-primary)]">
        {title}
      </h1>
      {subtitle && (
        <p className={cn("mt-0.5 whitespace-nowrap text-xs text-[var(--text-muted)]")}>{subtitle}</p>
      )}
    </div>
  );
}

export function PageStub({ title, description }: { title: string; description: string }) {
  return (
    <div className="flex flex-1 flex-col items-center justify-center p-8 text-center">
      <Badge variant="info" className="mb-3">
        디자인 단계
      </Badge>
      <h2 className="text-xl font-semibold text-[var(--text-primary)]">{title}</h2>
      <p className="mt-2 max-w-md text-sm text-[var(--text-secondary)]">{description}</p>
    </div>
  );
}
