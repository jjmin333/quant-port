import { afterEach, describe, expect, it, vi } from "vitest";
import type { QuoteSummary } from "@/lib/quotes-types";
import {
  __testing,
  batchQuotes,
  chunkSymbols,
  normalizeSymbols,
  pickSymbols,
  unionSymbols,
} from "./quote-batch";

function quote(symbol: string, price = 100): QuoteSummary {
  return {
    symbol,
    name: symbol,
    currentPrice: price,
    change: 0,
    changeRate: 0,
  } as QuoteSummary;
}

afterEach(() => {
  __testing.reset();
  vi.useRealTimers();
});

describe("normalizeSymbols", () => {
  it("공백을 털고 대문자로 맞추며 중복을 없앤다", () => {
    expect(normalizeSymbols([" aapl ", "AAPL", "nvda", ""])).toEqual(["AAPL", "NVDA"]);
  });
});

describe("unionSymbols", () => {
  it("처음 나온 순서를 지키며 합친다", () => {
    expect(
      unionSymbols([
        ["000660", "005930"],
        ["005930", "035420"],
        ["000660"],
      ]),
    ).toEqual(["000660", "005930", "035420"]);
  });

  it("빈 요청은 아무것도 더하지 않는다", () => {
    expect(unionSymbols([[], []])).toEqual([]);
  });
});

describe("chunkSymbols", () => {
  it("상한 이하면 한 덩어리", () => {
    expect(chunkSymbols(["A", "B"], 100)).toEqual([["A", "B"]]);
  });

  it("비었으면 부를 것도 없다", () => {
    expect(chunkSymbols([], 100)).toEqual([]);
  });

  it("상한을 넘기면 나눈다 — 서버 상한이 100종목이다", () => {
    const many = Array.from({ length: 250 }, (_, i) => `S${i}`);
    const chunks = chunkSymbols(many, 100);
    expect(chunks.map((c) => c.length)).toEqual([100, 100, 50]);
  });
});

describe("pickSymbols", () => {
  it("요청한 순서대로 골라내고, 없는 심볼은 뺀다", () => {
    const rows = new Map([
      ["A", quote("A")],
      ["B", quote("B")],
    ]);
    expect(pickSymbols(rows, ["B", "없음", "A"]).map((r) => r.symbol)).toEqual(["B", "A"]);
  });
});

describe("batchQuotes", () => {
  it("겹치는 목록 다섯을 요청 한 번으로 합친다 — 실측한 홈 화면 그대로", async () => {
    vi.useFakeTimers();
    const calls: string[][] = [];
    const fetcher = async (symbols: string[]) => {
      calls.push(symbols);
      return symbols.map((s) => quote(s));
    };

    // 2026-09-05 홈 실측에 나갔던 다섯 묶음.
    const promises = [
      batchQuotes(
        ["000660", "005930", "069500", "386380", "122630", "009150", "102110", "360750"],
        fetcher,
        5_000,
      ),
      batchQuotes(["000660", "005380", "005930"], fetcher, 5_000),
      batchQuotes(
        ["000950", "004960", "0091W0", "019175", "044780", "047080", "053950", "073570"],
        fetcher,
        5_000,
      ),
      batchQuotes(["ADBE", "CPRT", "FDX", "KR", "ORCL", "SUNB"], fetcher, 5_000),
      batchQuotes(["000660", "005930", "009150", "069500", "102110", "122630", "360750"], fetcher, 5_000),
    ];

    await vi.advanceTimersByTimeAsync(50);
    const results = await Promise.all(promises);

    expect(calls).toHaveLength(1);
    // 서른두 번 풀던 것이 스물세 종목이 된다(겹친 아홉이 빠진다).
    expect(calls[0]).toHaveLength(23);
    // 각 호출부는 자기가 물은 만큼만 받는다.
    expect(results[1]!.map((r) => r.symbol)).toEqual(["000660", "005380", "005930"]);
    expect(results[3]!).toHaveLength(6);
  });

  it("이미 받아 둔 종목만 물으면 요청이 아예 없다", async () => {
    vi.useFakeTimers();
    const fetcher = vi.fn(async (symbols: string[]) => symbols.map((s) => quote(s)));

    const first = batchQuotes(["AAPL", "NVDA"], fetcher, 5_000);
    await vi.advanceTimersByTimeAsync(50);
    await first;
    expect(fetcher).toHaveBeenCalledTimes(1);

    // 앞 요청의 부분집합 — 창을 기다리지도 않고 곧바로 답이 나온다.
    const again = await batchQuotes(["NVDA"], fetcher, 5_000);
    expect(fetcher).toHaveBeenCalledTimes(1);
    expect(again.map((r) => r.symbol)).toEqual(["NVDA"]);
  });

  it("하나라도 오래됐으면 다시 부른다", async () => {
    vi.useFakeTimers();
    const fetcher = vi.fn(async (symbols: string[]) => symbols.map((s) => quote(s)));

    const first = batchQuotes(["AAPL"], fetcher, 5_000);
    await vi.advanceTimersByTimeAsync(50);
    await first;

    await vi.advanceTimersByTimeAsync(6_000);
    const second = batchQuotes(["AAPL"], fetcher, 5_000);
    await vi.advanceTimersByTimeAsync(50);
    await second;
    expect(fetcher).toHaveBeenCalledTimes(2);
  });

  it("심볼이 없으면 부르지 않고 빈 배열", async () => {
    const fetcher = vi.fn(async () => []);
    expect(await batchQuotes([], fetcher, 5_000)).toEqual([]);
    expect(fetcher).not.toHaveBeenCalled();
  });

  it("실패하면 묶인 호출부가 모두 실패한다", async () => {
    vi.useFakeTimers();
    const fetcher = async () => {
      throw new Error("boom");
    };
    const a = batchQuotes(["AAPL"], fetcher, 5_000);
    const b = batchQuotes(["NVDA"], fetcher, 5_000);
    const settled = Promise.allSettled([a, b]);
    await vi.advanceTimersByTimeAsync(50);
    expect((await settled).map((r) => r.status)).toEqual(["rejected", "rejected"]);
  });

  it("상한을 넘기면 나눠 부르되 호출부에는 한 벌로 돌려준다", async () => {
    vi.useFakeTimers();
    const calls: number[] = [];
    const fetcher = async (symbols: string[]) => {
      calls.push(symbols.length);
      return symbols.map((s) => quote(s));
    };
    const many = Array.from({ length: 150 }, (_, i) => `S${i}`);
    const p = batchQuotes(many, fetcher, 5_000);
    await vi.advanceTimersByTimeAsync(50);
    const rows = await p;
    expect(calls).toEqual([100, 50]);
    expect(rows).toHaveLength(150);
  });
});
