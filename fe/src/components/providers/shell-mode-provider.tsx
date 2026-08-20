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

export function ShellModeProvider({ children }: { children: ReactNode }) {
  const searchParams = useSearchParams();
  const [isMobileViewport, setIsMobileViewport] = useState(false);
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
