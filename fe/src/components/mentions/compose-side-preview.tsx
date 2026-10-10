"use client";

import { useEffect, useState, useSyncExternalStore, type ReactNode } from "react";
import { cn } from "@/lib/utils";

/**
 * 쓰는 화면 옆 미리보기 (2026-10-01 요청 4) — **넓은 화면(≥1280px)만** 오른쪽에 상시 띄운다.
 * 좁은 화면은 편집기마다 있던 「미리보기」 토글 그대로다(두 칸이 들어갈 자리가 없다).
 *
 * 미리보기는 넓을 때만 **마운트**한다 — 숨긴 채로 두면 좁은 화면에서도 태그 해석 요청이 나간다.
 */
export function ComposeWithSidePreview({
  children,
  preview,
  className,
}: {
  children: ReactNode;
  /** 넓은 화면에서만 불린다. */
  preview: () => ReactNode;
  className?: string;
}) {
  const wide = useWideCompose();
  return (
    <div className={cn("xl:grid xl:grid-cols-[minmax(0,1fr)_minmax(0,1fr)] xl:items-start xl:gap-5", className)}>
      <div className="w-full min-w-0 space-y-4">{children}</div>
      {wide && (
        <aside aria-label="미리보기" className="sticky top-0 min-w-0">
          {preview()}
        </aside>
      )}
    </div>
  );
}

const WIDE_QUERY = "(min-width: 1280px)";

/** 옆 미리보기가 떠 있는 폭인가 — 편집기가 자기 토글을 숨길 때도 쓴다. */
export function useWideCompose(): boolean {
  return useSyncExternalStore(
    (cb) => {
      const m = window.matchMedia(WIDE_QUERY);
      m.addEventListener("change", cb);
      return () => m.removeEventListener("change", cb);
    },
    () => window.matchMedia(WIDE_QUERY).matches,
    () => false,
  );
}

/** 칠 때마다 서버 미리보기를 부르지 않게 — 멈춘 뒤 `ms` 지나서 값이 따라온다. */
export function useDebouncedValue<T>(value: T, ms = 400): T {
  const [v, setV] = useState(value);
  useEffect(() => {
    const t = window.setTimeout(() => setV(value), ms);
    return () => window.clearTimeout(t);
  }, [value, ms]);
  return v;
}
