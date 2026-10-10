import { describe, expect, it } from "vitest";
import { groupChanges, type ChangeEvent } from "./changes-api";

const ev = (over: Partial<ChangeEvent>): ChangeEvent => ({
  detectedFor: "2026-09-22",
  eventDate: "2026-09-22",
  symbol: "A",
  kind: "news_event",
  direction: 0,
  importance: 1,
  title: "t",
  ...over,
});

describe("groupChanges", () => {
  // 2026-10-09 요청 3 — 순서는 시가총액, 순위 번호는 1부터. 시총 없는 종목은 맨 뒤.
  it("groups by symbol, ranks by market cap, lines by importance", () => {
    const groups = groupChanges([
      ev({ symbol: "A", importance: 1.2, marketCap: 50 }),
      ev({ symbol: "B", importance: 3, kind: "price_move", explained: false, marketCap: 900 }),
      ev({ symbol: "C", importance: 9 }),
      ev({ symbol: "A", importance: 2.5, kind: "price_move", explained: true, marketCap: 50 }),
    ]);
    expect(groups.map((g) => [g.symbol, g.rank])).toEqual([
      ["B", 1],
      ["A", 2],
      ["C", 3],
    ]);
    expect(groups[1]!.events.map((e) => e.importance)).toEqual([2.5, 1.2]);
  });

  // 「설명 불가」는 급등락에서만 뜻이 있다 — 거래량 이벤트의 false로 묶음에 표시하지 않는다.
  it("flags unexplained only from price moves", () => {
    const [g] = groupChanges([ev({ kind: "volume_surge", explained: false })]);
    expect(g!.hasUnexplained).toBe(false);
    const [h] = groupChanges([ev({ kind: "price_move", explained: false })]);
    expect(h!.hasUnexplained).toBe(true);
  });
});
