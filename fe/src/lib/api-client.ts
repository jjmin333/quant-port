import type {
  ApiError,
  ApiResponse,
  PortfolioDiagnostics,
  PortfolioSummary,
} from "@/lib/portfolio-types";
import type {
  DashboardInsights,
  HistoryRange,
  PortfolioHistory,
} from "@/lib/insights-types";
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

export async function fetchPortfolioSummary(): Promise<PortfolioSummary> {
  const cookieHeader = await getServerCookieHeader();
  const res = await fetch(`${getApiBaseUrl()}/api/v1/portfolio/summary`, mergeFetchInit({
    cache: "no-store",
    headers: cookieHeader,
  }));

  if (!res.ok) {
    const body = await parseJson<ApiError>(res).catch(() => ({ error: res.statusText }));
    throw new PortfolioFetchError(body.error || "Failed to load portfolio", res.status);
  }

  const body = await parseJson<ApiResponse<PortfolioSummary>>(res);
  return body.data;
}

/** Client-side fetch for polling (no cache). */
export async function fetchPortfolioSummaryClient(): Promise<PortfolioSummary> {
  const res = await sessionFetch("/api/v1/portfolio/summary", mergeFetchInit({
    cache: "no-store",
  }));

  if (!res.ok) {
    const body = await parseJson<ApiError>(res).catch(() => ({ error: res.statusText }));
    throw new PortfolioFetchError(body.error || "Failed to load portfolio", res.status);
  }

  const body = await parseJson<ApiResponse<PortfolioSummary>>(res);
  return body.data;
}

/** Client-side live quotes (prices + FX, 5s). */
export async function fetchPortfolioLiveClient(): Promise<PortfolioSummary> {
  const res = await sessionFetch("/api/v1/portfolio/live", mergeFetchInit({
    cache: "no-store",
  }));

  if (!res.ok) {
    const body = await parseJson<ApiError>(res).catch(() => ({ error: res.statusText }));
    throw new PortfolioFetchError(body.error || "Failed to load live portfolio", res.status);
  }

  const body = await parseJson<ApiResponse<PortfolioSummary>>(res);
  return body.data;
}

export async function fetchPortfolioDiagnostics(): Promise<PortfolioDiagnostics> {
  const res = await sessionFetch("/api/v1/portfolio/diagnostics", mergeFetchInit({
    cache: "no-store",
  }));
  const body = await parseJson<ApiResponse<PortfolioDiagnostics>>(res);
  return body.data;
}

export async function fetchPortfolioHistory(
  range: HistoryRange = "1m",
): Promise<PortfolioHistory> {
  const cookieHeader = await getServerCookieHeader();
  const res = await fetch(`${getApiBaseUrl()}/api/v1/portfolio/history?range=${range}`, mergeFetchInit({
    cache: "no-store",
    headers: cookieHeader,
  }));

  if (!res.ok) {
    const body = await parseJson<ApiError>(res).catch(() => ({ error: res.statusText }));
    throw new PortfolioFetchError(body.error || "Failed to load asset history", res.status);
  }

  const body = await parseJson<ApiResponse<PortfolioHistory>>(res);
  return body.data;
}

export async function fetchPortfolioHistoryClient(
  range: HistoryRange = "1m",
): Promise<PortfolioHistory> {
  return fetchPortfolioHistory(range);
}

export async function fetchDashboardInsights(): Promise<DashboardInsights> {
  const cookieHeader = await getServerCookieHeader();
  const res = await fetch(`${getApiBaseUrl()}/api/v1/insights/dashboard`, mergeFetchInit({
    next: { revalidate: 300 },
    headers: cookieHeader,
  }));

  if (!res.ok) {
    const body = await parseJson<ApiError>(res).catch(() => ({ error: res.statusText }));
    throw new PortfolioFetchError(body.error || "Failed to load insights", res.status);
  }

  const body = await parseJson<ApiResponse<DashboardInsights>>(res);
  const data = body.data;
  return {
    ...data,
    risks: data.risks ?? [],
  };
}

export async function fetchDashboardInsightsClient(): Promise<DashboardInsights> {
  const res = await sessionFetch("/api/v1/insights/dashboard", mergeFetchInit({
    cache: "no-store",
  }));

  if (!res.ok) {
    const body = await parseJson<ApiError>(res).catch(() => ({ error: res.statusText }));
    throw new PortfolioFetchError(body.error || "Failed to load insights", res.status);
  }

  const body = await parseJson<ApiResponse<DashboardInsights>>(res);
  const data = body.data;
  return {
    ...data,
    risks: data.risks ?? [],
  };
}
