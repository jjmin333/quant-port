import { describe, expect, it } from "vitest";
import type { UpcomingEvent } from "@/hooks/use-upcoming-events";
import type { PriceTarget } from "@/lib/price-targets";
import {
  attentionScore,
  buildWatchSignals,
  watchBucket,
  type WatchSignal,
} from "@/lib/watchlist-signals";
import type { WatchlistRow } from "@/lib/watchlist-calc";

function row(over: Partial<WatchlistRow> = {}): WatchlistRow {
  return {
    symbol: "005930",
    quote: {
      symbol: "005930",
      name: "삼성전자",
      market: "KOSPI",
      currency: "KRW",
      assetType: "domestic",
      currentPrice: 100_000,
      change: 0,
      changeRate: 0,
      volume: 0,
      tradingValue: 0,
      isWatchlisted: true,
      isHeld: false,
    },
    score: 60,
    rangePct: 50,
    sinceAddedRate: 0,
    addedPrice: 100_000,
    addedAt: new Date().toISOString(),
    ...over,
  };
}

function target(over: Partial<PriceTarget> = {}): PriceTarget {
  return {
    id: "1",
    symbol: "005930",
    targetPrice: 200_000,
    currency: "KRW",
    note: "",
    createdAt: new Date().toISOString(),
    ...over,
  };
}

/** 오늘로부터 n일 뒤의 KST 날짜 문자열. */
function inDays(n: number): string {
  const t = new Date(Date.now() + n * 86_400_000);
  return t.toLocaleDateString("en-CA", { timeZone: "Asia/Seoul" });
}

function event(days: number): UpcomingEvent {
  return {
    symbol: "005930",
    name: "삼성전자",
    date: inDays(days),
    label: "실적",
    source: "fnguide",
    kind: "upcoming",
  };
}

function ids(signals: WatchSignal[]): string[] {
  return signals.map((s) => s.id);
}

describe("buildWatchSignals", () => {
  it("평범한 종목은 아무 말도 하지 않는다", () => {
    expect(buildWatchSignals({ row: row() })).toEqual([]);
  });

  it("내가 그어 둔 선에 닿은 것이 가장 앞에 온다", () => {
    const signals = buildWatchSignals({
      row: row({ quote: { ...row().quote!, currentPrice: 100_000, changeRate: -4 } }),
      target: target({ stopPrice: 101_000, targetPrice: 200_000 }),
    });
    expect(ids(signals)[0]).toBe("stop-hit");
  });

  it("목표가 근접은 5% 안쪽에서만 뜬다", () => {
    const near = buildWatchSignals({ row: row(), target: target({ targetPrice: 104_000 }) });
    const far = buildWatchSignals({ row: row(), target: target({ targetPrice: 130_000 }) });
    expect(ids(near)).toContain("target-near");
    expect(ids(far)).toEqual([]);
  });

  it("일정은 가까울수록 무겁고, 2주를 넘으면 말하지 않는다", () => {
    const soon = buildWatchSignals({ row: row(), events: [event(2)] });
    const later = buildWatchSignals({ row: row(), events: [event(10)] });
    expect(soon[0].weight).toBeGreaterThan(later[0].weight);
    expect(buildWatchSignals({ row: row(), events: [event(40)] })).toEqual([]);
  });

  it("전일 등락은 문턱을 넘은 것만 말한다", () => {
    const quiet = row({ quote: { ...row().quote!, changeRate: 1.2 } });
    const loud = row({ quote: { ...row().quote!, changeRate: -6.4 } });
    expect(ids(buildWatchSignals({ row: quiet }))).toEqual([]);
    expect(ids(buildWatchSignals({ row: loud }))).toContain("move");
  });

  it("52주 양 끝만 신호가 된다", () => {
    expect(ids(buildWatchSignals({ row: row({ rangePct: 98 }) }))).toContain("high52");
    expect(ids(buildWatchSignals({ row: row({ rangePct: 2 }) }))).toContain("low52");
    expect(ids(buildWatchSignals({ row: row({ rangePct: 60 }) }))).toEqual([]);
  });

  it("오래 담아 두고 제자리인 것은 다른 신호가 없을 때만 말한다", () => {
    const old = new Date(Date.now() - 200 * 86_400_000).toISOString();
    expect(ids(buildWatchSignals({ row: row({ addedAt: old, sinceAddedRate: 0.4 }) }))).toEqual([
      "stale",
    ]);
    const busy = buildWatchSignals({
      row: row({ addedAt: old, sinceAddedRate: 0.4, rangePct: 99 }),
    });
    expect(ids(busy)).not.toContain("stale");
  });
});

describe("watchBucket", () => {
  it("가장 무거운 신호가 칸을 정한다 — 약한 신호를 모아도 「지금 볼 것」이 되지 않는다", () => {
    const weak = buildWatchSignals({
      row: row({ rangePct: 1, score: 20, sinceAddedRate: -14 }),
    });
    expect(watchBucket(weak)).toBe("watch");

    const strong = buildWatchSignals({ row: row(), target: target({ targetPrice: 99_000 }) });
    expect(watchBucket(strong)).toBe("action");
    expect(watchBucket([])).toBe("quiet");
  });
});

describe("attentionScore", () => {
  it("가장 무거운 신호가 지배하고 나머지는 보탠다", () => {
    const one = buildWatchSignals({ row: row({ rangePct: 99 }) });
    const many = buildWatchSignals({ row: row({ rangePct: 99, score: 88 }) });
    expect(attentionScore(many)).toBeGreaterThan(attentionScore(one));
    expect(attentionScore(many)).toBeLessThan(
      many.reduce((sum, s) => sum + s.weight, 0),
    );
  });
});
