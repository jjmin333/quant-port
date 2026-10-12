"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { useEffect, useState } from "react";
import {
  Briefcase,
  Heart,
  Newspaper,
  NotepadText,
  Crosshair,
  Shield,
  PenLine,
  Database,
  Table2,
  Tags,
  Gauge,
  Sparkles,
  Trophy,
  Calculator,
  ListChecks,
  LineChart,
  ScanSearch,
  CalendarClock,
  Search,
  TrendingUp,
  MessagesSquare,
  MessageSquareWarning,
  PiggyBank,
  Building2,
  Gavel,
  Percent,
  HandCoins,
} from "lucide-react";
import { isAdminNavVisible, isAdminSession } from "@/lib/admin-features";
import { isHiddenRoute } from "@/lib/hidden-features";
import { navSectionOf } from "@/lib/nav-sections";
import { fetchAdminBatchHealth } from "@/lib/admin";
import { useAuth } from "@/components/providers/auth-provider";
import { useShellMode } from "@/components/providers/shell-mode-provider";
import { cn } from "@/lib/utils";

/*
 * 메뉴 정의. `rail`은 **접힌 레일에서 아이콘 밑에 적을 이름**이다.
 *
 * # 레일에서 그룹 이름을 뺀 뒤의 규칙 (2026-09-03 사용자 결정)
 *
 * 예전에는 레일에도 그룹 이름(순위/뉴스/실적/…)이 남아 있었고, 그것이 「국내」·「해외」가
 * 뉴스인지 실적인지 가르는 **유일한 단서**였다. 그래서 하위 이름을 두 글자로 줄여 쓸 수
 * 있었다. 지금은 그 줄이 사라졌으므로 **하위 이름 자체가 스스로를 설명해야 한다** —
 * 「국내」가 아니라 「국내 뉴스」, 「차트」가 아니라 「실시간 차트」다.
 *
 * 길어진 이름은 레일에서 **두 줄까지 접힌다**(`line-clamp-2`). 잘라 쓰면 「국내 뉴…」가
 * 되어 옆 항목과 구분이 안 되는데, 이 칸에서 구분이 안 되는 것은 곧 못 쓰는 메뉴다.
 * 「실시간 차트」는 한 줄에 들어간다 — 레일 폭을 그 글자에 맞춰 72px로 잡았다
 * (`--sidebar-collapsed-width` 주석, 2026-09-04 사용자 요구).
 *
 * 2026-09-04부터는 **관리 그룹까지 포함해 레일에 그룹 이름이 하나도 없다**
 * (사용자 결정 — 아래 `<p>`의 주석 참조).
 *
 * 전체 이름은 툴팁(`title`)과 펼친 상태가 그대로 들고 있다.
 */
