import { describe, expect, it } from "vitest";
import { condCategory, joinTargets, joinWays, myRate } from "@/lib/fin-conds";

describe("fin-conds", () => {
  it("조건 이름을 칩으로", () => {
    expect(condCategory("비대면 가입")).toBe("online");
    expect(condCategory("마케팅 전체동의")).toBe("marketing");
    expect(condCategory("최초 신규고객")).toBe("first");
    expect(condCategory("급여/연금 입금")).toBe("salary");
    expect(condCategory("카드 결제실적")).toBe("card");
    expect(condCategory("자동이체 출금실적")).toBe("autopay");
    expect(condCategory("주택청약 보유")).toBe("holding");
    expect(condCategory("영업점 재예치")).toBe("rollover");
    expect(condCategory("기본 우대금리")).toBe("auto");
    expect(condCategory("장애인 등록")).toBe("other");
  });

  it("내 금리 = 기본 + 고른 조건(+자동 가산), 최고금리 상한", () => {
    const conds = [
      { l: "비대면 가입", b: 0.1 },
      { l: "마케팅 동의", b: 0.2 },
      { l: "급여이체", b: 0.5 },
      { l: "기본 우대금리", b: 0.05 },
      { l: "카드 결제실적", b: 0 },
    ];
    expect(myRate(3.0, 3.8, conds, new Set())).toBe(3.05);
    expect(myRate(3.0, 3.8, conds, new Set(["online", "marketing"]))).toBe(3.35);
    expect(myRate(3.0, 3.5, conds, new Set(["online", "marketing", "salary"]))).toBe(3.5);
    expect(myRate(3.0, 3.8, undefined, new Set(["online"]))).toBe(3.0);
  });

  it("가입 방법·대상", () => {
    expect([...joinWays("영업점,인터넷,스마트폰")].sort()).toEqual(["branch", "online"]);
    expect([...joinWays("스마트폰")]).toEqual(["online"]);
    expect(joinTargets("만 19세~34세 청년").has("youth")).toBe(true);
    expect(joinTargets("만65세 이상 개인").has("senior")).toBe(true);
    expect(joinTargets("개인,개인사업자,법인").has("biz")).toBe(true);
    expect(joinTargets("실명의 개인\n(개인사업자 제외)").has("biz")).toBe(false);
    expect(joinTargets("제한없음").size).toBe(0);
  });
});
