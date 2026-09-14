import { describe, expect, it } from "vitest";

import {
  compactShowsDetail,
  shouldHistoryBackOnCompactBack,
  shouldPinListOnParamsChange,
  symbolSelectNavMode,
} from "@/lib/market-radar-mobile-nav";

describe("compactShowsDetail", () => {
  it("데스크톱은 항상 false", () => {
    expect(compactShowsDetail(false, "000660", false)).toBe(false);
    expect(compactShowsDetail(false, "000660", true)).toBe(false);
  });

  it("compact + symbol → 상세", () => {
    expect(compactShowsDetail(true, "000660", false)).toBe(true);
  });

  it("compact + symbol 없음 → 목록", () => {
    expect(compactShowsDetail(true, "", false)).toBe(false);
    expect(compactShowsDetail(true, "   ", false)).toBe(false);
  });

  it("필터 변경 후 listPinned면 URL에 symbol이 있어도 목록", () => {
    expect(compactShowsDetail(true, "000660", true)).toBe(false);
  });
});

describe("symbolSelectNavMode", () => {
  it("데스크톱은 replace", () => {
    expect(symbolSelectNavMode(false, "")).toBe("replace");
    expect(symbolSelectNavMode(false, "000660")).toBe("replace");
  });

  it("compact 목록→상세는 push", () => {
    expect(symbolSelectNavMode(true, "")).toBe("push");
  });

  it("compact 상세 내 종목 전환은 replace", () => {
    expect(symbolSelectNavMode(true, "000660")).toBe("replace");
  });
});

describe("shouldHistoryBackOnCompactBack", () => {
  it("symbol이 있을 때만 back", () => {
    expect(shouldHistoryBackOnCompactBack("000660")).toBe(true);
    expect(shouldHistoryBackOnCompactBack("")).toBe(false);
    expect(shouldHistoryBackOnCompactBack("  ")).toBe(false);
  });
});

describe("shouldPinListOnParamsChange", () => {
  it("compact에서 paramsKey가 바뀌면 목록 고정", () => {
    expect(shouldPinListOnParamsChange("KR|A|realtime", "US|A|realtime", true)).toBe(true);
  });

  it("같은 paramsKey면 핀하지 않음", () => {
    expect(shouldPinListOnParamsChange("KR|A|realtime", "KR|A|realtime", true)).toBe(false);
  });

  it("데스크톱은 핀하지 않음", () => {
    expect(shouldPinListOnParamsChange("KR|A|realtime", "US|A|realtime", false)).toBe(false);
  });
});
