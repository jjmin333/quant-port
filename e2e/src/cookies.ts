import { request, type APIRequestContext } from "@playwright/test";

import { API_URL, apiPath } from "./env";

export const ACCESS_COOKIE = "qp_access";
export const REFRESH_COOKIE = "qp_refresh";

export type StoredCookie = {
  name: string;
  value: string;
  domain: string;
  path: string;
  /** unix seconds. 세션 쿠키면 -1. */
  expires: number;
  httpOnly: boolean;
  secure: boolean;
  sameSite: string;
};

export async function cookiesOf(ctx: APIRequestContext): Promise<StoredCookie[]> {
  const state = await ctx.storageState();
  return state.cookies as StoredCookie[];
}

export async function cookieNamed(
  ctx: APIRequestContext,
  name: string,
): Promise<StoredCookie | undefined> {
  return (await cookiesOf(ctx)).find((c) => c.name === name);
}

/**
 * 특정 refresh 토큰 **하나만** 들고 `/auth/refresh`를 친다.
 *
 * 쿠키 항아리를 공유하는 컨텍스트로는 "이미 쓴 토큰을 다시 쓰면 어떻게 되는가"를
 * 물을 수 없다 — 회전이 일어나면서 항아리 안의 값이 새 것으로 덮여 버리기
 * 때문이다. 그래서 매번 빈 컨텍스트에 헤더로 토큰을 직접 실어 보낸다.
 */
export async function refreshWithToken(token: string): Promise<{
  status: number;
  body: string;
  context: APIRequestContext;
}> {
  const ctx = await request.newContext({
    baseURL: API_URL,
    extraHTTPHeaders: { Cookie: `${REFRESH_COOKIE}=${token}` },
  });
  const res = await ctx.post(apiPath("/auth/refresh"));
  return { status: res.status(), body: await res.text(), context: ctx };
}

/** 임의의 Authorization 헤더로 보호 경로를 친다. 위조 토큰 검증용. */
export async function getWithBearer(path: string, token: string): Promise<number> {
  const ctx = await request.newContext({
    baseURL: API_URL,
    extraHTTPHeaders: { Authorization: `Bearer ${token}` },
  });
  const res = await ctx.get(apiPath(path));
  const status = res.status();
  await ctx.dispose();
  return status;
}
