import { describe, expect, it } from "vitest";
import {
  activeMentionAt,
  assetTokenText,
  fieldTagAlias,
  mentionTokenText,
  mentionValueChanged,
  plainMentions,
  splitMentions,
  type MentionToken,
} from "@/lib/mentions";

describe("splitMentions", () => {
  // 서버 해석 결과(service/mention_parse_test.go와 같은 문장).
  const tokens: MentionToken[] = [
    { raw: "@삼성전자가", len: 5, symbol: "005930" },
    { raw: "@삼성전자:점수가", len: 8, symbol: "005930", field: "score" },
    { raw: "#SK하이닉스", len: 7, symbol: "000660", card: true },
    { raw: "@[TIGER 미국S&P500]:PER", len: 21, symbol: "360750", field: "per" },
  ];

  it("cuts particles off and keeps unknown candidates as text", () => {
    const segs = splitMentions("@삼성전자가 좋고 @삼성전자:점수가 70, @없는종목 me@삼성전자.com", tokens);
    expect(segs.map((s) => (s.kind === "mention" ? `[${s.text}]` : s.text)).join("")).toBe(
      "[@삼성전자]가 좋고 [@삼성전자:점수]가 70, @없는종목 me@삼성전자.com",
    );
  });

  it("handles brackets and cards", () => {
    const segs = splitMentions("#SK하이닉스 와 @[TIGER 미국S&P500]:PER", tokens);
    const tags = segs.filter((s) => s.kind === "mention");
    expect(tags.map((s) => s.text)).toEqual(["#SK하이닉스", "@[TIGER 미국S&P500]:PER"]);
    expect(tags[0].kind === "mention" && tags[0].token.card).toBe(true);
  });

  it("is plain text without tokens", () => {
    expect(splitMentions("그냥 글", [])).toEqual([{ kind: "text", text: "그냥 글" }]);
  });
});

describe("editor helpers", () => {
  it("brackets names outside the tag charset", () => {
    expect(mentionTokenText("@", "삼성전자")).toBe("@삼성전자");
    expect(mentionTokenText("@", "TIGER 미국S&P500", "PER")).toBe("@[TIGER 미국S&P500]:PER");
    expect(mentionTokenText("#", "Alphabet Inc.")).toBe("#[Alphabet Inc.]");
  });

  it("picks a tag-safe alias", () => {
    expect(fieldTagAlias({ label: "종합 점수", aliases: ["점수"] })).toBe("점수");
    expect(fieldTagAlias({ label: "52주 신고가 대비", aliases: [] })).toBe("52주신고가대비");
    expect(fieldTagAlias({ label: "추세", aliases: ["추세점수"] })).toBe("추세");
  });

  it("finds the tag being typed", () => {
    expect(activeMentionAt("반도체 @삼성", 7)).toMatchObject({ sigil: "@", name: "삼성", start: 4 });
    expect(activeMentionAt("@삼성전자:PE", 9)).toMatchObject({ name: "삼성전자", field: "PE" });
    expect(activeMentionAt("a@b", 3)).toBeNull();
    expect(activeMentionAt("@[TIGER 미국", 10)).toMatchObject({ name: "TIGER 미국", bracket: true });
  });

  it("strips sigils for excerpts and compares at display precision", () => {
    expect(plainMentions("@삼성전자:점수 좋고 #[TIGER 미국S&P500]")).toBe("삼성전자 점수 좋고 TIGER 미국S&P500");
    expect(mentionValueChanged(70.2, 69.9, { decimals: 0 } as never)).toBe(false);
    expect(mentionValueChanged(70, 72, { decimals: 0 } as never)).toBe(true);
  });
});

describe("자산 태그 (2026-10-03)", () => {
  const text = "요즘 @아파트[송파구 헬리오시티]:전세가율이 높다. #상품[우리은행 WON플러스예금]";
  it("서버 토큰으로 자산 태그를 자른다(조사는 글자)", () => {
    const tokens = [
      { raw: "@아파트[송파구 헬리오시티]:전세가율이", len: Array.from("@아파트[송파구 헬리오시티]:전세가율").length, symbol: "", kind: "apt" as const, key: "11710-1", field: "apt.jeonse" },
      { raw: "#상품[우리은행 WON플러스예금]", len: Array.from("#상품[우리은행 WON플러스예금]").length, symbol: "", kind: "product" as const, key: "42", card: true },
    ];
    const segs = splitMentions(text, tokens);
    expect(segs.map((s) => s.text)).toEqual(["요즘 ", "@아파트[송파구 헬리오시티]:전세가율", "이 높다. ", "#상품[우리은행 WON플러스예금]"]);
  });
  it("요약 글에서는 종류 낱말·대괄호를 걷는다", () => {
    expect(plainMentions(text)).toBe("요즘 송파구 헬리오시티 전세가율이 높다. 우리은행 WON플러스예금");
  });
  it("편집기 토큰 글자", () => {
    expect(assetTokenText("@", "apt", "송파구 헬리오시티", "전세가율")).toBe("@아파트[송파구 헬리오시티]:전세가율");
    expect(assetTokenText("#", "rate", "국고채 3년")).toBe("#금리[국고채 3년]");
  });
});

describe("activeMentionAt — 자산 태그", () => {
  it("종류 낱말 + 대괄호 뒤 지표 자리", () => {
    const t = "글 @아파트[송파구 헬리오시티]:전세";
    const a = activeMentionAt(t, t.length);
    expect(a).toMatchObject({ sigil: "@", kind: "apt", name: "송파구 헬리오시티", field: "전세", bracket: true });
    expect(activeMentionAt("@삼성전자:점", 7)).toMatchObject({ name: "삼성전자", field: "점", kind: undefined });
  });
});
