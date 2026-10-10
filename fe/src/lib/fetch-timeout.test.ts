import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  DEFAULT_FETCH_TIMEOUT_MS,
  FetchTimeoutError,
  fetchWithDeadline,
  fetchWithTimeout,
  isFetchTimeout,
  retryingFetch,
  SSR_TIMEOUT_MS,
} from "@/lib/fetch-timeout";

/*
이 파일이 잠그는 것은 **끝나지 않는 요청이 실패가 되는 것**이다.

상한이 없으면 화면은 `catch`를 기다리는데 promise가 pending으로 남아, 「로딩 중」이
참인 채로 굳는다 — 홈의 「갱신 중…」이 정확히 그 모양이었다(2026-09-20 점검 요청 3).
*/

/*
**브라우저인 척해야 한다.** vitest는 `environment: "node"`로 돌고, `fetchWithTimeout`은
서버에서 `fetchWithDeadline`으로 갈라진다(중단 신호가 Next 데이터 캐시를 깨뜨려서).
`window`를 안 세우면 아래의 중단 검사들이 **다른 함수를 검사하며 통과한다** — 처음
쓴 이 파일이 실제로 그랬다.
*/
beforeEach(() => {
  vi.stubGlobal("window", globalThis as unknown as Window);
});

afterEach(() => {
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
});

/**
 * 취소될 때까지 끝나지 않는 fetch — 상한이 없으면 영영 매달리는 그 요청.
 *
 * **이미 취소된 신호는 곧바로 거절한다.** 진짜 `fetch`가 그렇게 한다
 * (`signal.aborted`를 동기적으로 보고 즉시 reject). 리스너만 거는 가짜는 그
 * 경우에 영영 매달리는데, 그건 이 모듈의 문제가 아니라 가짜가 fetch가 아닌
 * 것이다 — 처음 쓴 가짜가 그래서 자기 테스트를 5초 동안 세웠다.
 */
function hangingFetch() {
  return vi.fn((_url: string, init?: RequestInit) => {
    return new Promise<Response>((_resolve, reject) => {
      const fail = () => reject(new DOMException("aborted", "AbortError"));
      if (init?.signal?.aborted) {
        fail();
        return;
      }
      init?.signal?.addEventListener("abort", fail);
    });
  });
}

describe("요청 상한", () => {
  it("상한을 넘기면 FetchTimeoutError로 끝난다 — 영영 매달리지 않는다", async () => {
    vi.useFakeTimers();
    vi.stubGlobal("fetch", hangingFetch());

    const pending = fetchWithTimeout("/api/v1/quotes/indices", {}, 8_000);
    const assertion = expect(pending).rejects.toBeInstanceOf(FetchTimeoutError);
    await vi.advanceTimersByTimeAsync(8_000);
    await assertion;

    vi.useRealTimers();
  });

  it("정상 응답은 그대로 돌려준다", async () => {
    const ok = new Response("{}", { status: 200 });
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(ok));
    await expect(fetchWithTimeout("/api/v1/quotes/indices")).resolves.toBe(ok);
  });

  it("호출부가 취소한 것은 **시간 초과가 아니다**", async () => {
    /*
      둘을 가르는 이유: 화면 이동·검색어 변경으로 우리가 끊은 요청은 사용자에게
      보여 줄 오류가 아니지만, 시간 초과는 보여 줄 오류다.
    */
    vi.stubGlobal("fetch", hangingFetch());
    const controller = new AbortController();
    const pending = fetchWithTimeout("/api/v1/quotes", { signal: controller.signal });
    controller.abort();

    await expect(pending).rejects.toSatisfy(
      (err: unknown) => !isFetchTimeout(err) && (err as Error).name === "AbortError",
    );
  });

  it("이미 취소된 신호를 받아도 매달리지 않는다 — 리렌더 경합에서 실제로 온다", async () => {
    vi.stubGlobal("fetch", hangingFetch());
    const controller = new AbortController();
    controller.abort();
    await expect(
      fetchWithTimeout("/api/v1/quotes", { signal: controller.signal }),
    ).rejects.toThrow();
  });

  it("SSR 상한이 브라우저보다 짧다 — 여기서 기다리는 시간은 TTFB에 그대로 붙는다", () => {
    expect(SSR_TIMEOUT_MS).toBeLessThan(DEFAULT_FETCH_TIMEOUT_MS);
  });

  it("브라우저 상한은 가장 느린 실측 응답(758ms)보다 한참 위다", () => {
    expect(DEFAULT_FETCH_TIMEOUT_MS).toBeGreaterThan(758 * 5);
  });
});

