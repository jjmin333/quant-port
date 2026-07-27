/** 로그인 필요 (회원 기능) */
export const MEMBER_ONLY_PATH_PREFIXES = ["/watchlist"] as const;

/** 로그인 + 계좌 연동 필요 */
export const BROKER_REQUIRED_PATH_PREFIXES = [
  "/holdings",
  "/transactions",
] as const;

export const ACCOUNT_PATH_PREFIXES = [
  "/onboarding/link-broker",
  ...BROKER_REQUIRED_PATH_PREFIXES,
] as const;

export const AUTH_PAGES = ["/login", "/signup"] as const;

export function isAuthPage(pathname: string): boolean {
  return AUTH_PAGES.some((p) => pathname === p || pathname.startsWith(`${p}/`));
}

export function isMemberOnlyPath(pathname: string): boolean {
  return MEMBER_ONLY_PATH_PREFIXES.some(
    (prefix) => pathname === prefix || pathname.startsWith(`${prefix}/`),
  );
}

export function isBrokerRequiredPath(pathname: string): boolean {
  return BROKER_REQUIRED_PATH_PREFIXES.some(
    (prefix) => pathname === prefix || pathname.startsWith(`${prefix}/`),
  );
}
