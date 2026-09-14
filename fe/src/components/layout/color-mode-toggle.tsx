"use client";

import { useEffect, useState } from "react";
import { Moon, Sun } from "lucide-react";
import { usePreferences } from "@/components/providers/preferences-provider";
import { cn } from "@/lib/utils";

type ColorModeToggleProps = {
  compact?: boolean;
};

export function ColorModeToggle({ compact = false }: ColorModeToggleProps) {
  const { preferences, setColorMode } = usePreferences();
  const [mounted, setMounted] = useState(false);

  useEffect(() => setMounted(true), []);

  const isDark = preferences.colorMode === "dark";

  const toggle = () => setColorMode(isDark ? "light" : "dark");

  return (
    <button
      type="button"
      onClick={toggle}
      aria-label={mounted ? (isDark ? "라이트 모드로 전환" : "다크 모드로 전환") : "색상 모드 전환"}
      className={cn(
        "flex shrink-0 items-center justify-center rounded-full border transition-colors outline-none focus-visible:ring-[3px] focus-visible:ring-ring/50",
        compact ? "size-7" : "size-8",
        isDark
          ? "border-white/15 bg-zinc-800 text-amber-300 hover:bg-zinc-700"
          : "border-border bg-background text-foreground shadow-xs hover:bg-muted",
      )}
      suppressHydrationWarning
    >
      {mounted && isDark ? (
        <Sun className={cn(compact ? "size-3.5" : "size-4")} strokeWidth={1.75} />
      ) : (
        <Moon className={cn(compact ? "size-3.5" : "size-4")} strokeWidth={1.75} />
      )}
    </button>
  );
}
