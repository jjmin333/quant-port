import { afterEach, describe, expect, it } from "vitest";
import {
  __testing,
  getHubState,
  installStreamPort,
  setVisible,
  subscribeListPrices,
  unionOf,
  type StreamHandlers,
  type StreamPort,
} from "./list-price-hub";
import { pinSymbol, __testing as pinTesting } from "./pinned-symbols";

/*
가짜 이음매. 진짜 EventSource 대신 「열린 연결」을 목록으로 들고 있어서, 시험이
「연결이 몇 개 열렸나 · 어떤 심볼로 열렸나」를 그대로 물을 수 있다.

타이머도 가짜다 — vi.useFakeTimers 대신 직접 들고 있는 이유는 허브가 `setTimer`를
이음매로 받기 때문이고, 그러면 「지금 걸려 있는 타이머」를 시험이 골라 터뜨릴 수 있다.
*/
type Opened = {
  symbols: string[];
  assetType: string;
  handlers: StreamHandlers;
  closed: boolean;
};

function makePort() {
  const opened: Opened[] = [];
  const timers = new Map<number, { due: number; fn: () => void }>();
  let nextId = 1;
  let clock = 0;

  /** 시각이 된 타이머만 터뜨린다 — 그 안에서 새로 걸린 것도 같은 시각이면 함께. */
  const runDue = () => {
    for (let round = 0; round < 10; round++) {
      const due = [...timers.entries()].filter(([, t]) => t.due <= clock);
      if (due.length === 0) return;
      for (const [id, t] of due) {
        timers.delete(id);
        t.fn();
      }
    }
  };

  const port: StreamPort = {
    now: () => clock,
    setTimer: (fn, ms) => {
      const id = nextId++;
      timers.set(id, { due: clock + ms, fn });
      return id;
    },
    clearTimer: (id) => {
      timers.delete(id);
    },
    open: (symbols, assetType, handlers) => {
      const entry: Opened = { symbols, assetType, handlers, closed: false };
      opened.push(entry);
      return {
        close: () => {
          entry.closed = true;
        },
        isClosed: () => entry.closed,
      };
    },
  };

  return {
    port,
    opened,
    live: () => opened.filter((o) => !o.closed),
    /** `ms`만큼 시간을 보내고, 그 사이에 시각이 된 타이머를 터뜨린다. */
    tick: (ms: number) => {
      clock += ms;
      runDue();
    },
  };
}

/** 재연결 디바운스(60ms)를 넘기는 최소 시간. */
const PAST_DEBOUNCE_MS = 100;

afterEach(() => {
  __testing.reset();
  pinTesting.reset();
});

describe("unionOf", () => {
  it("구독자들의 심볼을 대문자로 합친다", () => {
    const union = unionOf([
      { symbols: ["000660", "005930"], notify: () => {} },
      { symbols: [" aapl ", "005930"], notify: () => {} },
    ]);
    expect(union).toEqual(["000660", "005930", "AAPL"]);
  });
});


