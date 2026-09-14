import { describe, expect, it } from "vitest";
import {
  clearFlashDirection,
  IDLE_FLASH,
  nextFlashState,
  resetFlashState,
  type FlashState,
} from "@/lib/value-flash";

describe("nextFlashState", () => {
  it("첫 값에는 색이 없다 — 비교할 앞 값이 없다", () => {
    const s = nextFlashState(IDLE_FLASH, 100);
    expect(s.direction).toBeNull();
    expect(s.seen).toBe(100);
  });

  it("값이 오르면 up, 내리면 down", () => {
    let s = nextFlashState(IDLE_FLASH, 100);
    s = nextFlashState(s, 101);
    expect(s.direction).toBe("up");
    s = nextFlashState(s, 99);
    expect(s.direction).toBe("down");
  });

  it("값이 그대로면 **같은 상태 객체**를 돌려준다", () => {
    /*
      2026-09-04 사용자 지적 3. 서버가 값을 안 바꾸고 프레임만 보내도 예전 화면은
      그것을 갱신으로 세어 색을 칠했다. 참조까지 같아야 호출부가 리렌더를 건너뛴다.
    */
    const first = nextFlashState(IDLE_FLASH, 100);
    const second = nextFlashState(first, 100);
    expect(second).toBe(first);
  });

  it("리렌더가 몇 번 돌아도 seq는 안 올라간다", () => {
    let s = nextFlashState(IDLE_FLASH, 100);
    for (let i = 0; i < 50; i++) s = nextFlashState(s, 100);
    expect(s.seq).toBe(0);
  });

  it("같은 방향으로 연달아 움직이면 seq가 그때마다 올라간다", () => {
    // 방향만 보면 두 번째 상승에서 타이머가 다시 안 걸려 색이 일찍 꺼진다.
    let s = nextFlashState(IDLE_FLASH, 100);
    s = nextFlashState(s, 101);
    const firstSeq = s.seq;
    s = nextFlashState(s, 102);
    expect(s.direction).toBe("up");
    expect(s.seq).toBe(firstSeq + 1);
  });

  it("값이 없는 프레임은 기준을 건드리지 않는다", () => {
    // 로딩 한 프레임을 「0으로 떨어졌다」로 읽으면 목록이 통째로 파래진다.
    const s = nextFlashState(IDLE_FLASH, 100);
    expect(nextFlashState(s, null)).toBe(s);
    expect(nextFlashState(s, undefined)).toBe(s);
    expect(nextFlashState(s, Number.NaN)).toBe(s);
  });

  it("첫 값이 오기 전의 빈 프레임도 기준을 안 만든다", () => {
    expect(nextFlashState(IDLE_FLASH, null)).toBe(IDLE_FLASH);
  });
});

describe("resetFlashState", () => {
  it("다른 종목으로 갈아 끼울 때 색 없이 기준만 옮긴다", () => {
    // 7만원(삼성전자) → 25만원(SK하이닉스)에 상승 플래시가 뜨면 안 된다.
    const prev: FlashState = { seen: 70_000, direction: "up", seq: 3 };
    const s = resetFlashState(250_000);
    expect(s.direction).toBeNull();
    expect(s.seen).toBe(250_000);
    // 다음 틱이 같은 값이면 여전히 색이 없다.
    expect(nextFlashState(s, 250_000)).toBe(s);
    expect(prev.seen).toBe(70_000);
  });

  it("갈아 끼운 값이 비어 있으면 기준도 비운다", () => {
    expect(resetFlashState(undefined).seen).toBeNull();
  });
});

describe("clearFlashDirection", () => {
  it("방향만 지우고 기준값은 남긴다", () => {
    let s = nextFlashState(IDLE_FLASH, 100);
    s = nextFlashState(s, 101);
    const cleared = clearFlashDirection(s);
    expect(cleared.direction).toBeNull();
    expect(cleared.seen).toBe(101);
  });

  it("이미 색이 없으면 같은 객체다", () => {
    const s = nextFlashState(IDLE_FLASH, 100);
    expect(clearFlashDirection(s)).toBe(s);
  });
});
