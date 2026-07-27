/**
 * Account / broker linking UI visibility (FE only).
 * BE APIs and storage remain for future reuse — see docs/FE_ACCOUNT_FEATURES.md
 */
export const ACCOUNT_FEATURES_VISIBLE = false;

/** Sidebar + direct URL redirect when hidden */
export const HIDDEN_ACCOUNT_PATH_PREFIXES = [
  "/onboarding/link-broker",
  "/holdings",
  "/transactions",
  "/portfolio",
  "/risk",
  "/backtest",
] as const;

export function isHiddenAccountPath(pathname: string): boolean {
  if (ACCOUNT_FEATURES_VISIBLE) return false;
  return HIDDEN_ACCOUNT_PATH_PREFIXES.some(
    (prefix) => pathname === prefix || pathname.startsWith(`${prefix}/`),
  );
}

export function isSidebarNavVisible(href: string): boolean {
  if (ACCOUNT_FEATURES_VISIBLE) return true;

  const hiddenHrefs = new Set<string>([
    "/onboarding/link-broker",
    "/holdings",
    "/transactions",
    "/portfolio",
    "/risk",
    "/backtest",
  ]);
  return !hiddenHrefs.has(href);
}

export function isTradeSettingsVisible(): boolean {
  return ACCOUNT_FEATURES_VISIBLE;
}
