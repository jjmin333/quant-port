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

/**
 * 본문 최상단의 동작 줄.
 *
 * 2026-09-02에 페이지 제목 블록(`PageHeader`)을 사용자 화면에서 걷어내면서, 그
 * 헤더 우측에 붙어 있던 동작 버튼들이 갈 곳을 잃었다. 화면마다 알아서 두게 하면
 * 「예시로 둘러보기」·「스탬프 찍기」·리포트 필터가 다시 제각각의 위치에 서게
 * 되므로(`PageHeader`의 `actions`가 애초에 없애려던 문제다), 본문 첫 줄 우측이라는
 * **한 자리**를 대신 준다. docs/UI_SPEC.md §7.
 *
 * 넘길 것이 없으면 아무것도 그리지 않는다 — 빈 줄이 본문을 12px 밀어내면 헤더를
 * 지운 이유가 절반 사라진다.
 */
export function PageActions({
  children,
  className,
}: {
  children?: React.ReactNode;
  className?: string;
}) {
  if (!children) {
    return null;
  }
  return (
    <div className={cn("mb-3 flex flex-wrap items-center justify-end gap-1.5", className)}>
      {children}
    </div>
  );
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
