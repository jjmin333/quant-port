import { cn } from "@/lib/utils";

/**
 * Layout archetypes. Every page centers its content the same way; only the
 * max-width differs.
 *
 * - `wide` — **기본값**. 화면 폭을 다 쓴다. 08-11 사용자 결정: 모바일 반응형이 따로
 *   만들어져 있으므로 데스크탑에서 중앙 컬럼만 쓰던 것을 그만두고 데이터를 더 보여준다.
 * - `reading` — 긴 산문을 읽는 화면(리포트 본문·작성)만 명시적으로 선택한다.
 * - `wide` — 터미널형 페이지(마켓 레이더 · 시세 · 종목 워크스페이스). 리스트/호가/차트가
 *   동시에 자리를 요구해 reading 컬럼에 넣으면 차트가 압살된다. 초광폭에서만 잘린다.
 *
 * A page and its `PageHeader` must pass the same value, or the header column and
 * the body column start at different x positions.
 */
export type PageShellWidth = "reading" | "wide";

/** Centering rule for a content column. Shared by `PageHeader` and page bodies. */
export function pageShellClass(width: PageShellWidth) {
  return width === "wide" ? "page-shell-wide" : "page-shell";
}

type PageScrollShellProps = {
  children: React.ReactNode;
  className?: string;
  contentClassName?: string;
  width?: PageShellWidth;
  /** Page already wraps with a page-shell column */
  bare?: boolean;
};

/** Mobile/webview-friendly scroll region with centered content column. */
export function PageScrollShell({
  children,
  className,
  contentClassName,
  width = "wide",
  bare = false,
}: PageScrollShellProps) {
  return (
    <main
      className={cn(
        "flex-1 overflow-y-auto overflow-x-hidden bg-[var(--background)]",
        className,
      )}
    >
      {bare ? (
        children
      ) : (
        <div
          className={cn(
            pageShellClass(width),
            "min-w-0 py-4 sm:py-6",
            contentClassName,
          )}
        >
          {children}
        </div>
      )}
    </main>
  );
}
