const DEFAULT_API_URL = "http://localhost:8080";

function getApiBaseUrl(): string {
  if (typeof window !== "undefined") {
    return "";
  }
  return process.env.NEXT_PUBLIC_API_URL ?? DEFAULT_API_URL;
}

export const apiCredentials: RequestInit = {
  credentials: "include",
};

/** Access token default 15m — refresh proactively before expiry. */
const PROACTIVE_REFRESH_MS = 10 * 60 * 1000;

let inflightRefresh: Promise<boolean> | null = null;

export function proactiveRefreshIntervalMs(): number {
  return PROACTIVE_REFRESH_MS;
}

/** Silently renew access token using the refresh cookie (7-day session window). */
export async function refreshSession(): Promise<boolean> {
  if (inflightRefresh) {
    return inflightRefresh;
  }

  inflightRefresh = (async () => {
    try {
      const res = await fetch(`${getApiBaseUrl()}/api/v1/auth/refresh`, {
        method: "POST",
        credentials: "include",
      });
      return res.ok;
    } catch {
      return false;
    } finally {
      inflightRefresh = null;
    }
  })();

  return inflightRefresh;
}

export async function sessionFetch(path: string, init: RequestInit = {}): Promise<Response> {
  const url = `${getApiBaseUrl()}${path}`;
  const merged: RequestInit = {
    ...apiCredentials,
    ...init,
    headers: {
      ...(init.headers ?? {}),
    },
  };

  let res = await fetch(url, merged);
  if (res.status === 401 && !path.includes("/auth/refresh")) {
    const renewed = await refreshSession();
    if (renewed) {
      res = await fetch(url, merged);
    }
  }
  return res;
}
