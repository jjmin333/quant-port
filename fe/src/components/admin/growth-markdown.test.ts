import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import { GrowthMarkdown } from "./growth-markdown";

const render = (source: string) => renderToStaticMarkup(createElement(GrowthMarkdown, { source }));

describe("GrowthMarkdown", () => {
  it("draws headings, tables and the escaped pipe", () => {
    const html = render(
      ["## 검색어 변화", "", "| 검색어 | 노출 |", "| --- | --- |", "| a \\| b | 3 → 5 |"].join("\n"),
    );
    expect(html).toContain("검색어 변화");
    expect(html).toContain("<table");
    expect(html).toContain("<td class=\"max-w-[28rem] px-2 py-1.5\">a | b</td>");
    expect(html).not.toContain("---");
  });

  it("never turns report text into markup", () => {
    // 검색어는 외부 사람이 친 글자다 — 보고서에 그대로 실린다.
    const html = render("| 검색어 |\n| --- |\n| <img src=x onerror=alert(1)> |");
    expect(html).not.toContain("<img");
    expect(html).toContain("&lt;img");
  });

  it("keeps underscores inside a full-line footnote", () => {
    const html = render("_worker-growth는 growth_* 표만 쓴다._");
    expect(html).toContain("growth_* 표만 쓴다.");
    expect(html).not.toContain("<em");
  });

  it("renders inline bold, code and lists", () => {
    const html = render("- **굵게** 와 `코드`\n1. 첫째");
    expect(html).toContain("<strong>굵게</strong>");
    expect(html).toContain("코드</code>");
    expect(html).toContain("<ol");
  });
});
