import { describe, expect, it } from "vitest";
import { formatQuoteTabTitle, quoteChangeRate } from "@/lib/quote-tab-title";

/*
사용자가 보낸 두 장의 사진이 이 시험의 기준이다:

	$141.91 -1.98% | 오라클
	1,703,000원 +0.35% | SK하이닉스
*/

describe("formatQuoteTabTitle", () => {
  it("해외 종목 — 사진 그대로", () => {
    expect(
      formatQuoteTabTitle({ name: "오라클", price: 141.91, prevClose: 144.77, currency: "USD" }),
    ).toBe("$141.91 -1.98% | 오라클");
  });

  it("국내 종목 — 사진 그대로", () => {
    expect(
      formatQuoteTabTitle({
        name: "SK하이닉스",
        price: 1_703_000,
        prevClose: 1_697_000,
        currency: "KRW",
      }),
    ).toBe("1,703,000원 +0.35% | SK하이닉스");
  });

  /*
    환율을 안 넘기므로 `formatPrice`가 원화 병기를 붙이지 않는다. 붙으면
    `$141.91 (196,000원) -1.98% | 오라클`이 되어 탭에서 종목명이 먼저 잘린다.
  */
  it("해외 가격에 원화를 병기하지 않는다 — 탭은 좁다", () => {
    const title = formatQuoteTabTitle({
      name: "오라클",
      price: 141.91,
      prevClose: 144.77,
      currency: "USD",
    });
    expect(title).not.toContain("원");
  });

  it("보합은 부호 없이 0.00%", () => {
    expect(
      formatQuoteTabTitle({ name: "삼성전자", price: 70_000, prevClose: 70_000, currency: "KRW" }),
    ).toBe("70,000원 0.00% | 삼성전자");
  });

  /*
    0.00%로 적으면 「오늘 안 움직였다」는 거짓말이 된다 — 이 줄이 하는 유일한 일을
    뒤집는 값이라 등락률을 통째로 뺀다.
  */
  it("기준가를 모르면 등락률을 빼고 가격만 적는다", () => {
    expect(
      formatQuoteTabTitle({ name: "삼성전자", price: 70_000, prevClose: 0, currency: "KRW" }),
    ).toBe("70,000원 | 삼성전자");
  });

  it("아직 값이 없으면 빈 문자열 — 호출부가 원래 제목을 그대로 둔다", () => {
    expect(
      formatQuoteTabTitle({ name: "삼성전자", price: 0, prevClose: 70_000, currency: "KRW" }),
    ).toBe("");
    expect(
      formatQuoteTabTitle({ name: "  ", price: 70_000, prevClose: 69_000, currency: "KRW" }),
    ).toBe("");
  });
});

describe("quoteChangeRate", () => {
  /*
    스냅샷이 준 등락률을 그대로 쓰면 「가격은 움직이는데 등락률만 멈춰 있다」가 된다.
    스트림은 가격만 실어 주므로 기준가로 매번 다시 세야 한다.
  */
  it("새 가격과 전일 종가로 다시 센다", () => {
    expect(quoteChangeRate(110, 100)).toBeCloseTo(10, 10);
    expect(quoteChangeRate(90, 100)).toBeCloseTo(-10, 10);
  });

  it("기준가나 가격이 없으면 null", () => {
    expect(quoteChangeRate(110, 0)).toBeNull();
    expect(quoteChangeRate(0, 100)).toBeNull();
    expect(quoteChangeRate(110, -1)).toBeNull();
  });
});
