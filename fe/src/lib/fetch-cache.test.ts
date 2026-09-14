import { beforeEach, describe, expect, it, vi } from "vitest";
import {
  cachedGet,
  clearFetchCache,
  invalidateFetchCache,
} from "@/lib/fetch-cache";

beforeEach(() => {
  clearFetchCache();
  vi.useRealTimers();
});

describe("cachedGet", () => {
  it("같은 순간에 들어온 호출은 **요청 하나**를 함께 기다린다", async () => {
    // 화면 하나가 뜰 때 카드 여럿이 같은 값을 각자 부르던 것이 여기서 합쳐진다.
    let calls = 0;
    const load = () =>
      new Promise<number>((resolve) => {
        calls += 1;
        setTimeout(() => resolve(42), 5);
      });

    const [a, b, c] = await Promise.all([
      cachedGet("/k", load, 1000),
      cachedGet("/k", load, 1000),
      cachedGet("/k", load, 1000),
    ]);

    expect(calls).toBe(1);
    expect([a, b, c]).toEqual([42, 42, 42]);
  });

  it("TTL 안에서는 다시 부르지 않는다", async () => {
    let calls = 0;
    const load = async () => ++calls;

    await cachedGet("/k", load, 1000);
    await cachedGet("/k", load, 1000);
    expect(calls).toBe(1);
  });

  it("TTL이 0이면 매번 새로 받는다 — 폴링이 캐시에 막히면 안 된다", async () => {
    let calls = 0;
    const load = async () => ++calls;

    await cachedGet("/k", load, 0);
    await cachedGet("/k", load, 0);
    expect(calls).toBe(2);
  });

  it("키가 다르면 섞이지 않는다", async () => {
    const load = (v: string) => async () => v;
    expect(await cachedGet("/a", load("a"), 1000)).toBe("a");
    expect(await cachedGet("/b", load("b"), 1000)).toBe("b");
  });

  it("실패는 캐시하지 않는다 — 한 번 실패한 화면이 TTL 동안 굳으면 안 된다", async () => {
    let calls = 0;
    const load = async () => {
      calls += 1;
      if (calls === 1) throw new Error("boom");
      return "ok";
    };

    await expect(cachedGet("/k", load, 1000)).rejects.toThrow("boom");
    expect(await cachedGet("/k", load, 1000)).toBe("ok");
    expect(calls).toBe(2);
  });

  it("무효화하면 다음 호출이 새로 받는다 — 스탬프를 찍은 직후가 그렇다", async () => {
    let calls = 0;
    const load = async () => ++calls;

    await cachedGet("/api/v1/stamps/state?symbol=A", load, 10_000);
    invalidateFetchCache("/api/v1/stamps");
    await cachedGet("/api/v1/stamps/state?symbol=A", load, 10_000);
    expect(calls).toBe(2);
  });

  it("무효화는 접두사가 맞는 키만 버린다", async () => {
    let stamps = 0;
    let stocks = 0;

    await cachedGet("/api/v1/stamps/state", async () => ++stamps, 10_000);
    await cachedGet("/api/v1/stocks/000660", async () => ++stocks, 10_000);
    invalidateFetchCache("/api/v1/stamps");
    await cachedGet("/api/v1/stamps/state", async () => ++stamps, 10_000);
    await cachedGet("/api/v1/stocks/000660", async () => ++stocks, 10_000);

    expect(stamps).toBe(2);
    expect(stocks).toBe(1);
  });

  it("로그아웃(clearFetchCache)은 전부 버린다 — 남의 응답을 보면 안 된다", async () => {
    let calls = 0;
    const load = async () => ++calls;
    await cachedGet("/api/v1/user/holdings", load, 10_000);
    clearFetchCache();
    await cachedGet("/api/v1/user/holdings", load, 10_000);
    expect(calls).toBe(2);
  });
});
