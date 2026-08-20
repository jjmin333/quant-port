import type { ApiError, ApiResponse } from "@/lib/api-types";
import { apiCredentials, sessionFetch } from "@/lib/session-fetch";

const DEFAULT_API_URL = "http://localhost:8080";

/** SSR은 BE 직접, 브라우저 폴링은 Next rewrite(동일 origin) 경유 */
export function getApiBaseUrl(): string {
  if (typeof window !== "undefined") {
    return "";
  }
  return process.env.NEXT_PUBLIC_API_URL ?? DEFAULT_API_URL;
}

/** Server Components에서 BE 호출 시 쿠키 전달 */
export async function getServerCookieHeader(): Promise<Record<string, string>> {
  if (typeof window !== "undefined") {
    return {};
  }
  const { cookies } = await import("next/headers");
  const jar = await cookies();
  const header = jar.toString();
  return header ? { Cookie: header } : {};
}

function mergeFetchInit(init?: RequestInit): RequestInit {
  return {
    ...apiCredentials,
    ...init,
    headers: {
      ...(init?.headers ?? {}),
    },
  };
}

export { mergeFetchInit };

export class PortfolioFetchError extends Error {
  status: number;
  hints?: string[];

  constructor(message: string, status: number, hints?: string[]) {
    super(message);
    this.name = "PortfolioFetchError";
    this.status = status;
    this.hints = hints;
  }
}

export async function parseJson<T>(res: Response): Promise<T> {
  const text = await res.text();
  if (!text) {
    throw new PortfolioFetchError("Empty response from API", res.status);
  }
  try {
    return JSON.parse(text) as T;
  } catch {
    throw new PortfolioFetchError(text.slice(0, 200), res.status);
  }
}

/**
 * Throws a PortfolioFetchError carrying the server's error message when the
 * response is not ok. Every API wrapper repeated this block verbatim.
 */
export async function ensureOk(res: Response, fallbackMessage: string): Promise<void> {
  if (res.ok) return;
  const body = await parseJson<ApiError>(res).catch(() => ({ error: res.statusText }));
  throw new PortfolioFetchError(body.error || fallbackMessage, res.status);
}

/** Session-authenticated request that unwraps `{ data }`. Browser/polling path. */
export async function apiRequest<T>(
  path: string,
  init?: RequestInit,
  fallbackMessage = "API request failed",
): Promise<T> {
  const res = await sessionFetch(path, mergeFetchInit({ cache: "no-store", ...init }));
  await ensureOk(res, fallbackMessage);
  const body = await parseJson<ApiResponse<T>>(res);
  return body.data;
}

/**
 * Server Component request that forwards the request cookies. Caching is left
 * to the caller — some routes are no-store, others are revalidated.
 */
export async function serverApiRequest<T>(
  path: string,
  init: RequestInit,
  fallbackMessage = "API request failed",
): Promise<T> {
  const cookieHeader = await getServerCookieHeader();
  const res = await fetch(`${getApiBaseUrl()}${path}`, mergeFetchInit({
    ...init,
    headers: { ...cookieHeader, ...(init.headers ?? {}) },
  }));
  await ensureOk(res, fallbackMessage);
  const body = await parseJson<ApiResponse<T>>(res);
  return body.data;
}

