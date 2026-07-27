import type { ApiError, ApiResponse } from "@/lib/portfolio-types";
import { PortfolioFetchError, parseJson } from "@/lib/api-client";
import { sessionFetch } from "@/lib/session-fetch";
import type { AuthSession, BrokerLinkStatus } from "@/lib/auth-types";

export { apiCredentials } from "@/lib/session-fetch";

async function authFetch<T>(path: string, init: RequestInit = {}): Promise<T> {
  const res = await sessionFetch(path, {
    ...init,
    headers: {
      "Content-Type": "application/json",
      ...(init.headers ?? {}),
    },
  });
  if (!res.ok) {
    const body = await parseJson<ApiError>(res).catch(() => ({ error: res.statusText }));
    throw new PortfolioFetchError(body.error || "Request failed", res.status);
  }
  const body = await parseJson<ApiResponse<T>>(res);
  return body.data;
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
  return authFetch<AuthSession>("/api/v1/auth/login", {
    method: "POST",
    body: JSON.stringify({ email, password }),
  });
}

export async function logout(): Promise<void> {
  await authFetch<{ ok: boolean }>("/api/v1/auth/logout", { method: "POST" });
}

export async function fetchMe(): Promise<AuthSession> {
  return authFetch<AuthSession>("/api/v1/auth/me");
}

export async function linkTossBroker(
  clientId: string,
  clientSecret: string,
): Promise<BrokerLinkStatus> {
  return authFetch<BrokerLinkStatus>("/api/v1/broker/toss/link", {
    method: "POST",
    body: JSON.stringify({ clientId, clientSecret }),
  });
}

export async function fetchBrokerStatus(): Promise<BrokerLinkStatus> {
  return authFetch<BrokerLinkStatus>("/api/v1/broker/toss/status");
}

export async function unlinkTossBroker(): Promise<void> {
  await authFetch<{ ok: boolean }>("/api/v1/broker/toss/unlink", { method: "DELETE" });
}
