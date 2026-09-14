"use client";

import {
  createContext,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from "react";
import { useSearchParams } from "next/navigation";
import {
  detectWebViewFromSearch,
  detectWebViewFromUA,
  isCompactShell,
  persistWebViewFlag,
  readPersistedWebViewFlag,
  resolveShellMode,
  type ShellMode,
} from "@/lib/shell-mode";

type ShellModeContextValue = {
  mode: ShellMode;
  isWebView: boolean;
  isCompact: boolean;
};

const ShellModeContext = createContext<ShellModeContextValue>({
  mode: "desktop",
  isWebView: false,
  isCompact: false,
});

export function ShellModeProvider({
  children,
  /**
   * 서버가 요청 UA로 미리 찍은 값(`detectMobileFromUA`). **첫 프레임을 맞히는
   * 용도**이고, 마운트 직후 실제 미디어 쿼리가 이 값을 덮는다.
   *
   * 서버 렌더와 첫 클라이언트 렌더가 **같은 prop을 읽으므로** 하이드레이션이
   * 어긋나지 않는다 — 렌더 중에 `window`를 읽어 초기값을 정하면 그게 깨진다.
   */
  initialMobile = false,
}: {
  children: ReactNode;
  initialMobile?: boolean;
}) {
  const searchParams = useSearchParams();
  const [isMobileViewport, setIsMobileViewport] = useState(initialMobile);
  const [isWebViewUA, setIsWebViewUA] = useState(false);
  const [persistedWebView, setPersistedWebView] = useState(false);

  const queryWebView = detectWebViewFromSearch(`?${searchParams.toString()}`);

  useEffect(() => {
    setIsWebViewUA(detectWebViewFromUA(navigator.userAgent));
    setPersistedWebView(readPersistedWebViewFlag());
  }, []);

  useEffect(() => {
    if (queryWebView) {
      persistWebViewFlag(true);
      setPersistedWebView(true);
    }
  }, [queryWebView]);

  useEffect(() => {
    const media = window.matchMedia("(max-width: 1023px)");
    const sync = (matches: boolean) => setIsMobileViewport(matches);
    sync(media.matches);
    const onChange = (event: MediaQueryListEvent) => sync(event.matches);
    media.addEventListener("change", onChange);
    return () => media.removeEventListener("change", onChange);
  }, []);

  const isWebView = queryWebView || persistedWebView || isWebViewUA;
  const mode = resolveShellMode(isWebView, isMobileViewport);
  const isCompact = isCompactShell(mode);

  useEffect(() => {
    document.documentElement.dataset.shell = mode;
    return () => {
      delete document.documentElement.dataset.shell;
    };
  }, [mode]);

  const value = useMemo(
    () => ({ mode, isWebView, isCompact }),
    [mode, isWebView, isCompact],
  );

  return <ShellModeContext.Provider value={value}>{children}</ShellModeContext.Provider>;
}

export function useShellMode() {
  return useContext(ShellModeContext);
}
