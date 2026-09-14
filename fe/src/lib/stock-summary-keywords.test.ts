import { describe, expect, it } from "vitest";
import { usefulKeywords } from "@/lib/stock-summary-keywords";

describe("usefulKeywords", () => {
  it("종목명과 종목코드를 되풀이하는 태그를 버린다", () => {
    // 사용자 신고 (2026-09-03): SK하이닉스 개요 태그가 이 모양이었다.
    const got = usefulKeywords(
      ["SK하이닉스", "000660", "반도체", "성장성", "수익성", "밸류에이션", "내부자매수"],
      "000660",
      "SK하이닉스",
    );
    expect(got).toEqual(["반도체", "성장성", "수익성", "밸류에이션", "내부자매수"]);
  });

  it("띄어쓰기가 달라도 같은 이름으로 본다", () => {
    expect(usefulKeywords(["SK 하이닉스", "반도체"], "000660", "SK하이닉스")).toEqual(["반도체"]);
  });

  it("대소문자가 달라도 같은 이름으로 본다", () => {
    expect(usefulKeywords(["naver", "AI"], "035420", "NAVER")).toEqual(["AI"]);
  });

  it("같은 말이 두 번 오면 하나만 남긴다", () => {
    expect(usefulKeywords(["반도체", "반 도 체", "수출"])).toEqual(["반도체", "수출"]);
  });

  it("빈 값과 공백만 있는 태그는 버린다", () => {
    expect(usefulKeywords(["", "  ", "실적"])).toEqual(["실적"]);
  });

  it("종목 정보를 안 넘기면 아무것도 걸러내지 않는다", () => {
    expect(usefulKeywords(["SK하이닉스", "반도체"])).toEqual(["SK하이닉스", "반도체"]);
  });

  it("태그가 없으면 빈 배열이다", () => {
    expect(usefulKeywords(undefined, "000660", "SK하이닉스")).toEqual([]);
  });

  it("순서는 받은 그대로다 — 모델이 중요한 것을 앞에 놓는다", () => {
    expect(usefulKeywords(["가", "나", "다"])).toEqual(["가", "나", "다"]);
  });
});
