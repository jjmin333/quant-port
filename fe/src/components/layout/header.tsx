"use client";

import Link from "next/link";
import { useState } from "react";
import { LogOut, Search, UserCircle } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { ColorModeToggle } from "@/components/layout/color-mode-toggle";
import { NotificationBell } from "@/components/layout/notification-bell";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { SidebarToggle } from "@/components/layout/sidebar-toggle";
import {
  pageShellClass,
  type PageShellWidth,
} from "@/components/layout/page-content-shell";
import { StockSearchField } from "@/components/layout/stock-search-field";
import { useAuth } from "@/components/providers/auth-provider";
import { useHeaderSearch } from "@/components/providers/header-search-provider";
import { useShellMode } from "@/components/providers/shell-mode-provider";
import { cn } from "@/lib/utils";

type HeaderProps = {
  sidebarOpen: boolean;
  onSidebarToggle: () => void;
  showSidebarToggle?: boolean;
};

function HeaderActions({ compact }: { compact?: boolean }) {
  const { session, logout, loading } = useAuth();
  const initial = session?.user.displayName?.[0]?.toUpperCase() ?? "U";

  if (!loading && !session) {
    return (
      <>
        <Button
          asChild
          variant="outline"
          size={compact ? "sm" : "default"}
          className="hidden sm:inline-flex"
        >
          <Link href="/login">로그인</Link>
        </Button>
        <Button asChild size={compact ? "sm" : "default"}>
          <Link href="/signup">회원가입</Link>
        </Button>
      </>
    );
  }

  if (!loading && session) {
    /*
     * 헤더 우측은 아바타 하나로 통합했다 (docs/UI_SPEC.md §4).
     *  - 알림 🔔: 예전 것은 배지 3이 하드코딩된 가짜라 걷었고, 2026-10-10에 진짜로 다시 넣었다
     *    (`NotificationBell` — 관심 목록의 변화, 앱 안 알림 칸).
     *  - 색상 모드: 헤더 우측 아이콘 버튼으로 전환한다.
     *  - 계정: 드롭다운에서 /settings(계정 전용)로 연다.
     * 예전 메뉴는 group-hover CSS라 키보드로 열 수 없었다. DropdownMenu가 이를 해결한다.
     */
    return (
      <>
      <NotificationBell compact={compact} />
      <DropdownMenu>
        <DropdownMenuTrigger
          className={cn(
            "flex shrink-0 cursor-pointer items-center justify-center rounded-full bg-muted font-medium text-foreground outline-none focus-visible:ring-[3px] focus-visible:ring-ring/50",
            compact ? "size-7 text-cell" : "size-8 text-cell",
          )}
          aria-label="계정 메뉴"
        >
          {initial}
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end" className="w-52">
          <DropdownMenuLabel className="font-normal">
            <p className="truncate text-cell font-medium">{session.user.displayName}</p>
            <p className="truncate text-cell text-muted-foreground">{session.user.email}</p>
          </DropdownMenuLabel>
          <DropdownMenuSeparator />
          <DropdownMenuItem asChild>
            <Link href="/settings">
              <UserCircle />
              계정
            </Link>
          </DropdownMenuItem>
          <DropdownMenuItem onSelect={() => void logout()}>
            <LogOut />
            로그아웃
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
      </>
    );
  }

  return null;
}

function BrandLink({ compact }: { compact?: boolean }) {
  return (
    <Link href="/" className="flex shrink-0 items-center gap-1.5 whitespace-nowrap lg:gap-2.5">
      <span
        className={cn("leading-none text-[var(--brand)]", compact ? "text-sm" : "text-base")}
        aria-hidden
      >
        ◆
      </span>
      <span
        className={cn(
          "font-bold text-[var(--text-primary)]",
          compact
            ? "text-micro tracking-[0.12em]"
            : "text-sm tracking-[0.18em]",
        )}
      >
        QUANT PORT
      </span>
    </Link>
  );
}

