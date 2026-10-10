import { describe, expect, it } from "vitest";
import { peerGroupLabel } from "./score-explain-api";

describe("peerGroupLabel", () => {
  it("reads the cap groups as before", () => {
    expect(peerGroupLabel("domestic:KOSPI:cap3")).toBe("국내 KOSPI 대형주");
  });

  // 재무 지표의 섹터 그룹(2026-09-23) — 배치 내부 키가 화면에 그대로 서면 안 된다.
  it("reads sector groups", () => {
    expect(peerGroupLabel("domestic:information_technology")).toBe("국내 IT");
    expect(peerGroupLabel("us:health_care")).toBe("미국 헬스케어");
  });
});
