import { describe, expect, it } from "vitest";
import {
  formatRankingClock,
  rankingClock,
  rankingClockHint,
} from "@/lib/market-radar-clock";

describe("rankingClock", () => {
  it("가격을 받아 온 시각이 있으면 그것을 쓴다", () => {
    expect(
      rankingClock({
        pricesAt: "2026-09-03T13:05:07.000Z",
        refreshedAt: "2026-09-03T13:00:00.000Z",
        rankedAt: "2026-09-03T12:55:00.000Z",
      }),
    ).toEqual({ at: "2026-09-03T13:05:07.000Z", basis: "prices" });
  });

  it("장이 닫혀 있으면 마감 시각이 이긴다 — 스냅샷 시각은 「지금」으로 읽힌다", () => {
    expect(
      rankingClock({
        pricesAt: null,
        // KST 2026-09-03 20:00 — 국내 시간외까지 끝나는 시각.
        sessionClosedAt: "2026-09-03T11:00:00.000Z",
        refreshedAt: "2026-09-03T15:40:00.000Z",
        rankedAt: "2026-09-03T15:35:00.000Z",
      }),
    ).toEqual({ at: "2026-09-03T11:00:00.000Z", basis: "closed" });
  });

  it("시장을 막 바꿔 앞 시장의 pricesAt이 남아 있어도 마감이 이긴다", () => {
    expect(
      rankingClock({
        pricesAt: "2026-09-03T15:59:00.000Z",
        sessionClosedAt: "2026-09-03T11:00:00.000Z",
      })?.basis,
    ).toBe("closed");
  });

  it("마감 시각을 못 받으면(달력 없음) 스냅샷 시각으로 되돌아간다", () => {
    expect(
      rankingClock({
        pricesAt: null,
        refreshedAt: "2026-09-03T13:00:00.000Z",
        rankedAt: "2026-09-03T12:55:00.000Z",
      }),
    ).toEqual({ at: "2026-09-03T13:00:00.000Z", basis: "snapshot" });
  });

  it("refreshedAt이 없으면 rankedAt이 대신한다", () => {
    expect(rankingClock({ pricesAt: null, rankedAt: "2026-09-03T12:55:00.000Z" })).toEqual({
      at: "2026-09-03T12:55:00.000Z",
      basis: "snapshot",
    });
  });

  it("아무 시각도 없으면 찍지 않는다", () => {
    expect(rankingClock({ pricesAt: null })).toBeNull();
    expect(
      rankingClock({ pricesAt: "  ", sessionClosedAt: "", refreshedAt: "", rankedAt: null }),
    ).toBeNull();
  });
});

describe("formatRankingClock", () => {
  it("KST 시·분·초로 찍는다", () => {
    // 2026-09-03T13:05:07Z = KST 22:05:07
    expect(formatRankingClock("2026-09-03T13:05:07.000Z")).toContain("10:05:07");
  });

  it("마감 시각에는 날짜를 앞에 붙인다 — 자정을 넘겨 보는 값이다", () => {
    // 2026-09-03T11:00:00Z = KST 09-03 20:00:00
    const label = formatRankingClock("2026-09-03T11:00:00.000Z", "closed");
    expect(label).toContain("09.03");
    expect(label).toContain("08:00:00");
    // 「09. 03. 오후 …」처럼 점 뒤 공백이 남으면 좁은 레일에서 줄이 밀린다.
    expect(label).not.toMatch(/\.\s+\d/);
  });

  it("깨진 값은 빈 문자열이다 — 화면에 Invalid Date를 내보내지 않는다", () => {
    expect(formatRankingClock("어제")).toBe("");
    expect(formatRankingClock("어제", "closed")).toBe("");
  });
});

describe("rankingClockHint", () => {
  it("실시간 시세와 순위 갱신을 구분해 말한다", () => {
    expect(rankingClockHint("prices", "5분 주기")).toBe("실시간 시세 갱신");
    expect(rankingClockHint("snapshot", "5분 주기")).toBe("5분 주기 순위 갱신");
  });

  it("마감에는 값이 바뀌지 않는다는 것까지 말한다", () => {
    expect(rankingClockHint("closed", "5분 주기")).toContain("장 마감");
  });
});
