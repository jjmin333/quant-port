import { describe, expect, it } from "vitest";
import { formatCarryClock } from "@/lib/news-market-briefing";

describe("formatCarryClock", () => {
  it("같은 날이면 시각만", () => {
    // 21:00 KST 문장을 23:00 KST에 물려받음
    expect(formatCarryClock("2026-09-25T12:00:00Z", "2026-09-25T14:00:00Z")).toBe("21:00");
  });
  it("자정을 넘기면 「어제」", () => {
    // 21:00 KST → 다음 날 01:12 KST
    expect(formatCarryClock("2026-09-25T12:00:00Z", "2026-09-25T16:12:00Z")).toBe("어제 21:00");
  });
  it("이틀 이상이면 날짜", () => {
    expect(formatCarryClock("2026-09-24T12:00:00Z", "2026-09-26T01:00:00Z")).toBe("9/24 21:00");
  });
  it("잘못된 값이면 빈 문자열", () => {
    expect(formatCarryClock("nope", "2026-09-26T01:00:00Z")).toBe("");
  });
});
