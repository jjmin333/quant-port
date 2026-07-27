"use client";

import { usePreferences } from "@/components/providers/preferences-provider";
import { COLOR_MODE_OPTIONS } from "@/lib/settings-config";
import { cn } from "@/lib/utils";

function ThemePreview({
  bg,
  surface,
  accent,
  className,
}: {
  bg: string;
  surface: string;
  accent: string;
  className?: string;
}) {
  return (
    <div
      className={cn("overflow-hidden rounded-md border border-black/10", className)}
      style={{ backgroundColor: bg }}
    >
      <div className="flex h-full gap-1 p-2">
        <div className="w-3 shrink-0 rounded-sm" style={{ backgroundColor: surface }} />
        <div className="flex flex-1 flex-col gap-1">
          <div className="h-2 rounded-sm" style={{ backgroundColor: surface }} />
          <div
            className="h-6 flex-1 rounded-sm"
            style={{ backgroundColor: accent, opacity: 0.85 }}
          />
        </div>
      </div>
    </div>
  );
}

function OptionCard({
  label,
  selected,
  onClick,
  children,
}: {
  label: string;
  selected: boolean;
  onClick: () => void;
  children: React.ReactNode;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={cn(
        "flex flex-col items-center gap-2 rounded-xl border-2 p-3 transition-colors",
        selected
          ? "border-[var(--accent)] bg-[var(--accent-muted)]"
          : "border-[var(--border)] bg-[var(--surface)] hover:border-[var(--text-muted)]",
      )}
    >
      <div className="w-full">{children}</div>
      <span
        className={cn(
          "whitespace-nowrap text-sm font-medium",
          selected ? "text-[var(--accent)]" : "text-[var(--text-secondary)]",
        )}
      >
        {label}
      </span>
    </button>
  );
}

export function GeneralSettings() {
  const { preferences, setColorMode, setCurrency } = usePreferences();

  return (
    <div className="space-y-8">
      <div>
        <h3 className="mb-4 text-base font-semibold text-[var(--text-primary)]">색상 모드</h3>
        <div className="grid max-w-xl grid-cols-3 gap-3">
          {COLOR_MODE_OPTIONS.map((option) => (
            <OptionCard
              key={option.value}
              label={option.label}
              selected={preferences.colorMode === option.value}
              onClick={() => setColorMode(option.value)}
            >
              <ThemePreview
                bg={option.preview.bg}
                surface={option.preview.surface}
                accent={option.preview.accent}
                className="h-16 w-full"
              />
            </OptionCard>
          ))}
        </div>
      </div>

      <div className="border-t border-[var(--border)] pt-6">
        <div className="flex items-center justify-between gap-4">
          <h3 className="whitespace-nowrap text-base font-semibold text-[var(--text-primary)]">
            기본 통화
          </h3>
          <div className="flex shrink-0 rounded-lg border border-[var(--border)] bg-[var(--surface)] p-1">
            {(["USD", "KRW"] as const).map((currency) => (
              <button
                key={currency}
                type="button"
                onClick={() => setCurrency(currency)}
                className={cn(
                  "min-w-[3rem] rounded-md px-4 py-1.5 text-sm font-medium transition-colors",
                  preferences.currency === currency
                    ? "bg-[var(--accent-muted)] text-[var(--accent)]"
                    : "text-[var(--text-secondary)] hover:text-[var(--text-primary)]",
                )}
              >
                {currency === "USD" ? "$" : "원"}
              </button>
            ))}
          </div>
        </div>
      </div>
    </div>
  );
}
