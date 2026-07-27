"use client";

import { Menu } from "lucide-react";
import { cn } from "@/lib/utils";

type SidebarToggleProps = {
  open: boolean;
  onToggle: () => void;
};

export function SidebarToggle({ open, onToggle }: SidebarToggleProps) {
  return (
    <button
      type="button"
      onClick={onToggle}
      className={cn(
        "flex h-9 w-9 shrink-0 items-center justify-center rounded-md text-[var(--text-secondary)] transition-colors hover:text-[var(--text-primary)]",
      )}
      aria-label={open ? "사이드바 닫기" : "사이드바 열기"}
      aria-expanded={open}
    >
      <Menu className="h-5 w-5" strokeWidth={1.75} />
    </button>
  );
}
