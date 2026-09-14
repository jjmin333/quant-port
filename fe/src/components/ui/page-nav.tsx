"use client";

import { ChevronLeft, ChevronRight } from "lucide-react";
import { cn } from "@/lib/utils";

/**
 * 번호 페이지네이션. **한 곳에만 둔다** — 종목 뉴스 탭과 AI 분석 탭의 일별 목록이
 * 같은 모양이어야 한다는 요구였고, 두 벌로 두면 반드시 갈라진다.
 *
 * 목록 아래에 놓는 것이 전제다(뉴스 탭이 그렇게 쓰고 있었다). 위쪽 헤더에 두면
 * 한 페이지가 길 때 버튼이 화면 밖으로 밀린다.
 */

/** 현재 페이지 주변의 번호 창 (최대 5개). `1 … 4 5 6 … 12` 꼴. */
export function pageWindow(current: number, total: number): (number | "ellipsis")[] {
  if (total <= 7) return Array.from({ length: total }, (_, i) => i + 1);
  const pages = new Set<number>([1, total, current, current - 1, current + 1]);
  const sorted = Array.from(pages)
    .filter((p) => p >= 1 && p <= total)
    .sort((a, b) => a - b);
  const out: (number | "ellipsis")[] = [];
  sorted.forEach((p, i) => {
    if (i > 0 && p - sorted[i - 1] > 1) out.push("ellipsis");
    out.push(p);
  });
  return out;
}

export function PageNav({
  page,
  totalPages,
  onChange,
  label,
  className,
}: {
  /** 1부터 시작한다. */
  page: number;
  totalPages: number;
  onChange: (next: number) => void;
  /** 스크린리더용 이름 — "뉴스 페이지"처럼 무엇의 페이지인지 적는다. */
  label: string;
  className?: string;
}) {
  if (totalPages <= 1) return null;

  return (
    <nav
      aria-label={label}
      className={cn(
        "flex flex-wrap items-center justify-center gap-1 border-t border-[var(--border)] pt-3",
        className,
      )}
    >
      <button
        type="button"
        onClick={() => onChange(Math.max(1, page - 1))}
        disabled={page <= 1}
        aria-label="이전 페이지"
        className="flex h-7 w-7 items-center justify-center rounded-md text-[var(--text-secondary)] hover:bg-[var(--surface-elevated)] disabled:pointer-events-none disabled:opacity-30"
      >
        <ChevronLeft className="h-3.5 w-3.5" />
      </button>
      {pageWindow(page, totalPages).map((p, i) =>
        p === "ellipsis" ? (
          <span
            key={`ellipsis-${i}`}
            className="flex h-7 w-7 items-center justify-center text-cell text-[var(--text-muted)]"
          >
            …
          </span>
        ) : (
          <button
            key={p}
            type="button"
            onClick={() => onChange(p)}
            aria-current={p === page ? "page" : undefined}
            className={cn(
              "flex h-7 min-w-7 items-center justify-center rounded-md px-1.5 text-cell font-medium tabular-nums",
              p === page
                ? "bg-[var(--brand)] text-[var(--brand-foreground)]"
                : "text-[var(--text-secondary)] hover:bg-[var(--surface-elevated)]",
            )}
          >
            {p}
          </button>
        ),
      )}
      <button
        type="button"
        onClick={() => onChange(Math.min(totalPages, page + 1))}
        disabled={page >= totalPages}
        aria-label="다음 페이지"
        className="flex h-7 w-7 items-center justify-center rounded-md text-[var(--text-secondary)] hover:bg-[var(--surface-elevated)] disabled:pointer-events-none disabled:opacity-30"
      >
        <ChevronRight className="h-3.5 w-3.5" />
      </button>
    </nav>
  );
}
