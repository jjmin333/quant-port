import { describe, expect, it } from "vitest";
import { communityShareHref } from "@/lib/community-share";

describe("communityShareHref", () => {
  it("기사 id·한국어 제목·종목(최대 3)을 싣는다", () => {
    const href = communityShareHref({
      id: "kr-123",
      headline: "Samsung",
      headlineKo: "삼성전자 3분기 30조 배당",
      symbols: ["005930", "000660", "NVDA", "AAPL"],
    });
    const q = new URL(href, "https://x").searchParams;
    expect(href.startsWith("/community/new?")).toBe(true);
    expect(q.get("article")).toBe("kr-123");
    expect(q.get("title")).toBe("삼성전자 3분기 30조 배당");
    expect(q.get("symbols")).toBe("005930,000660,NVDA");
  });
});