describe("서버에서는 끊지 않고 기다리기만 한다", () => {
  /*
    Next의 데이터 캐시는 응답 본문을 갈라 한쪽을 캐시에 흘린다. 거기에 중단 신호가
    얹히면 그 갈래가 깨져 냉시동 첫 렌더가 터졌다 — 실측과 오류 문구는
    `fetchWithDeadline` 머리글. 그래서 서버 경로에는 **신호를 만들지 않는다.**
  */
  it("`window`가 없으면 fetch에 signal을 얹지 않는다", async () => {
    vi.unstubAllGlobals(); // 위의 beforeEach가 세운 가짜 window를 걷어낸다.
    const spy = vi.fn().mockResolvedValue(new Response("{}"));
    vi.stubGlobal("fetch", spy);

    await fetchWithTimeout("/api/v1/quotes/indices", { next: { revalidate: 30 } });

    const init = spy.mock.calls[0]![1] as RequestInit & { next?: unknown };
    expect(init.signal).toBeUndefined();
    // 캐시 지시는 그대로 넘어가야 한다 — 그게 이 경로의 존재 이유다.
    expect(init.next).toEqual({ revalidate: 30 });
  });

  it("그래도 기다림에는 상한이 있다", async () => {
    vi.useFakeTimers();
    // signal을 안 받으므로 영영 안 끝나는 fetch.
    vi.stubGlobal("fetch", vi.fn(() => new Promise<Response>(() => {})));

    const pending = fetchWithDeadline("/api/v1/quotes/indices", {}, 3_000);
    const assertion = expect(pending).rejects.toBeInstanceOf(FetchTimeoutError);
    await vi.advanceTimersByTimeAsync(3_000);
    await assertion;

    vi.useRealTimers();
  });

  it("경주에서 진 요청의 실패가 unhandledRejection이 되지 않는다", async () => {
    vi.useFakeTimers();
    let rejectIt: (err: unknown) => void = () => {};
    vi.stubGlobal(
      "fetch",
      vi.fn(() => new Promise<Response>((_res, rej) => { rejectIt = rej; })),
    );

    const pending = fetchWithDeadline("/api/v1/quotes", {}, 1_000);
    const assertion = expect(pending).rejects.toBeInstanceOf(FetchTimeoutError);
    await vi.advanceTimersByTimeAsync(1_000);
    await assertion;

    // 마감 뒤에 도착한 실패. 받아 주는 곳이 없으면 프로세스가 시끄러워진다.
    rejectIt(new Error("늦게 온 실패"));
    await vi.advanceTimersByTimeAsync(10);

    vi.useRealTimers();
  });
});

describe("재시도", () => {
  it("일시적 실패는 물러섰다 다시 한다", async () => {
    vi.useFakeTimers();
    const run = vi
      .fn()
      .mockRejectedValueOnce(new FetchTimeoutError(8_000))
      .mockResolvedValueOnce("ok");

    const pending = retryingFetch(run);
    await vi.advanceTimersByTimeAsync(2_000);
    await expect(pending).resolves.toBe("ok");
    expect(run).toHaveBeenCalledTimes(2);

    vi.useRealTimers();
  });

  it("4xx는 다시 묻지 않는다 — 2초 뒤에도 같은 답이다", async () => {
    const err = Object.assign(new Error("unauthorized"), { status: 401 });
    const run = vi.fn().mockRejectedValue(err);

    await expect(retryingFetch(run)).rejects.toBe(err);
    expect(run).toHaveBeenCalledTimes(1);
  });

  it("429도 여기서 물러서지 않는다 — 고칠 곳은 부르는 주기다", async () => {
    const err = Object.assign(new Error("rate limit"), { status: 429 });
    const run = vi.fn().mockRejectedValue(err);

    await expect(retryingFetch(run)).rejects.toBe(err);
    expect(run).toHaveBeenCalledTimes(1);
  });

  it("5xx는 다시 묻는다 — 서버 사정은 바뀔 수 있다", async () => {
    vi.useFakeTimers();
    const err = Object.assign(new Error("internal"), { status: 503 });
    const run = vi.fn().mockRejectedValue(err);

    const pending = retryingFetch(run);
    const assertion = expect(pending).rejects.toBe(err);
    await vi.advanceTimersByTimeAsync(10_000);
    await assertion;
    // 첫 시도 + 두 번의 재시도. 세 번째부터는 사용자가 기다리는 시간이 너무 길다.
    expect(run).toHaveBeenCalledTimes(3);

    vi.useRealTimers();
  });
});