const navGroups = [
  {
    /*
     * 2026-10-10 사용자 결정 A — 12칸 → 7칸. 「뉴스」·「실적」은 국내/해외(+서프라이즈)를 한 칸에 묶고,
     * 화면 위 탭 줄(`SectionTabs`)이 그 안을 오간다. 주소는 그대로다(각자 색인됨). 묶음 정의는 `lib/nav-sections.ts` 한 곳.
     *
     * 2026-10-12 사용자 요청 3 — 「종목 찾기」를 AI 점수 순위·종목 스크리너 두 칸으로 나누고, 업종·스타일·오늘의
     * 변화는 메뉴에서 보류. 「AI 추천 랭킹」으로 되돌리지 않는 이유는 10-10 사업화 검토 D8·S5(「추천」 표현).
     */
    label: "시장",
    items: [
      { href: "/market", label: "실시간 차트", icon: LineChart, rail: "실시간 차트" },
      { href: "/analysis", label: "AI 점수 순위", icon: Trophy, rail: "AI 점수 순위" },
      { href: "/screener", label: "종목 스크리너", icon: ScanSearch, rail: "스크리너" },
      { href: "/news/domestic", label: "뉴스", icon: Newspaper, rail: "뉴스", section: "news" },
      { href: "/earnings/domestic", label: "실적", icon: CalendarClock, rail: "실적", section: "earnings" },
    ],
  },
  {
    label: "내 투자",
    items: [
      { href: "/holdings", label: "내 포트폴리오", icon: Briefcase, rail: "보유" },
      { href: "/watchlist", label: "관심 목록", icon: Heart, rail: "관심" },
      /*
       * 「목표가 예측」 = 옛 `스탬프` + `목표가 관리`.
       *
       * 두 메뉴는 같은 물건의 두 반쪽이었다 — 목표가는 「얼마까지」만 있어 채점이
       * 안 됐고, 스탬프는 손절가·2차 목표가 없어 매매 계획이 못 됐다. 한 줄로
       * 합치면서 사이드바에서도 한 자리만 쓴다. 옛 두 경로(`/stamps`·`/targets`)는
       * 살아 있지만 메뉴에서는 내렸다 — 같은 기록이 세 군데에 나오면 어느 것이
       * 진짜인지 사용자가 판단해야 한다.
       */
      { href: "/predictions", label: "목표가 예측", icon: Crosshair, rail: "목표가 예측" },
      { href: "/journal", label: "메모장", icon: NotepadText, rail: "메모장" },
    ],
  },
  {
    /*
     * 부동산 (2026-10-01 사용자 결정 — 순자산은 「내 포트폴리오」로 합쳤다). 매물이 아니라 신고된 실거래,
     * 공매는 캠코 온비드.
     */
    label: "부동산",
    items: [
      { href: "/real-estate", label: "부동산 시세", icon: Building2, rail: "부동산" },
      { href: "/real-estate/auctions", label: "부동산 공매", icon: Gavel, rail: "공매" },
    ],
  },
  {
    // 리포트는 공개 발행물이라 사적 기록과 그룹을 분리한다.
    // 순위표(리더보드)는 두지 않는다 — 작성자를 줄 세우면 리포트가 대회
    // 참가작이 되고, 대회는 보상 없이는 유지되지 않는다.
    label: "커뮤니티",
    // 커뮤니티(2026-09-27 사용자 요청 4)는 투자 리포트 **위**. 뱃지는 활동·습관만이고
    // 순위·보상은 없다 — 리포트와 같은 방침.
    items: [
      { href: "/community", label: "커뮤니티", icon: MessagesSquare, rail: "커뮤니티" },
      { href: "/reports", label: "투자 리포트", icon: PenLine, rail: "리포트" },
    ],
  },
  {
    /*
     * 금융상품 (2026-09-29 사용자 결정 — 3항목 + 페이지 안 탭). 예금·적금, 신용·주담대·전세는
     * 각 화면의 탭이다. 주식 메뉴와 섞지 않으려고 커뮤니티 아래 독립 그룹으로 둔다.
     * 파킹통장은 공식 공시 API가 없어 넣지 않았다.
     */
    // 2026-10-09 사용자 결정 A — 「비교」가 아니라 정보 + AI 분석. 금융 계산기는 없앴다(주소는 예·적금 분석으로).
    label: "금융상품 분석",
    items: [
      { href: "/finance/savings", label: "예·적금 분석", icon: PiggyBank, rail: "예·적금" },
      { href: "/finance/loans", label: "대출 분석", icon: HandCoins, rail: "대출" },
      { href: "/finance/rates", label: "금리 동향", icon: Percent, rail: "금리" },
    ],
  },
  {
    // 그룹 이름이 레일에서 사라졌으므로 하위 이름이 스스로를 설명한다 — 앱 메뉴에
    // 적용한 규칙(위 머리글)을 관리자 메뉴에도 그대로 편다. 「뉴스」 하나만 있으면
    // 위쪽 「국내 뉴스」·「해외 뉴스」와 구분이 안 된다.
    label: "관리",
    items: [
      { href: "/admin", label: "뉴스 관리", icon: Shield, rail: "뉴스 관리" },
      { href: "/admin/api", label: "API 관리", icon: Gauge, rail: "API 관리" },
      { href: "/admin/llm", label: "LLM 모델 관리", icon: Sparkles, rail: "LLM 관리" },
      { href: "/admin/score", label: "점수 관리", icon: Calculator, rail: "점수 관리" },
      { href: "/admin/batch", label: "배치 관리", icon: ListChecks, rail: "배치 관리" },
      { href: "/admin/aliases", label: "별칭 관리", icon: Tags, rail: "별칭 관리" },
      { href: "/admin/seo", label: "화면 메타 관리", icon: Search, rail: "메타 관리" },
      { href: "/admin/growth", label: "검색 유입 보고서", icon: TrendingUp, rail: "유입 보고서" },
      { href: "/admin/community", label: "커뮤니티 신고 관리", icon: MessageSquareWarning, rail: "신고 관리" },
      { href: "/admin/cache", label: "캐시 관리", icon: Database, rail: "캐시 관리" },
      { href: "/admin/database", label: "데이터베이스 관리", icon: Table2, rail: "DB 관리" },
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
  // 묶인 화면(뉴스·실적)은 그 묶음의 사이드바 칸이 켜진다.
  const sectionId = navSectionOf(pathname)?.section.id;
  if (sectionId) {
    const item = navGroups.flatMap((g) => g.items).find((i) => "section" in i && i.section === sectionId);
    if (item) return item.href;
  }
  let best = "";
  for (const href of allNavHrefs) {
    if (pathname !== href && !pathname.startsWith(`${href}/`)) continue;
    if (href.length > best.length) best = href;
  }
  return best;
}

type SidebarNavProps = {
  onNavigate?: () => void;
};

/*
 * 접힘/펼침 스타일은 `collapsed` prop 대신 조상의 `data-sidebar-state`로만 분기한다.
 * 데스크톱 레일(접힘)과 모바일 드로어(항상 펼침)가 동시에 DOM에 있을 때, 자식마다
 * cn(collapsed ? … : …)을 쓰면 같은 태그에 서로 다른 className이 박혀 hydration이
 * 깨진다. 자식 className은 고정하고 group-data로만 모양을 바꾼다.
 */
/*
 * 배치 감시 뱃지 — 「배치 관리」 항목에 붙는 숫자.
 *
 * 콘솔 배너만으로는 **그 화면을 연 사람에게만** 닿는다. 실제로 놓친 사고는 아무도
 * 그 화면을 열지 않아서 생겼으므로, 관리자가 어느 화면에 있든 보이는 자리가 하나는
 * 있어야 한다.
 *
 * 관리자에게만, 5분마다. 실패는 삼킨다 — 뱃지 하나 때문에 사이드바가 시끄러워질
 * 이유가 없고, 값이 없으면 그냥 안 그린다.
 */
const BATCH_ALERT_POLL_MS = 5 * 60 * 1000;

function useBatchAlertCount(session: ReturnType<typeof useAuth>["session"]): number {
  const [count, setCount] = useState(0);
  const isAdmin = isAdminSession(session);

  useEffect(() => {
    // 관리자가 아니면 아무것도 하지 않는다 — 「배치 관리」 항목 자체가
    // isAdminNavVisible에서 이미 걸러지므로 남은 값이 화면에 나갈 일이 없다.
    if (!isAdmin) return;
    let cancelled = false;
    const load = async () => {
      try {
        const health = await fetchAdminBatchHealth();
        if (cancelled) return;
        // 감시 자체가 멈춘 것도 한 건으로 센다 — 그때의 「0건」은 정상이라는 뜻이
        // 아니라 아무도 안 보고 있다는 뜻이다.
        const unhealthy = health?.report?.unhealthy ?? 0;
        setCount(unhealthy + (health?.watchdogStale ? 1 : 0));
      } catch {
        if (!cancelled) setCount(0);
      }
    };
    void load();
    const timer = setInterval(() => void load(), BATCH_ALERT_POLL_MS);
    return () => {
      cancelled = true;
      clearInterval(timer);
    };
  }, [isAdmin]);

  return count;
}

function SidebarNav({ onNavigate }: SidebarNavProps) {
  const pathname = usePathname();
  const { session } = useAuth();
  const activeHref = activeNavHref(pathname);
  const batchAlerts = useBatchAlertCount(session);

  return (
    <nav
      className={cn(
        "flex-1 overflow-y-auto py-3",
        "group-data-[sidebar-state=collapsed]/sidebar:px-1 group-data-[sidebar-state=open]/sidebar:px-2",
      )}
    >
      {navGroups
        .map((group) => ({
          ...group,
          // 노출을 끈 기능(lib/hidden-features)은 관리자에게도 메뉴에서 뺀다.
          items: group.items.filter(
            (item) => isAdminNavVisible(item.href, session) && !isHiddenRoute(item.href),
          ),
        }))
        .filter((group) => group.items.length > 0)
        .map((group) => (
          <div
            key={group.label}
            className="mb-4 group-data-[sidebar-state=collapsed]/sidebar:mb-3"
          >
            {/*
              접힌 레일에서는 그룹 이름을 **하나도 남기지 않는다**
              (2026-09-04 사용자 결정: 「상위 메뉴명은 모두 제거」).

              2026-09-03에 앱 메뉴의 그룹 이름을 먼저 걷었고, 관리 그룹만
              `railLabel`로 남겨 뒀다 — 하위가 「뉴스」·「API」처럼 짧은 관리자 용어라
              그 줄이 「무엇의 관리인가」를 대신 말한다고 봤기 때문이다. 그런데 그
              한 줄만 남으니 레일 중간에 홀로 선 제목이 되어, 걷어낸 이유(세로 공간과
              시선의 단순함)를 그 자리에서 되돌리고 있었다. 관리자 메뉴가 무엇인지는
              방패·게이지 같은 아이콘과 툴팁이 말한다.

              전체 이름은 펼친 상태와 `title` 툴팁이 그대로 들고 있다.
            */}
            <p
              className={cn(
                "font-semibold text-muted-foreground",
                "group-data-[sidebar-state=open]/sidebar:mb-1.5 group-data-[sidebar-state=open]/sidebar:block group-data-[sidebar-state=open]/sidebar:whitespace-nowrap group-data-[sidebar-state=open]/sidebar:px-1.5 group-data-[sidebar-state=open]/sidebar:text-micro group-data-[sidebar-state=open]/sidebar:uppercase group-data-[sidebar-state=open]/sidebar:tracking-wider",
                "group-data-[sidebar-state=collapsed]/sidebar:hidden",
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
                      aria-label={item.label}
                      className={cn(
                        "flex rounded-lg text-sm transition-colors",
                        "group-data-[sidebar-state=collapsed]/sidebar:flex-col group-data-[sidebar-state=collapsed]/sidebar:items-center group-data-[sidebar-state=collapsed]/sidebar:gap-0.5 group-data-[sidebar-state=collapsed]/sidebar:px-0.5 group-data-[sidebar-state=collapsed]/sidebar:py-1.5",
                        "group-data-[sidebar-state=open]/sidebar:items-center group-data-[sidebar-state=open]/sidebar:gap-2 group-data-[sidebar-state=open]/sidebar:whitespace-nowrap group-data-[sidebar-state=open]/sidebar:px-2 group-data-[sidebar-state=open]/sidebar:py-1.5",
                        active
                          ? "bg-sidebar-accent font-medium text-sidebar-accent-foreground"
                          : "text-muted-foreground hover:bg-sidebar-accent/60 hover:text-sidebar-accent-foreground",
                      )}
                    >
                      <Icon className="h-4 w-4 shrink-0" />
                      {/*
                        레일 이름은 **두 줄까지** 접는다. 그룹 이름이 사라진 뒤로 이
                        글자가 메뉴를 가르는 유일한 단서라, 한 줄로 잘라 「국내 뉴…」가
                        되면 옆 항목과 구분이 안 된다. `break-keep`은 한국어를 낱말
                        단위로 끊어 「실시간 차트」가 「실시간 / 차트」로 나뉘게 한다
                        (기본값은 글자 아무 데서나 끊어 「실시간 차 / 트」가 된다).
                      */}
                      <span
                        className={cn(
                          "line-clamp-2 w-full break-keep text-center text-[10px] leading-tight",
                          "group-data-[sidebar-state=open]/sidebar:hidden",
                        )}
                      >
                        {item.rail}
                      </span>
                      <span className="hidden group-data-[sidebar-state=open]/sidebar:inline">
                        {item.label}
                      </span>
                      {item.href === "/admin/batch" && batchAlerts > 0 && (
                        <span
                          aria-label={`제때 돌지 않은 배치 ${batchAlerts}개`}
                          className={cn(
                            "ml-auto shrink-0 rounded-full bg-[var(--danger)] px-1.5 text-[10px] font-semibold leading-[18px] text-white",
                            // 접힌 레일에서는 글자 옆 자리가 없다 — 아이콘 아래에 붙인다.
                            "group-data-[sidebar-state=collapsed]/sidebar:ml-0",
                          )}
                        >
                          {batchAlerts}
                        </span>
                      )}
                    </Link>
                  </li>
                );
              })}
            </ul>
          </div>
        ))}
      {/*
        푸터 대신(이 앱에는 푸터가 없다) — 약관 두 페이지가 어디서도 링크되지 않던 고아였다
        (2026-10-09 색인 점검). 접힌 레일에서는 자리가 없어 숨기고, 링크 자체는 HTML에 남는다.
      */}
      <p className="mt-4 px-2 text-micro text-[var(--text-muted)] group-data-[sidebar-state=collapsed]/sidebar:hidden">
        <Link href="/legal/terms" prefetch={false} onClick={onNavigate} className="hover:underline">
          이용약관
        </Link>
        {" · "}
        <Link href="/legal/privacy" prefetch={false} onClick={onNavigate} className="hover:underline">
          개인정보처리방침
        </Link>
      </p>
    </nav>
  );
}

