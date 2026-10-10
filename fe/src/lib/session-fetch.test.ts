import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  markRefreshImpossible,
  markRefreshUsable,
  refreshSession,
  sessionFetch,
  setSignedOutHandler,
} from "./session-fetch";

/*
비로그인 사용자에게 `POST /auth/refresh`가 401마다 따라붙던 것을 막는 규칙의 시험
(2026-09-05 사용자 지적 6). 실측에서는 탭이 다시 보일 때마다 한 벌씩 나갔다.
*/

const realFetch = globalThis.fetch;

function stubFetch(reply: (url: string, init?: RequestInit) => Response) {
  const calls: string[] = [];
  const fn = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    const url = String(input);
    calls.push(`${init?.method ?? "GET"} ${url}`);
    return reply(url, init);
  });
  globalThis.fetch = fn as unknown as typeof fetch;
  return { calls, fn };
}

beforeEach(() => {
  // 각 시험은 「아직 해 보지 않은」 상태에서 시작한다.
  markRefreshUsable();
});

afterEach(() => {
  globalThis.fetch = realFetch;
  vi.restoreAllMocks();
});

describe("refreshSession", () => {
  it("401을 한 번 받으면 그 뒤로는 묻지 않는다", async () => {
    const { calls } = stubFetch(() => new Response(null, { status: 401 }));

    expect(await refreshSession()).toBe(false);
    expect(await refreshSession()).toBe(false);
    expect(await refreshSession()).toBe(false);

    expect(calls.filter((c) => c.includes("/auth/refresh"))).toHaveLength(1);
  });

  it("서버 사정(5xx)은 굳히지 않는다 — 로그인한 사람을 쫓아내면 안 된다", async () => {
    const { calls } = stubFetch(() => new Response(null, { status: 503 }));

    expect(await refreshSession()).toBe(false);
    expect(await refreshSession()).toBe(false);

    expect(calls.filter((c) => c.includes("/auth/refresh"))).toHaveLength(2);
  });

  it("로그인하면 다시 열린다", async () => {
    const { calls } = stubFetch(() => new Response(null, { status: 401 }));
    await refreshSession();
    expect(calls).toHaveLength(1);

    markRefreshUsable(); // 로그인 성공이 부르는 것
    await refreshSession();
    expect(calls).toHaveLength(2);
  });

  it("갱신 401은 화면에 로그아웃을 알리고, 5xx는 알리지 않는다 (10-07 제보 2)", async () => {
    const signedOut = vi.fn();
    setSignedOutHandler(signedOut);
    stubFetch(() => new Response(null, { status: 503 }));
    await refreshSession();
    expect(signedOut).not.toHaveBeenCalled();
    stubFetch(() => new Response(null, { status: 401 }));
    await refreshSession();
    expect(signedOut).toHaveBeenCalledTimes(1);
    setSignedOutHandler(null);
  });

  it("로그아웃하면 곧바로 닫힌다 — 해 보지도 않는다", async () => {
    const { calls } = stubFetch(() => new Response(null, { status: 200 }));
    markRefreshImpossible();
    expect(await refreshSession()).toBe(false);
    expect(calls).toHaveLength(0);
  });
});

describe("sessionFetch", () => {
  it("비로그인 화면의 401 여럿에 갱신은 한 번만 붙는다", async () => {
    const { calls } = stubFetch((url) =>
      url.includes("/auth/refresh")
        ? new Response(null, { status: 401 })
        : new Response(null, { status: 401 }),
    );

    // 비로그인 홈이 실제로 묻는 것들.
    await sessionFetch("/api/v1/auth/me");
    await sessionFetch("/api/v1/stamps/state?symbol=005930");
    await sessionFetch("/api/v1/watchlist");

    expect(calls.filter((c) => c.includes("/auth/refresh"))).toHaveLength(1);
    // 원래 요청 셋은 그대로 나간다 — 막은 것은 갱신뿐이다.
    expect(calls.filter((c) => !c.includes("/auth/refresh"))).toHaveLength(3);
  });

  it("갱신이 되면 원래 요청을 다시 부른다", async () => {
    let refreshed = false;
    const { calls } = stubFetch((url) => {
      if (url.includes("/auth/refresh")) {
        refreshed = true;
        return new Response(null, { status: 200 });
      }
      return new Response(null, { status: refreshed ? 200 : 401 });
    });

    const res = await sessionFetch("/api/v1/watchlist");
    expect(res.status).toBe(200);
    expect(calls.filter((c) => c.includes("/watchlist"))).toHaveLength(2);
  });

  it("갱신 요청 자신은 401에 갱신을 붙이지 않는다", async () => {
    const { calls } = stubFetch(() => new Response(null, { status: 401 }));
    await sessionFetch("/api/v1/auth/refresh", { method: "POST" });
    expect(calls).toHaveLength(1);
  });
});
