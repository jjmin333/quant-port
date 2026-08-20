"use client";

import { useEffect, useState } from "react";
import { Menu } from "lucide-react";
import { cn } from "@/lib/utils";

type SidebarToggleProps = {
  open: boolean;
  onToggle: () => void;
};

export function SidebarToggle({ open, onToggle }: SidebarToggleProps) {
  // 서버·첫 페인트는 false, 이펙트 뒤 localStorage·화면 폭으로 open이 바뀐다.
  // aria-label까지 즉시 바꾸면 hydration mismatch가 난다.
  const [mounted, setMounted] = useState(false);
  useEffect(() => setMounted(true), []);

  const label = !mounted ? "사이드바" : open ? "사이드바 닫기" : "사이드바 열기";

  return (
    <button
      type="button"
      onClick={onToggle}
      className={cn(
        "flex h-9 w-9 shrink-0 items-center justify-center rounded-md text-[var(--text-secondary)] transition-colors hover:text-[var(--text-primary)]",
      )}
      aria-label={label}
      aria-expanded={mounted ? open : false}
      suppressHydrationWarning
    >
      <Menu className="h-5 w-5" strokeWidth={1.75} />
    </button>
  );
}