type SidebarProps = {
  open: boolean;
  onClose: () => void;
};

export function Sidebar({ open, onClose }: SidebarProps) {
  const { isCompact } = useShellMode();
  const [mounted, setMounted] = useState(false);

  useEffect(() => {
    setMounted(true);
  }, []);

  const handleNavigate = () => {
    if (window.matchMedia("(max-width: 1023px)").matches) {
      onClose();
    }
  };

  /*
   * SSR·첫 페인트에는 모바일 드로어를 그대로 둔다(모바일에서 lg:hidden aside가 유일한
   * 보이는 네비). hydration이 끝난 뒤 데스크톱이면 드로어 트리를 걷어 중복 nav를 없앤다.
   */
  const showMobileDrawer = !mounted || isCompact;

  return (
    <>
      <div
        className={cn(
          "group/sidebar hidden h-full shrink-0 flex-col border-r border-sidebar-border bg-sidebar text-sidebar-foreground transition-[width] duration-300 ease-in-out lg:flex",
          open ? "w-[var(--sidebar-width)]" : "w-[var(--sidebar-collapsed-width)]",
        )}
        data-sidebar-state={open ? "open" : "collapsed"}
      >
        <div className="flex min-h-0 flex-1 flex-col overflow-hidden">
          <SidebarNav />
        </div>
      </div>

      {showMobileDrawer && open && (
        <button
          type="button"
          aria-label="메뉴 닫기"
          className="fixed inset-0 z-40 bg-black/50 lg:hidden"
          onClick={onClose}
        />
      )}

      {showMobileDrawer && (
        <aside
          className={cn(
            "group/sidebar fixed inset-y-0 left-0 z-50 flex w-[var(--sidebar-width)] min-w-[var(--sidebar-width)] flex-col border-r border-sidebar-border bg-sidebar text-sidebar-foreground transition-transform duration-300 ease-in-out lg:hidden",
            open ? "translate-x-0" : "-translate-x-full",
          )}
          data-sidebar-state="open"
        >
          <div className="flex h-14 shrink-0 items-center border-b border-sidebar-border px-4">
            <span className="whitespace-nowrap text-sm font-bold tracking-[0.18em] text-sidebar-foreground">
              QUANT PORT
            </span>
          </div>
          <SidebarNav onNavigate={handleNavigate} />
        </aside>
      )}
    </>
  );
}
