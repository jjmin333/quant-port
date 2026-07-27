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
import type { AuthSession } from "@/lib/auth-types";
import { fetchMe, login as apiLogin, logout as apiLogout, signup as apiSignup } from "@/lib/auth";
import { proactiveRefreshIntervalMs, refreshSession } from "@/lib/session-fetch";
import { isAuthPage } from "@/lib/auth-routes";

type AuthContextValue = {
  session: AuthSession | null;
  loading: boolean;
  login: (email: string, password: string, redirectTo?: string) => Promise<void>;
  signup: (email: string, password: string, displayName: string) => Promise<void>;
  logout: () => Promise<void>;
  refresh: () => Promise<void>;
};

const AuthContext = createContext<AuthContextValue | null>(null);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [session, setSession] = useState<AuthSession | null>(null);
  const [loading, setLoading] = useState(true);
  const router = useRouter();
  const pathname = usePathname();

  const refresh = useCallback(async () => {
    try {
      const me = await fetchMe();
      setSession(me);
    } catch {
      setSession(null);
    }
  }, []);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const me = await fetchMe();
        if (!cancelled) setSession(me);
      } catch {
        if (!cancelled) setSession(null);
      } finally {
        if (!cancelled) setLoading(false);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, []);

  useEffect(() => {
    if (!session) return;
    const id = setInterval(async () => {
      const ok = await refreshSession();
      if (ok) {
        try {
          const me = await fetchMe();
          setSession(me);
        } catch {
          setSession(null);
        }
      }
    }, proactiveRefreshIntervalMs());
    return () => clearInterval(id);
  }, [session]);

  const login = useCallback(
    async (email: string, password: string, redirectTo?: string) => {
      const next = await apiLogin(email, password);
      setSession(next);
      if (redirectTo && redirectTo.startsWith("/") && !isAuthPage(redirectTo)) {
        router.push(redirectTo);
      } else {
        router.push("/");
      }
      router.refresh();
    },
    [router],
  );

  const signup = useCallback(
    async (email: string, password: string, displayName: string) => {
      const next = await apiSignup(email, password, displayName);
      setSession(next);
      router.push("/");
      router.refresh();
    },
    [router],
  );

  const logout = useCallback(async () => {
    try {
      await apiLogout();
    } finally {
      setSession(null);
      if (!isAuthPage(pathname)) {
        router.push("/");
        router.refresh();
      }
    }
  }, [pathname, router]);

  const value = useMemo(
    () => ({ session, loading, login, signup, logout, refresh }),
    [session, loading, login, signup, logout, refresh],
  );

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth() {
  const ctx = useContext(AuthContext);
  if (!ctx) {
    throw new Error("useAuth must be used within AuthProvider");
  }
  return ctx;
}
