import { describe, expect, it } from "vitest";
import { fitImageSize, splitLinks } from "@/lib/community-text";

describe("splitLinks — 평문 본문의 링크만 살린다", () => {
  it("http(s) 주소를 링크로, 나머지는 글자로", () => {
    expect(splitLinks("참고 https://a.com/x 입니다")).toEqual([
      { kind: "text", text: "참고 " },
      { kind: "link", text: "https://a.com/x", href: "https://a.com/x" },
      { kind: "text", text: " 입니다" },
    ]);
  });

  it("문장 끝 마침표는 링크에서 뗀다", () => {
    const segs = splitLinks("여기 https://a.com/x.");
    expect(segs[1]).toEqual({ kind: "link", text: "https://a.com/x", href: "https://a.com/x" });
    expect(segs[2]).toEqual({ kind: "text", text: "." });
  });

  it("javascript: 같은 스킴은 링크가 되지 않는다", () => {
    expect(splitLinks("javascript:alert(1)")).toEqual([{ kind: "text", text: "javascript:alert(1)" }]);
  });

  it("링크가 없으면 한 조각", () => {
    expect(splitLinks("낼이 배당 권리 막차?")).toEqual([{ kind: "text", text: "낼이 배당 권리 막차?" }]);
  });
});

describe("fitImageSize — 올리기 전 축소 크기", () => {
  it("긴 변을 1600으로 맞추고 비율을 지킨다", () => {
    expect(fitImageSize(4032, 3024)).toEqual({ w: 1600, h: 1200 });
    expect(fitImageSize(3024, 4032)).toEqual({ w: 1200, h: 1600 });
  });
  it("작으면 그대로 — 키우지 않는다", () => {
    expect(fitImageSize(800, 600)).toEqual({ w: 800, h: 600 });
  });
});
