import { describe, expect, it } from "vitest";
import { buildingKey, groupByBuilding } from "@/lib/onbid-group";

describe("onbid-group", () => {
  it("층·호·숫자 동을 지운 주소가 건물 키", () => {
    const k = "강원특별자치도 원주시 지정면 가곡리 1352-2";
    expect(buildingKey(`${k} 제5층 제501호`)).toBe(k);
    expect(buildingKey(`${k} 제1층 제105호`)).toBe(k);
    expect(buildingKey("서울특별시 강남구 역삼동 123 101동 1203호")).toBe("서울특별시 강남구 역삼동 123");
    expect(buildingKey("경기도 성남시 분당구 정자동 1 지하1층 B102호")).toBe("경기도 성남시 분당구 정자동 1");
  });

  it("처음 나온 순서대로 묶고 빈 주소는 따로", () => {
    const g = groupByBuilding([
      { id: 1, address: "A동 1 제1층 제101호" },
      { id: 2, address: "B리 9" },
      { id: 3, address: "A동 1 제2층 제201호" },
      { id: 4, address: "" },
      { id: 5, address: "" },
    ]);
    expect(g.map((x) => [x.lead.id, x.rest.map((r) => r.id)])).toEqual([[1, [3]], [2, []], [4, []], [5, []]]);
  });
});