describe("subscribeListPrices", () => {
  it("겹치는 구독 셋이 연결 하나가 된다 — 실측한 홈 화면 그대로", () => {
    const p = makePort();
    installStreamPort(p.port);

    subscribeListPrices("domestic", ["000660", "005380", "005930"], () => {});
    subscribeListPrices("domestic", ["000950", "004960", "053950"], () => {});
    subscribeListPrices("domestic", ["000660", "005930", "009150"], () => {});
    subscribeListPrices("us", ["ADBE", "CPRT", "FDX"], () => {});

    // 디바운스가 지나기 전에는 아무것도 안 연다.
    expect(p.opened).toHaveLength(0);
    p.tick(PAST_DEBOUNCE_MS);

    expect(p.live()).toHaveLength(2);
    expect(p.live().map((o) => o.assetType).sort()).toEqual(["domestic", "us"]);
    const kr = p.live().find((o) => o.assetType === "domestic")!;
    expect(kr.symbols.sort()).toEqual([
      "000660",
      "000950",
      "004960",
      "005380",
      "005930",
      "009150",
      "053950",
    ]);
  });

  it("같은 합집합이면 다시 열지 않는다", () => {
    const p = makePort();
    installStreamPort(p.port);
    subscribeListPrices("domestic", ["005930"], () => {});
    p.tick(PAST_DEBOUNCE_MS);
    expect(p.opened).toHaveLength(1);

    // 이미 구독 중인 심볼만 더하는 구독자 — 합집합이 그대로다.
    subscribeListPrices("domestic", ["005930"], () => {});
    p.tick(PAST_DEBOUNCE_MS);
    expect(p.opened).toHaveLength(1);
  });

  it("합집합이 늘면 새 심볼로 다시 연다", () => {
    const p = makePort();
    installStreamPort(p.port);
    subscribeListPrices("domestic", ["005930"], () => {});
    p.tick(PAST_DEBOUNCE_MS);
    subscribeListPrices("domestic", ["000660"], () => {});
    p.tick(PAST_DEBOUNCE_MS);

    expect(p.opened).toHaveLength(2);
    expect(p.opened[0]!.closed).toBe(true);
    expect(p.live()[0]!.symbols.sort()).toEqual(["000660", "005930"]);
  });

  it("마지막 구독자가 떠나면 연결을 닫고 가격도 버린다", () => {
    const p = makePort();
    installStreamPort(p.port);
    const off = subscribeListPrices("domestic", ["005930"], () => {});
    p.tick(PAST_DEBOUNCE_MS);
    p.live()[0]!.handlers.onSnapshot([
      { symbol: "005930", price: 100, updatedAt: "2026-09-07T00:00:00Z" },
    ]);
    expect(getHubState("domestic").prices.size).toBe(1);

    off();
    p.tick(PAST_DEBOUNCE_MS);
    expect(p.live()).toHaveLength(0);
    expect(getHubState("domestic").prices.size).toBe(0);
  });

  it("가격이 오면 구독자에게 알리고 맵에 쌓는다", () => {
    const p = makePort();
    installStreamPort(p.port);
    let notified = 0;
    subscribeListPrices("domestic", ["005930"], () => {
      notified += 1;
    });
    p.tick(PAST_DEBOUNCE_MS);

    p.live()[0]!.handlers.onPrice({
      symbol: "005930",
      price: 71_000,
      updatedAt: "2026-09-07T00:00:00Z",
    });
    expect(notified).toBeGreaterThan(0);
    expect(getHubState("domestic").prices.get("005930")?.price).toBe(71_000);
    expect(getHubState("domestic").connected).toBe(true);
  });

  it("마감 통보를 받으면 끊고, 다음 개장에 다시 연다", () => {
    const p = makePort();
    installStreamPort(p.port);
    subscribeListPrices("domestic", ["005930"], () => {});
    p.tick(PAST_DEBOUNCE_MS);

    p.live()[0]!.handlers.onSession(new Date(p.port.now() + 1_000).toISOString());
    expect(p.live()).toHaveLength(0);
    expect(getHubState("domestic").connected).toBe(false);

    p.tick(4_000);
    expect(p.live()).toHaveLength(1);
  });

  it("마감 중에는 카드가 새로 떠도 다시 열지 않는다 — 실측에서 가장 잦던 낭비", () => {
    const p = makePort();
    installStreamPort(p.port);
    subscribeListPrices("domestic", ["005930"], () => {});
    p.tick(PAST_DEBOUNCE_MS);
    // 월요일 08:00에 열린다는 통보.
    p.live()[0]!.handlers.onSession(new Date(p.port.now() + 2 * 24 * 3_600_000).toISOString());
    expect(p.live()).toHaveLength(0);
    const opensSoFar = p.opened.length;

    // 카드 셋이 뒤늦게 뜨면서 각자 심볼을 등록한다.
    subscribeListPrices("domestic", ["000660"], () => {});
    subscribeListPrices("domestic", ["035420"], () => {});
    subscribeListPrices("domestic", ["005380"], () => {});
    p.tick(5_000);
    p.tick(PAST_DEBOUNCE_MS);

    expect(p.opened).toHaveLength(opensSoFar);
  });

  it("개장 시각이 되면 그동안 모인 심볼 전부로 한 번에 연다", () => {
    const p = makePort();
    installStreamPort(p.port);
    subscribeListPrices("domestic", ["005930"], () => {});
    p.tick(PAST_DEBOUNCE_MS);
    p.live()[0]!.handlers.onSession(new Date(p.port.now() + 10_000).toISOString());

    subscribeListPrices("domestic", ["000660"], () => {});
    p.tick(20_000);
    p.tick(PAST_DEBOUNCE_MS);

    expect(p.live()).toHaveLength(1);
    expect(p.live()[0]!.symbols.sort()).toEqual(["000660", "005930"]);
  });
});

