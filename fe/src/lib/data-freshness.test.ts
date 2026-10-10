import { describe, expect, it } from "vitest";
import {
  freshnessOf,
  staleAgeLabel,
  tradeDateLabel,
  updatedAtLabel,
} from "@/lib/data-freshness";

const MIN = 60_000;
const NOW = Date.parse("2026-09-20T14:00:00Z");

function ago(ms: number): string {
  return new Date(NOW - ms).toISOString();
}

/*
이 파일이 잠그는 것은 **두 「기준」을 섞지 않는다**는 규칙이다
(`lib/data-freshness.ts` 머리글). 갱신 시각은 늙을 수 있고 거래일은 그럴 수 없다 —
후자에 벽시계를 들이대면 주말마다 앱 전체가 「낡음」이라고 자백한다.
*/
describe("갱신 기준시각의 신선도", () => {
  it("주기의 3배 안쪽이면 신선하다", () => {
    const f = freshnessOf(ago(2 * MIN), 1 * MIN, NOW);
    expect(f.stale).toBe(false);
    expect(f.ageMs).toBe(2 * MIN);
  });

  it("정확히 3배는 아직 신선하다 — 경계에서 배지가 깜빡이지 않게", () => {
    expect(freshnessOf(ago(3 * MIN), 1 * MIN, NOW).stale).toBe(false);
  });

  it("3배를 넘기면 낡았다", () => {
    expect(freshnessOf(ago(3 * MIN + 1), 1 * MIN, NOW).stale).toBe(true);
  });

  it("주기가 길면 임계값도 같이 길어진다 — 숫자를 새로 지어내지 않는다", () => {
    // 30분 주기(하루 한 번 도는 배치)는 90분까지 신선하다.
    expect(freshnessOf(ago(80 * MIN), 30 * MIN, NOW).stale).toBe(false);
    expect(freshnessOf(ago(100 * MIN), 30 * MIN, NOW).stale).toBe(true);
  });

  it("기준시각을 모르면 낡지 않았다 — 모름은 고장이 아니다", () => {
    for (const value of [null, undefined, "", "말도 안 되는 값"]) {
      const f = freshnessOf(value, 1 * MIN, NOW);
      expect(f.stale).toBe(false);
      expect(f.at).toBeNull();
      expect(f.ageMs).toBeNull();
    }
  });

  it("미래 시각은 나이가 0이다 — 서버·브라우저 시계 차이로 음수가 새지 않게", () => {
    const f = freshnessOf(ago(-30 * MIN), 1 * MIN, NOW);
    expect(f.ageMs).toBe(0);
    expect(f.stale).toBe(false);
  });
});

describe("거래일 표기", () => {
  /*
    **이 파일에 「거래일이 늙었는가」를 묻는 함수는 없다.** 일요일에 금요일 종가인
    것은 정상이고, 그걸 낡음으로 부르면 주말마다 거짓 경보가 난다 — 배치 감시가
    이미 한 번 밟은 함정이다(09-19 하루에 67회).
  */
  it("며칠 전 거래일도 그냥 표기한다", () => {
    expect(tradeDateLabel("2026-09-18")).toBe("09-18 종가 기준");
    expect(tradeDateLabel("2026-01-02T00:00:00Z")).toBe("01-02 종가 기준");
  });

  it("날짜가 아니면 아무것도 적지 않는다", () => {
    for (const value of [null, undefined, "", "live", "2026-9-8"]) {
      expect(tradeDateLabel(value)).toBeNull();
    }
  });
});

describe("사람이 읽는 문구", () => {
  it("기준시각은 서울 시간으로 적는다 — 해외에서 열어도 같은 값이 같은 이름이다", () => {
    // 2026-09-20T14:00:00Z = 서울 2026-09-20 23:00
    expect(updatedAtLabel(new Date(NOW))).toContain("11:00");
  });

  it("경과 시간은 분 · 시간 · 일로 올라간다", () => {
    expect(staleAgeLabel(12 * MIN)).toBe("12분 지남");
    expect(staleAgeLabel(3 * 60 * MIN)).toBe("3시간 지남");
    expect(staleAgeLabel(50 * 60 * MIN)).toBe("2일 지남");
  });

  it("1분 미만도 「1분」으로 적는다 — 「0분 지남」은 말이 안 된다", () => {
    expect(staleAgeLabel(5_000)).toBe("1분 지남");
  });

  it("나이를 모르면 배지를 안 그린다", () => {
    expect(staleAgeLabel(null)).toBeNull();
  });
});
