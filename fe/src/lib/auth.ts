import { apiRequest } from "@/lib/api-client";
import { clearFetchCache } from "@/lib/fetch-cache";
import type { AuthSession } from "@/lib/auth-types";

export { apiCredentials } from "@/lib/session-fetch";

function authFetch<T>(path: string, init: RequestInit = {}): Promise<T> {
  return apiRequest<T>(
    path,
    {
      ...init,
      headers: {
        "Content-Type": "application/json",
        ...(init.headers ?? {}),
      },
    },
    "Request failed",
  );
}

export async function signup(
  email: string,
  password: string,
  displayName: string,
): Promise<AuthSession> {
  return authFetch<AuthSession>("/api/v1/auth/signup", {
    method: "POST",
    body: JSON.stringify({ email, password, displayName }),
  });
}

export async function login(email: string, password: string): Promise<AuthSession> {
  const session = await authFetch<AuthSession>("/api/v1/auth/login", {
    method: "POST",
    body: JSON.stringify({ email, password }),
  });
  // 로그인 전에 받아 둔 응답 중에는 비로그인 상태의 것이 섞여 있다(관심·보유
  // 표시가 빠진 시세 등). "누구인지"가 바뀌었으니 통째로 버린다.
  clearFetchCache();
  return session;
}

export async function logout(): Promise<void> {
  await authFetch<{ ok: boolean }>("/api/v1/auth/logout", { method: "POST" });
  // 로그아웃 뒤에 이전 사용자의 응답이 남아 있으면 안 된다.
  clearFetchCache();
}

export async function fetchMe(): Promise<AuthSession> {
  return authFetch<AuthSession>("/api/v1/auth/me");
}
