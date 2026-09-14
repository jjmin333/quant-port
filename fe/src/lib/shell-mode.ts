export type ShellMode = "desktop" | "mobile" | "webview";

const WEBVIEW_STORAGE_KEY = "quant-port-webview";

export function detectWebViewFromSearch(search: string): boolean {
  const params = new URLSearchParams(search);
  return params.get("webview") === "1" || params.get("embed") === "1";
}

export function detectWebViewFromUA(ua: string): boolean {
  return /;\s*wv\)|WebView|QuantPortApp/i.test(ua);
}

/**
 * 요청 User-Agent만 보고 「좁은 화면일 것」을 **미리** 찍는다 (2026-09-03).
 *
 * # 왜 서버가 이걸 아는가
 *
 * 화면 폭은 브라우저에만 있는 값이라, 서버가 그린 첫 HTML은 늘 데스크톱 배치였다.
 * 폰에서 메뉴를 처음 열면 그 데스크톱 배치가 **먼저 그려진 뒤** 자바스크립트가
 * 붙고 나서야 모바일로 바뀐다 — 사용자에게는 1~2초 동안 화면이 뭉개져 보였고,
 * 로딩이 아니라 **고장으로** 읽혔다(2026-09-03 사용자 제보, 스크린샷 2장).
 *
 * UA는 폭을 정확히 말해 주지 않지만 「폰인가」는 거의 언제나 맞고, 첫 프레임을
 * 맞히는 데는 그것으로 충분하다. 마운트 뒤에는 실제 미디어 쿼리가 이 값을 덮는다
 * (`ShellModeProvider`) — **틀려도 한 프레임 뒤에 바로잡힌다.**
 *
 * # 태블릿
 *
 * iPadOS 13부터 사파리가 데스크톱 UA(`Macintosh`)를 보내므로 여기서 잡히지 않는다.
 * 대신 안드로이드 태블릿·구형 iPad는 잡힌다. 못 잡은 기기는 예전과 같은 경로를
 * 밟을 뿐이라 **더 나빠지지는 않는다.**
 */
export function detectMobileFromUA(ua: string): boolean {
  if (!ua) return false;
  // 안드로이드 태블릿은 `Mobile`이 없지만 폭은 대개 1024 미만이라 함께 잡는다.
  if (/iPhone|iPod|iPad|Android|Windows Phone|IEMobile|BlackBerry|Opera Mini/i.test(ua)) return true;
  return /Mobi|Mobile/i.test(ua);
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
