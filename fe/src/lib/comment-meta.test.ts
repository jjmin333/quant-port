import { describe, expect, it } from "vitest";
import { activityBadge, relativeTime } from "@/lib/comment-meta";

describe("activityBadge", () => {
  it("구간 경계에서 한 칸 아래로 접는다", () => {
    expect(activityBadge(9)).toBeNull();
    expect(activityBadge(10)).toBe("10+");
    expect(activityBadge(29)).toBe("10+");
    expect(activityBadge(30)).toBe("30+");
    expect(activityBadge(99)).toBe("30+");
    expect(activityBadge(100)).toBe("100+");
    expect(activityBadge(299)).toBe("100+");
    expect(activityBadge(300)).toBe("300+");
    expect(activityBadge(9999)).toBe("1000+");
  });

  /*
    「2+」는 「처음 보는 사람」과 구분되지 않는데, 배지가 붙는 것 자체가 신호라
    아무 신호도 없는 것보다 나쁘다.
  */
  it("활동이 적으면 배지를 달지 않는다", () => {
    expect(activityBadge(0)).toBeNull();
    expect(activityBadge(undefined)).toBeNull();
    expect(activityBadge(1)).toBeNull();
  });
});

describe("relativeTime", () => {
  const now = Date.parse("2026-09-14T12:00:00Z");
  const ago = (ms: number) => new Date(now - ms).toISOString();

  it("분·시간 단위로 말한다", () => {
    expect(relativeTime(ago(30 * 1000), now)).toBe("방금");
    expect(relativeTime(ago(60 * 1000), now)).toBe("1분 전");
    expect(relativeTime(ago(59 * 60 * 1000), now)).toBe("59분 전");
    expect(relativeTime(ago(60 * 60 * 1000), now)).toBe("1시간 전");
    expect(relativeTime(ago(23 * 3600 * 1000), now)).toBe("23시간 전");
  });

  /*
    하루가 넘으면 null — 화면이 날짜를 그대로 쓴다. 「9일 전」은 그게 언제인지
    세어 봐야 알고, 댓글은 보통 글과 같은 주에 몰린다.
  */
  it("하루가 넘으면 상대 시각을 포기한다", () => {
    expect(relativeTime(ago(24 * 3600 * 1000), now)).toBeNull();
    expect(relativeTime(ago(40 * 24 * 3600 * 1000), now)).toBeNull();
  });

  it("시계가 어긋나 미래로 보여도 음수를 내놓지 않는다", () => {
    expect(relativeTime(new Date(now + 3600 * 1000).toISOString(), now)).toBe("방금");
  });

  it("못 읽는 값에는 아무 말도 하지 않는다", () => {
    expect(relativeTime("", now)).toBeNull();
    expect(relativeTime("어제", now)).toBeNull();
  });
});
