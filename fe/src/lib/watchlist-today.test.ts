import { describe, expect, it } from "vitest";
import { summarizeWatchlistToday, type WatchlistTodayInput } from "@/lib/watchlist-today";

const row = (over: Partial<WatchlistTodayInput> & { name: string }): WatchlistTodayInput => ({
  symbol: over.name,
  eventDays: null,
  ...over,
});
const today = (over: { scoreDelta?: number; news24h?: number }) => ({
  symbol: "X",
  name: "X",
  assetType: "domestic",
  held: false,
  watched: true,
  news24h: over.news24h ?? 0,
  scoreDelta: over.scoreDelta,
});

describe("summarizeWatchlistToday — 관심 종목 상단 띠 (2026-09-27 결정 A)", () => {
  it("오른 것·내린 것을 센다 — 보합과 시세 없음은 어느 쪽도 아니다", () => {
    const s = summarizeWatchlistToday([
      row({ name: "A", changeRate: 1.2 }),
      row({ name: "B", changeRate: -0.4 }),
      row({ name: "C", changeRate: 0 }),
      row({ name: "D" }),
    ]);
    expect(s).toMatchObject({ total: 4, up: 1, down: 1 });
  });

  it("점수 변화는 폭이 가장 큰 한 종목만, 3점 미만은 잡음이라 뺀다", () => {
    const s = summarizeWatchlistToday([
      row({ name: "A", today: today({ scoreDelta: 2 }) }),
      row({ name: "B", today: today({ scoreDelta: -7 }) }),
      row({ name: "C", today: today({ scoreDelta: 5 }) }),
    ]);
    expect(s.scoreMover).toEqual({ name: "B", delta: -7 });
    expect(summarizeWatchlistToday([row({ name: "A", today: today({ scoreDelta: 2 }) })]).scoreMover).toBeUndefined();
  });

  it("뉴스 최다 · 가장 가까운 일정(14일 안만)", () => {
    const s = summarizeWatchlistToday([
      row({ name: "A", today: today({ news24h: 12 }), eventDays: 20 }),
      row({ name: "B", today: today({ news24h: 165 }), eventDays: 9 }),
      row({ name: "C", today: today({ news24h: 0 }), eventDays: 3 }),
    ]);
    expect(s.newsLeader).toEqual({ name: "B", count: 165 });
    expect(s.nextEvent).toEqual({ name: "C", days: 3 });
    expect(summarizeWatchlistToday([row({ name: "A", eventDays: 15 })]).nextEvent).toBeUndefined();
  });
});
