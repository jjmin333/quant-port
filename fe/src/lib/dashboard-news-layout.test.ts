import { describe, expect, it } from "vitest";

import { newsCardSharesBriefingRow } from "@/lib/dashboard-news-layout";

describe("newsCardSharesBriefingRow", () => {
  it("브리핑이 하나라도 있으면 한 행을 나눠 쓴다", () => {
    expect(
      newsCardSharesBriefingRow({ loading: false, hasDomestic: true, hasUs: false }),
    ).toBe(true);
    expect(
      newsCardSharesBriefingRow({ loading: false, hasDomestic: false, hasUs: true }),
    ).toBe(true);
    expect(
      newsCardSharesBriefingRow({ loading: false, hasDomestic: true, hasUs: true }),
    ).toBe(true);
  });

  it("받는 중에는 자리를 비워 두지 않는다", () => {
    expect(
      newsCardSharesBriefingRow({ loading: true, hasDomestic: false, hasUs: false }),
    ).toBe(true);
  });

  /* 이것이 「PC에서 다섯 줄」의 조건이다 — 브리핑이 없으면 뉴스가 행을 혼자 쓴다. */
  it("브리핑이 둘 다 없으면 뉴스가 행을 혼자 쓴다", () => {
    expect(
      newsCardSharesBriefingRow({ loading: false, hasDomestic: false, hasUs: false }),
    ).toBe(false);
  });
});
