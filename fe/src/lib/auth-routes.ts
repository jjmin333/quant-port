/** 로그인 필요 (회원 기능) */
export const MEMBER_ONLY_PATH_PREFIXES = ["/watchlist"] as const;

export const AUTH_PAGES = ["/login", "/signup"] as const;

export function isAuthPage(pathname: string): boolean {
  return AUTH_PAGES.some((p) => pathname === p || pathname.startsWith(`${p}/`));
}

/**
 * 로그인·가입 뒤 돌아갈 곳으로 **믿어도 되는** 값만 돌려준다.
 *
 * 앱 안 경로만 받는다: `/`로 시작하되 `//`나 `/\`로 시작하면 안 된다 — 브라우저는 그 둘을
 * 다른 호스트로 읽는다(`/login?next=//evil.example` → 로그인 직후 바깥으로 나간다).
 * 로그인·가입 화면 자신도 받지 않는다(돌아가 봐야 같은 폼이다).
 */
export function safeNext(next: string | null | undefined): string | undefined {
  if (!next) return undefined;
  const v = next.trim();
  if (!v.startsWith("/") || v.startsWith("//") || v.startsWith("/\\")) return undefined;
  if (isAuthPage(v.split("?")[0])) return undefined;
  return v;
}

/** 가입 링크. 보던 곳이 있으면 싣는다 — 가입 → 관심 종목 고르기 → 그곳으로 돌아온다. */
export function signupHref(next?: string | null): string {
  const n = safeNext(next);
  return n ? `/signup?next=${encodeURIComponent(n)}` : "/signup";
}

/** 가입 직후 화면. 온보딩(관심 종목 고르기)을 거쳐 `next`로 간다. */
export function welcomeHref(next?: string | null): string {
  const n = safeNext(next);
  return n && n !== "/" ? `/welcome?next=${encodeURIComponent(n)}` : "/welcome";
}

export function isMemberOnlyPath(pathname: string): boolean {
  return MEMBER_ONLY_PATH_PREFIXES.some(
    (prefix) => pathname === prefix || pathname.startsWith(`${prefix}/`),
  );
}
