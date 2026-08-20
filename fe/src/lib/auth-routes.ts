/** 로그인 필요 (회원 기능) */
export const MEMBER_ONLY_PATH_PREFIXES = ["/watchlist"] as const;

export const AUTH_PAGES = ["/login", "/signup"] as const;

export function isAuthPage(pathname: string): boolean {
  return AUTH_PAGES.some((p) => pathname === p || pathname.startsWith(`${p}/`));
}

export function isMemberOnlyPath(pathname: string): boolean {
  return MEMBER_ONLY_PATH_PREFIXES.some(
    (prefix) => pathname === prefix || pathname.startsWith(`${prefix}/`),
  );
}