/*
탭이 뒤로 갔을 때 (2026-09-15 사용자 요청 3·4).

09-05에는 유예 3시간에 「지나면 통째로 닫는다」였다. 지금은 30분에 「지나면 **열어 둔
종목만 남긴다**」이다. 아래 시험들은 그 두 가지 — **언제** 좁히나, **무엇이** 남나 —
를 각각 센다.
*/
const GRACE_MS = 30 * 60 * 1000;

describe("안 보이는 탭", () => {
  it("숨는 것만으로는 끊지 않는다 — 창을 잠깐 옮겼다 오는 일이 여기 있다", () => {
    const p = makePort();
    installStreamPort(p.port, GRACE_MS);
    subscribeListPrices("domestic", ["005930"], () => {});
    p.tick(PAST_DEBOUNCE_MS);

    setVisible(false);
    expect(p.live()).toHaveLength(1);
  });

  it("유예가 지나면 놓는다 — 열어 둔 종목이 없을 때", () => {
    const p = makePort();
    installStreamPort(p.port, GRACE_MS);
    subscribeListPrices("domestic", ["005930", "000660"], () => {});
    p.tick(PAST_DEBOUNCE_MS);

    setVisible(false);
    p.tick(GRACE_MS);
    p.tick(PAST_DEBOUNCE_MS);
    expect(p.live()).toHaveLength(0);
  });

  it("유예 안에 돌아오면 살아 있는 연결을 다시 열지 않는다", () => {
    const p = makePort();
    installStreamPort(p.port, GRACE_MS);
    subscribeListPrices("domestic", ["005930"], () => {});
    p.tick(PAST_DEBOUNCE_MS);

    setVisible(false);
    p.tick(60_000);
    setVisible(true);
    p.tick(PAST_DEBOUNCE_MS);
    expect(p.opened).toHaveLength(1);
  });

  it("유예가 지나 놓았더라도 돌아오면 다시 붙는다", () => {
    const p = makePort();
    installStreamPort(p.port, GRACE_MS);
    subscribeListPrices("domestic", ["005930"], () => {});
    p.tick(PAST_DEBOUNCE_MS);

    setVisible(false);
    p.tick(2 * GRACE_MS);
    p.tick(PAST_DEBOUNCE_MS);
    expect(p.live()).toHaveLength(0);

    setVisible(true);
    p.tick(PAST_DEBOUNCE_MS);
    expect(p.live()).toHaveLength(1);
  });
});

