export type ShellMode = "desktop" | "mobile" | "webview";

const WEBVIEW_STORAGE_KEY = "quant-port-webview";

export function detectWebViewFromSearch(search: string): boolean {
  const params = new URLSearchParams(search);
  return params.get("webview") === "1" || params.get("embed") === "1";
}

export function detectWebViewFromUA(ua: string): boolean {
  return /;\s*wv\)|WebView|QuantPortApp/i.test(ua);
}

export function persistWebViewFlag(enabled: boolean) {
  if (typeof window === "undefined") return;
  if (enabled) {
    sessionStorage.setItem(WEBVIEW_STORAGE_KEY, "1");
  } else {
    sessionStorage.removeItem(WEBVIEW_STORAGE_KEY);
  }
}

export function readPersistedWebViewFlag(): boolean {
  if (typeof window === "undefined") return false;
  return sessionStorage.getItem(WEBVIEW_STORAGE_KEY) === "1";
}

export function resolveShellMode(
  isWebView: boolean,
  isMobileViewport: boolean,
): ShellMode {
  if (isWebView) return "webview";
  if (isMobileViewport) return "mobile";
  return "desktop";
}

export function isCompactShell(mode: ShellMode): boolean {
  return mode === "mobile" || mode === "webview";
}
