import { describe, expect, it } from "vitest";
import {
  applyListPriceTick,
  applyListPriceTicks,
  changeRateFrom,
  reconnectDelayMs,
  streamSymbolsKey,
  symbolsFromKey,
  type ListPriceMap,
} from "./quote-stream-state";

const tick = (symbol: string, price: number, updatedAt: string) => ({
  symbol,
  price,
  updatedAt,
});

describe("streamSymbolsKey", () => {
  it("같은 집합이면 순서가 달라도 같은 키다", () => {
    // 순위는 목록이 갱신될 때마다 흔들린다. 그때마다 SSE를 다시 연결하면
    // 연결 하나로 아끼려던 것이 사라진다.
    expect(streamSymbolsKey(["AAPL", "MSFT"])).toBe(streamSymbolsKey(["MSFT", "AAPL"]));
  });

  it("대소문자·공백·중복을 정규화한다", () => {
    expect(streamSymbolsKey([" aapl ", "AAPL", "msft"])).toBe("AAPL,MSFT");
  });

  it("빈 값은 버린다 — 빈 심볼이 섞이면 서버가 400을 준다", () => {
    expect(streamSymbolsKey(["", "  ", "AAPL"])).toBe("AAPL");
  });

  it("키에서 목록을 되돌릴 수 있다", () => {
    expect(symbolsFromKey(streamSymbolsKey(["b", "a"]))).toEqual(["A", "B"]);
    expect(symbolsFromKey("")).toEqual([]);
  });
});

describe("applyListPriceTick", () => {
  it("새 가격을 넣는다", () => {
    const next = applyListPriceTick(new Map(), tick("AAPL", 100, "2026-09-04T00:00:01Z"));
    expect(next.get("AAPL")?.price).toBe(100);
  });

  it("심볼을 대문자로 맞춘다 — 조회하는 쪽이 대문자를 쓴다", () => {
    const next = applyListPriceTick(new Map(), tick("aapl", 100, "2026-09-04T00:00:01Z"));
    expect(next.get("AAPL")?.price).toBe(100);
  });

  it("늦게 도착한 값이 새 값을 덮지 않는다", () => {
    // 스냅샷과 스트림이 엇갈리면 방금 받은 가격 위에 1초 전 값이 얹혀 숫자가 뒤로 튄다.
    let map: ListPriceMap = new Map();
    map = applyListPriceTick(map, tick("AAPL", 200, "2026-09-04T00:00:05Z"));
    map = applyListPriceTick(map, tick("AAPL", 100, "2026-09-04T00:00:01Z"));
    expect(map.get("AAPL")?.price).toBe(200);
  });

  it("값이 그대로면 같은 맵을 돌려준다", () => {
    // 참조가 바뀌면 100줄짜리 목록이 2초마다 통째로 다시 그려진다.
    const first = applyListPriceTick(new Map(), tick("AAPL", 100, "2026-09-04T00:00:01Z"));
    const second = applyListPriceTick(first, tick("AAPL", 100, "2026-09-04T00:00:01Z"));
    expect(second).toBe(first);
  });

  it("시각만 새롭고 가격이 같으면 갱신으로 세지 않는다", () => {
    /*
      2026-09-04 사용자 지적 3의 회귀 테스트.

      서버가 붙이는 `updatedAt`은 「캐시에 쓴 시각」이라 **값이 안 변해도 틱마다
      달라진다.** 예전 판정은 시각까지 같아야 그대로 뒀기 때문에, 조용한 종목도
      틱마다 새 엔트리가 되고 `seq`가 올라가 화면이 셀을 깜빡였다 — 가격은
      그대로인데 사용자에게는 체결이 일어난 것처럼 보인다.
    */
    const first = applyListPriceTick(new Map(), tick("AAPL", 100, "2026-09-04T00:00:01Z"));
    const second = applyListPriceTick(first, tick("AAPL", 100, "2026-09-04T00:00:02Z"));
    expect(second).toBe(first);
    expect(second.get("AAPL")?.seq).toBe(1);
  });

  it("seq는 가격이 움직인 횟수다 — 받은 프레임 수가 아니다", () => {
    let map = applyListPriceTick(new Map(), tick("AAPL", 100, "2026-09-04T00:00:01Z"));
    for (let i = 2; i <= 10; i++) {
      map = applyListPriceTick(map, tick("AAPL", 100, `2026-09-04T00:00:${String(i).padStart(2, "0")}Z`));
    }
    map = applyListPriceTick(map, tick("AAPL", 101, "2026-09-04T00:00:11Z"));
    // 프레임은 11개, 실제 움직임은 두 번(첫 값 + 101).
    expect(map.get("AAPL")?.seq).toBe(2);
    expect(map.get("AAPL")?.prevPrice).toBe(100);
  });

  it("가격이 같은 틱 뒤에도 늦게 온 값은 여전히 막힌다", () => {
    // 가격이 같은 틱을 버리면 맵의 `updatedAt`이 안 올라간다. 그때 그보다 오래된
    // 프레임이 뒤늦게 도착해 값을 되돌리면 안 된다.
    let map = applyListPriceTick(new Map(), tick("AAPL", 100, "2026-09-04T00:00:05Z"));
    map = applyListPriceTick(map, tick("AAPL", 100, "2026-09-04T00:00:09Z"));
    map = applyListPriceTick(map, tick("AAPL", 90, "2026-09-04T00:00:03Z"));
    expect(map.get("AAPL")?.price).toBe(100);
  });

  it("값이 바뀌면 새 맵이다", () => {
    const first = applyListPriceTick(new Map(), tick("AAPL", 100, "2026-09-04T00:00:01Z"));
    const second = applyListPriceTick(first, tick("AAPL", 101, "2026-09-04T00:00:03Z"));
    expect(second).not.toBe(first);
    expect(second.get("AAPL")?.price).toBe(101);
  });

  it("0 이하 가격은 버린다 — 빈 값이 좋은 값을 지우면 안 된다", () => {
    const first = applyListPriceTick(new Map(), tick("AAPL", 100, "2026-09-04T00:00:01Z"));
    const second = applyListPriceTick(first, tick("AAPL", 0, "2026-09-04T00:00:09Z"));
    expect(second.get("AAPL")?.price).toBe(100);
  });

  it("여러 틱을 한 번에 반영한다", () => {
    const map = applyListPriceTicks(new Map(), [
      tick("AAPL", 100, "2026-09-04T00:00:01Z"),
      tick("MSFT", 200, "2026-09-04T00:00:01Z"),
    ]);
    expect(map.size).toBe(2);
  });

  it("전부 무시되는 묶음이면 같은 맵이다", () => {
    const before: ListPriceMap = new Map();
    expect(applyListPriceTicks(before, [tick("", 1, "x"), tick("A", 0, "y")])).toBe(before);
  });
});

