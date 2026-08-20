"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { X } from "lucide-react";
import { PageHeader } from "@/components/layout/header";
import { PageScrollShell } from "@/components/layout/page-content-shell";
import { GeneralSettings } from "@/components/settings/general-settings";
import { useShellMode } from "@/components/providers/shell-mode-provider";
import { SETTINGS_NAV, type SettingsTab } from "@/lib/settings-config";
import { cn } from "@/lib/utils";

function SettingsPlaceholder({ title }: { title: string }) {
  return (
    <div className="flex h-48 items-center justify-center rounded-xl border border-dashed border-[var(--border)] text-sm text-[var(--text-muted)]">
      {title} 설정은 준비 중입니다.
    </div>
  );
}

function SettingsTabContent({ activeTab }: { activeTab: SettingsTab }) {
  if (activeTab === "general") return <GeneralSettings />;
  if (activeTab === "notification") return <SettingsPlaceholder title="알림" />;
  if (activeTab === "community") return <SettingsPlaceholder title="커뮤니티" />;
  return <SettingsPlaceholder title="단축키" />;
}

export function SettingsPanel() {
  const router = useRouter();
  const { isCompact } = useShellMode();
  const [activeTab, setActiveTab] = useState<SettingsTab>("general");

  const activeLabel = SETTINGS_NAV.find((item) => item.id === activeTab)?.label ?? "일반";
  const onClose = () => router.back();

  if (isCompact) {
    return (
      <>
        <PageHeader title="설정" subtitle="앱 · 알림" />
        <PageScrollShell bare>
          <div className="page-shell-wide min-w-0 space-y-4 py-4">
            <div className="flex gap-1 overflow-x-auto pb-1">
              {SETTINGS_NAV.map((item) => (
                <button
                  key={item.id}
                  type="button"
                  onClick={() => setActiveTab(item.id)}
                  className={cn(
                    "shrink-0 rounded-lg px-3 py-1.5 text-xs font-medium transition-colors",
                    activeTab === item.id
                      ? "bg-[var(--brand-muted)] text-[var(--brand)]"
                      : "text-[var(--text-muted)] hover:text-[var(--text-secondary)]",
                  )}
                >
                  {item.label}
                </button>
              ))}
            </div>
            <div>
              <h2 className="mb-4 text-lg font-semibold text-[var(--text-primary)]">{activeLabel}</h2>
              <SettingsTabContent activeTab={activeTab} />
            </div>
          </div>
        </PageScrollShell>
      </>
    );
  }

  return (
    <div className="fixed inset-0 z-[60] flex items-center justify-center bg-black/60 p-4">
      <div className="flex h-[min(640px,90vh)] w-full max-w-4xl min-w-0 shrink-0 overflow-hidden rounded-2xl border border-[var(--border)] bg-[var(--surface)] shadow-2xl">
        <nav className="flex w-44 shrink-0 flex-col border-r border-[var(--border)] bg-[var(--background)] py-4">
          {SETTINGS_NAV.map((item) => (
            <button
              key={item.id}
              type="button"
              onClick={() => setActiveTab(item.id)}
              className={cn(
                "whitespace-nowrap px-5 py-2.5 text-left text-sm transition-colors",
                activeTab === item.id
                  ? "bg-[var(--surface-elevated)] font-medium text-[var(--text-primary)]"
                  : "text-[var(--text-secondary)] hover:bg-[var(--surface)] hover:text-[var(--text-primary)]",
              )}
            >
              {item.label}
            </button>
          ))}
        </nav>

        <div className="flex min-w-0 flex-1 flex-col">
          <div className="flex shrink-0 items-center justify-between border-b border-[var(--border)] px-6 py-4">
            <h2 className="text-xl font-semibold text-[var(--text-primary)]">{activeLabel}</h2>
            <button
              type="button"
              onClick={onClose}
              className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg text-[var(--text-secondary)] hover:bg-[var(--surface-elevated)] hover:text-[var(--text-primary)]"
              aria-label="설정 닫기"
            >
              <X className="h-5 w-5" />
            </button>
          </div>

          <div className="flex-1 overflow-y-auto overflow-x-hidden p-6">
            <SettingsTabContent activeTab={activeTab} />
          </div>
        </div>
      </div>
    </div>
  );
}
