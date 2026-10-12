import { describe, expect, it } from "vitest";
import { newsTopicLabel, parseNewsTopicFilter } from "@/lib/news-types";

describe("news topic filter", () => {
  it("옛 칩 주소는 묶음 칩으로, 모르는 값은 null", () => {
    expect(parseNewsTopicFilter("rates")).toBe("finance");
    expect(parseNewsTopicFilter("loan")).toBe("finance");
    expect(parseNewsTopicFilter("finance")).toBe("finance");
    expect(parseNewsTopicFilter("stock")).toBe("stock");
    expect(parseNewsTopicFilter("bogus")).toBeNull();
    expect(parseNewsTopicFilter(null)).toBeNull();
  });
  it("기사 배지는 세부 topic 그대로", () => {
    expect(newsTopicLabel("deposit")).toBe("예·적금");
    expect(newsTopicLabel("stock")).toBeNull();
  });
});