describe("열어 둔 종목은 숨어도 남는다 (사용자 요청 4)", () => {
  it("서른 종목이 아니라 열어 둔 하나가 남는다", () => {
    const p = makePort();
    installStreamPort(p.port, GRACE_MS);
    subscribeListPrices("domestic", ["005930", "000660", "035420"], () => {});
    p.tick(PAST_DEBOUNCE_MS);
    pinSymbol("000660", "domestic");

    setVisible(false);
    p.tick(GRACE_MS);
    p.tick(PAST_DEBOUNCE_MS);

    expect(p.live()).toHaveLength(1);
    expect(p.live()[0]!.symbols).toEqual(["000660"]);
  });

  it("다시 보이면 목록 전체로 돌아온다 — 살아 있다고 건너뛰지 않는다", () => {
    const p = makePort();
    installStreamPort(p.port, GRACE_MS);
    subscribeListPrices("domestic", ["005930", "000660"], () => {});
    p.tick(PAST_DEBOUNCE_MS);
    pinSymbol("000660", "domestic");

    setVisible(false);
    p.tick(GRACE_MS);
    p.tick(PAST_DEBOUNCE_MS);
    expect(p.live()[0]!.symbols).toEqual(["000660"]);

    setVisible(true);
    p.tick(PAST_DEBOUNCE_MS);
    expect(p.live()).toHaveLength(1);
    expect(p.live()[0]!.symbols.sort()).toEqual(["000660", "005930"]);
  });

  it("핀이 다른 시장이면 이 시장에는 아무것도 안 남는다", () => {
    const p = makePort();
    installStreamPort(p.port, GRACE_MS);
    subscribeListPrices("domestic", ["005930"], () => {});
    p.tick(PAST_DEBOUNCE_MS);
    pinSymbol("AAPL", "us");

    setVisible(false);
    p.tick(GRACE_MS);
    p.tick(PAST_DEBOUNCE_MS);
    expect(p.live()).toHaveLength(0);
  });

  it("아무도 구독하지 않는 종목을 핀이 **새로 열지는 않는다**", () => {
    const p = makePort();
    installStreamPort(p.port, GRACE_MS);
    subscribeListPrices("domestic", ["005930"], () => {});
    p.tick(PAST_DEBOUNCE_MS);
    // 종목 상세 화면이 건 핀 — 그 화면은 자기 체결 스트림을 따로 연다.
    pinSymbol("068270", "domestic");

    setVisible(false);
    p.tick(GRACE_MS);
    p.tick(PAST_DEBOUNCE_MS);
    expect(p.live()).toHaveLength(0);
  });

  /*
    창을 닫아도 **마지막으로 본 종목은 남는다** (2026-09-15 사용자 결정: 「× 까지
    유지」). 브라우저 탭 제목이 그 값을 읽으므로, 모달을 닫자마자 구독이 끊기면
    다른 메뉴로 옮긴 순간 제목이 멈춘 값을 들고 있게 된다.
  */
  it("숨어 있는 동안 창을 닫아도 마지막으로 본 종목은 남는다", () => {
    const p = makePort();
    installStreamPort(p.port, GRACE_MS);
    subscribeListPrices("domestic", ["005930", "000660"], () => {});
    p.tick(PAST_DEBOUNCE_MS);
    const unpin = pinSymbol("000660", "domestic");

    setVisible(false);
    p.tick(GRACE_MS);
    p.tick(PAST_DEBOUNCE_MS);
    expect(p.live()[0]!.symbols).toEqual(["000660"]);

    unpin();
    p.tick(PAST_DEBOUNCE_MS);
    expect(p.live()).toHaveLength(1);
    expect(p.live()[0]!.symbols).toEqual(["000660"]);
  });

  it("종목을 한 번도 안 열었으면 숨은 탭은 통째로 놓는다", () => {
    const p = makePort();
    installStreamPort(p.port, GRACE_MS);
    subscribeListPrices("domestic", ["005930", "000660"], () => {});
    p.tick(PAST_DEBOUNCE_MS);

    setVisible(false);
    p.tick(GRACE_MS);
    p.tick(PAST_DEBOUNCE_MS);
    expect(p.live()).toHaveLength(0);
  });
});
