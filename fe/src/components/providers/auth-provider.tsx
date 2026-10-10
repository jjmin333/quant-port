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
import { fetchInitialSession, fetchMe, login as apiLogin, logout as apiLogout, signup as apiSignup } from "@/lib/auth";
import { proactiveRefreshIntervalMs, refreshSession, setSignedOutHandler } from "@/lib/session-fetch";
import { isAuthPage, safeNext, welcomeHref } from "@/lib/auth-routes";
import { resetWatchlist } from "@/lib/watchlist-store";
import { isSignedOutError } from "@/lib/auth-error";

type AuthContextValue = {
  session: AuthSession | null;
  loading: boolean;
  login: (loginId: string, password: string, redirectTo?: string) => Promise<void>;
  signup: (loginId: string, password: string, displayName: string, redirectTo?: string) => Promise<void>;
  logout: () => Promise<void>;
  refresh: () => Promise<void>;
};

const AuthContext = createContext<AuthContextValue | null>(null);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [session, setSession] = useState<AuthSession | null>(null);
  const [loading, setLoading] = useState(true);
  const router = useRouter();
  const pathname = usePathname();

  /*
    실패했다고 무조건 로그아웃시키지 않는다 — 서버가 「너는 로그인하지 않았다」
    (401·403)고 말했을 때만 세션을 비운다. 429·5xx·네트워크 실패는 「답을 못
    받았다」이고, 그때 세션을 비우면 쿠키가 멀쩡한 사용자가 화면에서만 쫓겨난다
    (`lib/auth-error.ts`).
  */
  const refresh = useCallback(async () => {
    try {
      const me = await fetchMe();
      setSession(me);
    } catch (err) {
      if (isSignedOutError(err)) setSession(null);
    }
  }, []);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        // 비로그인도 200인 경로로 묻는다 — 첫 방문자 콘솔에 401이 찍히지 않게(lib/auth.ts).
        const me = await fetchInitialSession();
        if (!cancelled) setSession(me);
      } catch (err) {
        // 첫 조회는 아직 세션이 null이므로 유지든 비움이든 결과가 같다. 그래도
        // 같은 규칙을 쓴다 — 나중에 초기값이 생겨도 규칙이 한 곳에 남는다.
        if (!cancelled && isSignedOutError(err)) setSession(null);
      } finally {
        if (!cancelled) setLoading(false);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, []);

  // 갱신이 401·403이면 세션이 끝난 것이다 — 헤더와 「내 종목」을 비로그인으로 맞춘다.
  useEffect(() => {
    setSignedOutHandler(() => {
      setSession(null);
      resetWatchlist();
    });
    return () => setSignedOutHandler(null);
  }, []);

  useEffect(() => {
    if (!session) return;
    const id = setInterval(async () => {
      const ok = await refreshSession();
      if (ok) {
        try {
          const me = await fetchMe();
          setSession(me);
        } catch (err) {
          // 토큰 갱신이 **성공한 직후**다. 여기서 한 번 실패했다고 쫓아내면
          // 방금 살아난 세션을 스스로 버리는 셈이 된다.
          if (isSignedOutError(err)) setSession(null);
        }
      }
    }, proactiveRefreshIntervalMs());
    return () => clearInterval(id);
  }, [session]);

  const login = useCallback(
    async (loginId: string, password: string, redirectTo?: string) => {
      const next = await apiLogin(loginId, password);
      setSession(next);
      // `startsWith("/")`만 보면 `//다른호스트`가 통과한다 — 검사는 `safeNext` 한 곳.
      router.push(safeNext(redirectTo) ?? "/");
      router.refresh();
    },
    [router],
  );

  const signup = useCallback(
    async (loginId: string, password: string, displayName: string, redirectTo?: string) => {
      const next = await apiSignup(loginId, password, displayName);
      setSession(next);
      /*
        가입 직후에는 **관심 종목 고르기**로 간다 (2026-09-26 재방문 개선 B). 예전에는 홈으로
        떨어졌는데 홈은 비로그인 화면과 똑같아서 「가입해서 달라진 것」이 없었다. 보던 곳이
        있으면(`?next=`) 고르기를 마친 뒤 그리로 돌아간다 — 로그인과 같은 약속이다.
      */
      router.push(welcomeHref(redirectTo));
      router.refresh();
    },
    [router],
  );

  const logout = useCallback(async () => {
    try {
      await apiLogout();
    } finally {
      setSession(null);
      // 관심종목 집합은 모듈에 산다(`lib/watchlist-store`) — 세션과 함께 버리지
      // 않으면 다음 사람이 앞사람의 하트를 본다.
      resetWatchlist();
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