export function Header({
  sidebarOpen,
  onSidebarToggle,
  showSidebarToggle = true,
}: HeaderProps) {
  const { config } = useHeaderSearch();
  const { isCompact } = useShellMode();
  const showSearch = config.mode !== "hidden";

  /*
    접히는 검색 (`config.collapsible`) — 지금은 종목 상세뿐이다.

    화면을 옮기면 다시 접는다. 종목 A에서 펼쳐 둔 입력줄이 종목 B로 넘어가서도
    펼쳐져 있으면, 접기로 되찾은 43px이 실제로는 안 돌아온다.
  */
  const collapsibleSearch = showSearch && Boolean(config.collapsible);
  const [searchOpen, setSearchOpen] = useState(false);
  // `config`는 경로마다 새로 만들어지므로(useMemo[pathname]) 이것이 곧 「화면이
  // 바뀌었나」다. 이펙트가 아니라 렌더 중에 되돌린다 — 이펙트로 하면 접힌 헤더가
  // 한 프레임 펼쳐졌다 닫힌다.
  const [searchConfigSeen, setSearchConfigSeen] = useState(config);
  if (searchConfigSeen !== config) {
    setSearchConfigSeen(config);
    setSearchOpen(false);
  }
  const searchRowVisible = showSearch && (!collapsibleSearch || searchOpen);

  if (isCompact) {
    return (
      <header className="shrink-0 border-b border-[var(--border)] bg-[var(--background)] px-3 py-2">
        <div className="flex items-center gap-2">
          {showSidebarToggle && (
            <SidebarToggle open={sidebarOpen} onToggle={onSidebarToggle} />
          )}
          <BrandLink compact />
          <div className="flex-1" />
          <div className="flex shrink-0 items-center gap-1.5">
            {/* 색상 모드 버튼을 대체하지 않는다 — 그 왼쪽에 선다. */}
            {collapsibleSearch && (
              <button
                type="button"
                onClick={() => setSearchOpen((open) => !open)}
                aria-label={searchOpen ? "검색 닫기" : "종목 검색"}
                aria-expanded={searchOpen}
                aria-controls="header-stock-search-row"
                className={cn(
                  "flex size-7 shrink-0 items-center justify-center rounded-full border transition-colors outline-none focus-visible:ring-[3px] focus-visible:ring-ring/50",
                  // 선택 상태는 중립색이다 (UI_SPEC §5) — 이 앱에서 파랑은 하락이다.
                  searchOpen
                    ? "border-transparent bg-accent text-accent-foreground"
                    : "border-border bg-background text-foreground shadow-xs hover:bg-muted",
                )}
              >
                <Search className="size-3.5" strokeWidth={1.75} />
              </button>
            )}
            <ColorModeToggle compact />
            <HeaderActions compact />
          </div>
        </div>
        {searchRowVisible && (
          <div
            id="header-stock-search-row"
            className="mt-2 flex w-full min-w-0 justify-center"
          >
            <StockSearchField
              compact
              autoFocus={collapsibleSearch}
              className="w-full min-w-0 max-w-md"
              suggestionsId="header-stock-suggestions"
            />
          </div>
        )}
      </header>
    );
  }

  return (
    // 높이 56px 고정 (docs/UI_SPEC.md §4). 화면 종류와 무관하게 같은 높이를 쓴다.
    <header className="flex h-14 min-w-0 shrink-0 items-center gap-3 border-b bg-background px-4 sm:px-6">
      <div className="flex shrink-0 items-center gap-2">
        {showSidebarToggle && (
          <SidebarToggle open={sidebarOpen} onToggle={onSidebarToggle} />
        )}
        <BrandLink />
      </div>

      <div className="flex min-w-0 flex-1 justify-center px-1 sm:px-2">
        {showSearch ? (
          <StockSearchField className="w-full min-w-0 max-w-md" />
        ) : (
          <div className="h-8 w-full max-w-md" aria-hidden />
        )}
      </div>

      <div className="flex shrink-0 items-center gap-2 sm:gap-3">
        <ColorModeToggle />
        <HeaderActions />
      </div>
    </header>
  );
}

export function PageHeader({
  title,
  subtitle,
  width = "wide",
  actions,
}: {
  title: string;
  subtitle?: string;
  /** Must match the page body's archetype, or the two columns' edges disagree. */
  width?: PageShellWidth;
  /**
   * 페이지 단위 동작(필터·정렬·새로 만들기 등). 제목 오른쪽에 붙는다.
   *
   * 이 자리가 없던 동안 각 페이지가 본문 최상단에 제각기 버튼 줄을 만들었고, 그래서
   * 리포트 필터·관심종목 추가·관리자 실행 버튼이 화면마다 다른 위치에 있었다.
   * 새 화면의 동작 버튼은 본문이 아니라 여기에 넣는다 (docs/UI_SPEC.md §7).
   */
  actions?: React.ReactNode;
}) {
  return (
    <div className="shrink-0 border-b bg-background lg:min-w-[var(--dashboard-content-min)]">
      <div
        className={cn(
          pageShellClass(width),
          // 거터는 `.page-shell-wide`가 가진다. 헤더가 따로 px를 얹으면 본문 컬럼과
          // 좌우 경계가 어긋난다 — 과거 마켓 레이더가 겉돌아 보이던 원인이 그것이었다.
          "flex min-w-0 flex-wrap items-start gap-x-3 gap-y-2 py-3 sm:py-4",
        )}
      >
        <div className="min-w-0 flex-1">
          <h1 className="text-page font-semibold">{title}</h1>
          {subtitle && (
            <p className="mt-0.5 text-cell leading-relaxed text-muted-foreground sm:whitespace-nowrap">
              {subtitle}
            </p>
          )}
        </div>
        {actions && (
          // 좁은 화면에서는 아랫줄로 내려가되 우측 정렬을 유지한다.
          <div className="ml-auto flex shrink-0 flex-wrap items-center gap-1.5">{actions}</div>
        )}
      </div>
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
