"use client";

import { useCallback, useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { Bell, Building2, LineChart, PiggyBank } from "lucide-react";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { isPageVisible } from "@/hooks/use-page-visible";
import { formatRelativeTime } from "@/lib/format";
import {
  fetchNotifications,
  fetchUnreadNotifications,
  markNotificationsRead,
  type AppNotification,
} from "@/lib/notifications-api";
import { cn } from "@/lib/utils";

const KIND_ICON = { stock: LineChart, re_trade: Building2, fin_rate: PiggyBank } as const;

/** 읽지 않은 수는 3분마다(화면이 보일 때만) — 알림은 30분 주기로 생기므로 이보다 자주 볼 일이 없다. */
const POLL_MS = 3 * 60 * 1000;

/**
 * 헤더 종 아이콘 — 관심 목록(종목·단지·금융상품)의 변화 (2026-10-10 사용자 요청 4: 웹 푸시가 아니라 앱 안 알림 칸).
 * 숫자는 가벼운 `/notifications/unread`로, 목록은 열 때만 받는다. 누르면 그 줄을 읽음으로 하고 해당 화면으로 간다.
 */
export function NotificationBell({ compact }: { compact?: boolean }) {
  const router = useRouter();
  const [unread, setUnread] = useState(0);
  const [items, setItems] = useState<AppNotification[] | null>(null);

  const refreshCount = useCallback(() => {
    if (!isPageVisible()) return;
    fetchUnreadNotifications()
      .then(setUnread)
      .catch(() => {});
  }, []);

  useEffect(() => {
    const first = setTimeout(refreshCount, 0);
    const t = setInterval(refreshCount, POLL_MS);
    document.addEventListener("visibilitychange", refreshCount);
    return () => {
      clearTimeout(first);
      clearInterval(t);
      document.removeEventListener("visibilitychange", refreshCount);
    };
  }, [refreshCount]);

  const onOpenChange = (open: boolean) => {
    if (!open) return;
    fetchNotifications()
      .then((d) => {
        setItems(d.items);
        setUnread(d.unread);
      })
      .catch(() => setItems([]));
  };

  const open = (n: AppNotification) => {
    if (!n.read) {
      setUnread((u) => Math.max(0, u - 1));
      setItems((prev) => prev?.map((x) => (x.id === n.id ? { ...x, read: true } : x)) ?? prev);
      void markNotificationsRead([n.id]).catch(() => {});
    }
    if (n.href) router.push(n.href);
  };

  const readAll = () => {
    setUnread(0);
    setItems((prev) => prev?.map((x) => ({ ...x, read: true })) ?? prev);
    void markNotificationsRead().catch(() => {});
  };

  return (
    <DropdownMenu onOpenChange={onOpenChange}>
      <DropdownMenuTrigger
        className={cn(
          "relative flex shrink-0 cursor-pointer items-center justify-center rounded-full text-[var(--text-secondary)] outline-none hover:bg-muted focus-visible:ring-[3px] focus-visible:ring-ring/50",
          compact ? "size-7" : "size-8",
        )}
        aria-label={unread > 0 ? `알림 ${unread}개 읽지 않음` : "알림"}
      >
        <Bell className={compact ? "size-4" : "size-[18px]"} />
        {unread > 0 && (
          <span className="absolute -right-0.5 -top-0.5 min-w-4 rounded-full bg-[var(--loss)] px-1 text-center text-[10px] font-bold leading-4 text-white">
            {unread > 99 ? "99+" : unread}
          </span>
        )}
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-80 p-0">
        <DropdownMenuLabel className="flex items-center justify-between px-3 py-2">
          <span className="text-cell font-semibold">알림</span>
          {unread > 0 && (
            <button type="button" onClick={readAll} className="text-micro font-normal text-[var(--brand)] hover:underline">
              모두 읽음
            </button>
          )}
        </DropdownMenuLabel>
        <DropdownMenuSeparator className="my-0" />
        <div className="max-h-[60vh] overflow-y-auto">
          {items === null ? (
            <p className="px-3 py-4 text-cell text-[var(--text-muted)]">불러오는 중…</p>
          ) : items.length === 0 ? (
            <p className="px-3 py-4 text-cell leading-relaxed text-[var(--text-muted)]">
              관심 목록의 종목·단지·금융상품에 변화가 생기면 여기에 알려 드립니다.
            </p>
          ) : (
            items.map((n) => {
              const Icon = KIND_ICON[n.kind] ?? Bell;
              return (
                <DropdownMenuItem
                  key={n.id}
                  onSelect={() => open(n)}
                  className={cn("flex items-start gap-2 rounded-none px-3 py-2.5", !n.read && "bg-[var(--brand)]/5")}
                >
                  <Icon className="mt-0.5 size-4 shrink-0 text-[var(--text-muted)]" />
                  <span className="min-w-0 flex-1">
                    <span className={cn("block text-cell leading-snug", n.read ? "text-[var(--text-secondary)]" : "font-semibold text-[var(--text-primary)]")}>
                      {n.title}
                    </span>
                    <span className="mt-0.5 block text-micro text-[var(--text-muted)]">
                      {n.body ? `${n.body} · ` : ""}
                      <span suppressHydrationWarning>{formatRelativeTime(n.createdAt)}</span>
                    </span>
                  </span>
                  {!n.read && <span className="mt-1.5 size-1.5 shrink-0 rounded-full bg-[var(--brand)]" aria-label="읽지 않음" />}
                </DropdownMenuItem>
              );
            })
          )}
        </div>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
