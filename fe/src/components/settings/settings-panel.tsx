"use client";

import { useRouter } from "next/navigation";
import { X } from "lucide-react";
import { PageScrollShell } from "@/components/layout/page-content-shell";
import { AccountSettings } from "@/components/settings/account-settings";
import { useShellMode } from "@/components/providers/shell-mode-provider";

export function SettingsPanel() {
  const router = useRouter();
  const { isCompact } = useShellMode();
  const onClose = () => router.back();

  if (isCompact) {
    return (
      <>
        <PageScrollShell bare>
          <div className="page-shell-wide min-w-0 py-4">
            <AccountSettings />
          </div>
        </PageScrollShell>
      </>
    );
  }

  return (
    <div className="fixed inset-0 z-[60] flex items-center justify-center bg-black/60 p-4">
      <div className="flex h-[min(640px,90vh)] w-full max-w-2xl min-w-0 shrink-0 flex-col overflow-hidden rounded-2xl border border-[var(--border)] bg-[var(--surface)] shadow-2xl">
        <div className="flex shrink-0 items-center justify-between border-b border-[var(--border)] px-6 py-4">
          <div>
            <h2 className="text-xl font-semibold text-[var(--text-primary)]">계정</h2>
            <p className="mt-0.5 text-cell text-[var(--text-muted)]">표시 이름 · 비밀번호 · 회원 탈퇴</p>
          </div>
          <button
            type="button"
            onClick={onClose}
            className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg text-[var(--text-secondary)] hover:bg-[var(--surface-elevated)] hover:text-[var(--text-primary)]"
            aria-label="계정 설정 닫기"
          >
            <X className="h-5 w-5" />
          </button>
        </div>

        <div className="flex-1 overflow-y-auto overflow-x-hidden p-6">
          <AccountSettings />
        </div>
      </div>
    </div>
  );
}