describe("reconnectDelayMs", () => {
  const now = Date.parse("2026-09-04T00:00:00Z");

  it("개장까지 남은 시간 + 2초를 준다", () => {
    // 종이 울리는 그 순간에는 서버 달력도 막 넘어가는 중이라 한 박자 늦게 묻는다.
    expect(reconnectDelayMs("2026-09-04T00:10:00Z", now)).toBe(10 * 60_000 + 2_000);
  });

  it("이미 지난 시각이면 최소 1초로 눌린다", () => {
    expect(reconnectDelayMs("2026-09-03T00:00:00Z", now)).toBe(1_000);
  });

  it("32비트를 넘는 대기는 잘라 낸다", () => {
    // setTimeout이 그 범위를 넘으면 **즉시 발화**한다 — 주말을 넘기는 대기가 여기 든다.
    expect(reconnectDelayMs("2027-09-04T00:00:00Z", now)).toBe(0x7fffffff);
  });

  it("값이 없거나 깨졌으면 null — 호출부가 되묻기 주기로 물러선다", () => {
    expect(reconnectDelayMs(undefined, now)).toBeNull();
    expect(reconnectDelayMs("보통날", now)).toBeNull();
  });
});

describe("changeRateFrom", () => {
  it("기준가 대비 등락률", () => {
    expect(changeRateFrom(110, 100)).toBeCloseTo(10);
    expect(changeRateFrom(90, 100)).toBeCloseTo(-10);
  });

  it("기준가를 모르면 null이다 — 0.00%는 「보합」으로 읽힌다", () => {
    expect(changeRateFrom(110, null)).toBeNull();
    expect(changeRateFrom(110, 0)).toBeNull();
    expect(changeRateFrom(110, undefined)).toBeNull();
  });

  it("가격이 없으면 null", () => {
    expect(changeRateFrom(0, 100)).toBeNull();
  });
});
