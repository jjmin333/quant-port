"use client";

import Link from "next/link";
import { PenLine } from "lucide-react";
import { cn } from "@/lib/utils";

/**
 * Entry point into the compose screen from a stock page.
 *
 * It is a plain link, not a modal: publishing a report is a deliberate act with
 * frozen consequences, unlike the stamp button next to it, which saves on the
 * first click. The two live side by side on purpose — a stamp is a private
 * moment, a report is a public claim.
 */
export function PublishCallButton({
  symbol,
  className,
  compact = false,
}: {
  symbol: string;
  className?: string;
  compact?: boolean;
}) {
  return (
    <Link
      href={`/reports/new?symbol=${encodeURIComponent(symbol)}`}
      title="이 종목으로 투자 리포트 작성"
      className={cn(
        "inline-flex shrink-0 items-center gap-1 rounded-lg border border-[var(--border)] px-2 py-1 text-[11px] text-[var(--text-secondary)] hover:bg-[var(--surface-elevated)] hover:text-[var(--text-primary)]",
        className,
      )}
    >
      <PenLine className="h-3.5 w-3.5" />
      {compact ? "리포트" : "리포트 작성"}
    </Link>
  );
}
