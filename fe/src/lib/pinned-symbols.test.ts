import { afterEach, describe, expect, it, vi } from "vitest";
import {
  __testing,
  isPinned,
  lastViewedSymbol,
  pinSymbol,
  pinnedSymbolsFor,
  subscribePinned,
} from "./pinned-symbols";

afterEach(() => {
  __testing.reset();
});

describe("pinSymbol", () => {
  it("시장별로 갈라 돌려준다 — 허브가 시장마다 따로 연결하기 때문이다", () => {
    pinSymbol("005930", "domestic");
    pinSymbol("AAPL", "us");
    expect(pinnedSymbolsFor("domestic")).toEqual(["005930"]);
    expect(pinnedSymbolsFor("us")).toEqual(["AAPL"]);
    expect(pinnedSymbolsFor("crypto")).toEqual([]);
  });

  it("대소문자와 공백을 정규화한다 — 허브의 합집합이 대문자로 서기 때문이다", () => {
    pinSymbol("  aapl ", "us");
    expect(pinnedSymbolsFor("us")).toEqual(["AAPL"]);
    expect(isPinned("aapl")).toBe(true);
  });

  it("빈 심볼은 등록하지 않는다 — 종목을 아직 못 정한 화면이 빈 값으로 부른다", () => {
    pinSymbol("", "domestic");
    pinSymbol("   ", "domestic");
    expect(__testing.count()).toBe(0);
    expect(isPinned("")).toBe(false);
    expect(lastViewedSymbol()).toBeNull();
  });

  /*
    상세 화면과 모달이 같은 종목을 동시에 열어 둘 수 있다. 한쪽이 닫혔다고 다른 쪽의
    핀까지 빠지면, 남아 있는 화면이 숨은 탭에서 시세를 잃는다.
  */
  it("같은 종목을 둘이 열어 두면 하나가 닫혀도 열려 있는 것으로 센다", () => {
    const offA = pinSymbol("005930", "domestic");
    pinSymbol("005930", "domestic");
    offA();
    expect(isPinned("005930")).toBe(true);
  });
});

/*
「열자마자 뜨고 × 까지 유지」 (2026-09-15 사용자 결정). 탭 제목이 읽는 값이라,
모달을 닫는다고 사라지면 안 된다 — 사라지면 다른 메뉴로 옮긴 순간 제목이 돌아간다.
*/
describe("마지막으로 본 종목", () => {
  it("화면이 닫혀도 남는다 — active 에서만 빠진다", () => {
    const off = pinSymbol("005930", "domestic");
    off();

    expect(isPinned("005930")).toBe(false); // 더 이상 열려 있지는 않다
    expect(lastViewedSymbol()).toEqual({ symbol: "005930", assetType: "domestic" });
    // 숨은 탭이 살려 둘 목록에는 그대로 남는다 — 탭 제목이 계속 값을 받아야 한다.
    expect(pinnedSymbolsFor("domestic")).toEqual(["005930"]);
  });

  it("새 종목을 열면 그 자리를 갈아끼운다 — 한 번에 하나다", () => {
    pinSymbol("005930", "domestic")();
    pinSymbol("000660", "domestic")();

    expect(lastViewedSymbol()?.symbol).toBe("000660");
    expect(pinnedSymbolsFor("domestic")).toEqual(["000660"]);
  });

  it("시장이 다른 종목으로 옮기면 앞 시장에는 아무것도 안 남는다", () => {
    pinSymbol("005930", "domestic")();
    pinSymbol("AAPL", "us")();

    expect(pinnedSymbolsFor("domestic")).toEqual([]);
    expect(pinnedSymbolsFor("us")).toEqual(["AAPL"]);
  });

  it("열려 있는 종목과 마지막으로 본 종목이 다르면 둘 다 남는다", () => {
    pinSymbol("005930", "domestic")(); // 닫았다 — lastViewed 로만 남는다
    pinSymbol("000660", "domestic"); // 지금 열려 있다

    expect(pinnedSymbolsFor("domestic").sort()).toEqual(["000660"]);
    expect(lastViewedSymbol()?.symbol).toBe("000660");
  });
});

describe("subscribePinned", () => {
  it("등록과 해제를 알린다 — 숨은 탭이 구독을 다시 맞추는 신호다", () => {
    const seen = vi.fn();
    const off = subscribePinned(seen);

    const unpin = pinSymbol("005930", "domestic");
    expect(seen).toHaveBeenCalledTimes(1);
    unpin();
    expect(seen).toHaveBeenCalledTimes(2);

    off();
    pinSymbol("000660", "domestic");
    expect(seen).toHaveBeenCalledTimes(2);
  });

  it("빈 심볼은 알림도 만들지 않는다", () => {
    const seen = vi.fn();
    subscribePinned(seen);
    pinSymbol("", "domestic");
    expect(seen).not.toHaveBeenCalled();
  });
});
