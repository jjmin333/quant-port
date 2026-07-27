"use client";

import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from "react";
import { usePathname, useRouter } from "next/navigation";
import {
  resolveHeaderSearchConfig,
  type HeaderSearchConfig,
  type HeaderSearchMode,
} from "@/lib/header-search-config";

type HeaderSearchContextValue = {
  config: HeaderSearchConfig;
  mode: HeaderSearchMode;
  inputValue: string;
  setInputValue: (value: string) => void;
  /** Debounced query consumed by page components */
  activeQuery: string;
  isSearchActive: boolean;
  clearSearch: () => void;
  submitSearch: () => void;
};

const HeaderSearchContext = createContext<HeaderSearchContextValue | null>(null);

export function HeaderSearchProvider({ children }: { children: ReactNode }) {
  const pathname = usePathname();
  const router = useRouter();
  const config = useMemo(() => resolveHeaderSearchConfig(pathname), [pathname]);
  const [inputValue, setInputValue] = useState("");
  const [activeQuery, setActiveQuery] = useState("");

  useEffect(() => {
    setInputValue("");
    setActiveQuery("");
  }, [pathname, config.mode]);

  useEffect(() => {
    if (config.mode === "hidden") {
      setActiveQuery("");
      return;
    }
    const trimmed = inputValue.trim();
    if (!trimmed) {
      setActiveQuery("");
      return;
    }
    const timer = window.setTimeout(() => {
      setActiveQuery(trimmed);
    }, config.debounceMs);
    return () => window.clearTimeout(timer);
  }, [inputValue, config.mode, config.debounceMs]);

  const clearSearch = useCallback(() => {
    setInputValue("");
    setActiveQuery("");
  }, []);

  const submitSearch = useCallback(() => {
    const trimmed = inputValue.trim();
    setActiveQuery(trimmed);

    if (config.mode === "stock" && trimmed) {
      const symbol = trimmed.toUpperCase().replace(/\s+/g, "");
      if (pathname.startsWith("/stocks/")) {
        router.push(`/stocks/${encodeURIComponent(symbol)}`);
      } else {
        router.push(`/analysis?symbol=${encodeURIComponent(symbol)}`);
      }
    }
  }, [config.mode, inputValue, pathname, router]);

  const value = useMemo(
    () => ({
      config,
      mode: config.mode,
      inputValue,
      setInputValue,
      activeQuery,
      isSearchActive: activeQuery.length > 0,
      clearSearch,
      submitSearch,
    }),
    [config, inputValue, activeQuery, clearSearch, submitSearch],
  );

  return (
    <HeaderSearchContext.Provider value={value}>{children}</HeaderSearchContext.Provider>
  );
}

export function useHeaderSearch() {
  const ctx = useContext(HeaderSearchContext);
  if (!ctx) {
    throw new Error("useHeaderSearch must be used within HeaderSearchProvider");
  }
  return ctx;
}

/** Returns null when provider is missing (e.g. auth layout). */
export function useHeaderSearchOptional() {
  return useContext(HeaderSearchContext);
}
