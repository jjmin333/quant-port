import { SYMBOLS, TRADES } from "@/data";
import { expect, test } from "@/fixtures";
import type { CashBalances, HoldingPosition, HoldingTrade, UserHolding } from "@/types";

/**
 * 보유·매매 회계 (INV-C군) — 이 서비스에서 숫자가 틀리면 가장 아픈 곳.
 *
 * `holding_trades`가 진실이고 `user_holdings`는 그것에서 파생된 캐시다
 * (`service/holding_trades.go` syncHolding). 두 표가 어긋나면 화면은 있지도 않은
 * 자산을 보여준다. 그래서 모든 테스트가 **매매를 넣고 → 파생 결과를 읽어** 둘의
 * 일치를 확인한다.
 */

const S = SYMBOLS.trade;

function buy(over: Partial<HoldingTrade> & { quantity: number; price: number; tradeDate: string }) {
  return { symbol: S, side: "buy", currency: "USD", ...over };
}

test.describe("평단과 수량", () => {
  test("매수 → 매수 → 초과 매도: 평단·실현손익·행 존재가 규칙대로 움직인다", async ({ user }) => {
    // 1) 첫 매수
    const first = await user.api.post<HoldingPosition>(
      "/user/holdings/trades",
      buy(TRADES.firstBuy),
    );
    expect(first).toMatchObject(TRADES.expected.afterFirstBuy);

    // 파생 캐시(user_holdings)도 같은 값을 말해야 한다.
    const holdingsAfterFirst = await user.api.get<UserHolding[]>("/user/holdings");
    expect(holdingsAfterFirst, "매수 직후 보유 목록에 종목이 있어야 한다").toHaveLength(1);
    expect(holdingsAfterFirst[0]).toMatchObject({
      symbol: S,
      quantity: TRADES.expected.afterFirstBuy.quantity,
      avgPrice: TRADES.expected.afterFirstBuy.avgPrice,
      source: "trades",
    });

    // 2) 다른 단가로 두 번째 매수 → 평단은 매수 가중평균
    const second = await user.api.post<HoldingPosition>(
      "/user/holdings/trades",
      buy(TRADES.secondBuy),
    );
    expect(
      second,
      `(10*100 + 10*200)/20 = 150 이어야 한다 — 실제 ${second.avgPrice}`,
    ).toMatchObject(TRADES.expected.afterSecondBuy);

    /*
      3) 보유량(20) 전부를 판다.

      **예전에는 여기서 100주를 팔았다.** 서버가 그것을 받아 20주까지만 깎아
      계산했고, 이 테스트는 그 관용을 계약으로 못박고 있었다. 그런데 그러면 매매
      이력에는 있지도 않았던 「100주 매도」가 남아, 같은 화면의 실현손익(20주분)과
      서로를 부정한다 — 사용자가 증권사와 대조하면 어디에도 없는 80주를 찾게 된다.
      2026-09-06에 **쓰기에서 거부**하기로 정했고, 초과 매도 자체는 아래 전용
      테스트가 본다.

      여기서 검증할 것은 그대로다: 매도가 평단을 건드리지 않고, 실현손익이
      매수 가중평균을 기준으로 나오고, 다 팔면 보유 행이 사라진다는 것.
    */
    const sold = await user.api.post<HoldingPosition>("/user/holdings/trades", {
      ...buy({ quantity: 20, price: 300, tradeDate: TRADES.oversell.tradeDate }),
      side: "sell",
    });
    expect(sold.quantity, "다 팔았으면 0이어야 한다").toBe(0);
    expect(
      sold.realizedPnl,
      `(300-150) * 20주 = 3000 이어야 한다`,
    ).toBe(TRADES.expected.afterOversell.realizedPnl);

    // 4) 다 팔았으면 보유 행은 사라지고, 매매 이력은 남는다.
    expect(
      await user.api.get<UserHolding[]>("/user/holdings"),
      "0주짜리 보유 행이 남으면 '가진 것 목록'에 판 종목이 들어간다",
    ).toHaveLength(0);
    const trades = await user.api.get<HoldingTrade[]>("/user/holdings/trades");
    expect(
      trades,
      "실현손익은 이력에서만 나온다 — 이력이 지워지면 복구할 수 없다",
    ).toHaveLength(TRADES.expected.afterOversell.tradeCount);
  });

  /*
    보유분을 넘는 매도는 **저장되기 전에** 거부된다 (2026-09-06 결정).

    실측으로 드러난 모습: 10주를 사고 50주를 팔면 API가 200을 주고, 이력에는
    「50주 매도」가, 실현손익에는 10주분이 남았다. 아무것도 500이 되지 않으니
    화면만 봐서는 알 수 없고, 이력을 내려받아 손으로 더해 봐야 보인다.
  */
  test("보유분을 넘는 매도는 거부되고 이력에 남지 않는다", async ({ user }) => {
    await user.api.post("/user/holdings/trades", buy({ quantity: 10, price: 100, tradeDate: "2026-01-05" }));

    const rejected = await user.api.send("POST", "/user/holdings/trades", {
      ...buy({ quantity: 50, price: 200, tradeDate: "2026-01-06" }),
      side: "sell",
    });
    expect(rejected.status(), "보유분을 넘는 매도가 받아들여지면 안 된다").toBe(400);

    // 상태코드와 무관한 진짜 불변식 — 거부됐으면 아무것도 안 바뀐다.
    const trades = await user.api.get<HoldingTrade[]>("/user/holdings/trades");
    expect(trades, "거부된 매도가 이력에 남으면 안 된다").toHaveLength(1);
    expect(trades[0]!.side).toBe("buy");

    const holdings = await user.api.get<UserHolding[]>("/user/holdings");
    expect(holdings[0]!.quantity, "거부된 매도가 보유 수량을 건드리면 안 된다").toBe(10);
  });

  test("보유가 없는 종목의 매도는 거부된다", async ({ user }) => {
    const rejected = await user.api.send("POST", "/user/holdings/trades", {
      ...buy({ quantity: 1, price: 100, tradeDate: "2026-01-05" }),
      side: "sell",
    });
    expect(rejected.status(), "가진 적 없는 종목을 파는 기록이 남으면 안 된다").toBe(400);
    expect(await user.api.get<HoldingTrade[]>("/user/holdings/trades")).toHaveLength(0);
  });

  test("정상 매도는 그대로 통과하고, 같은 매도를 다시 보내도 초과로 오해받지 않는다", async ({
    user,
  }) => {
    await user.api.post("/user/holdings/trades", buy({ quantity: 10, price: 100, tradeDate: "2026-01-05" }));

    const sell = { ...buy({ quantity: 10, price: 200, tradeDate: "2026-01-06" }), side: "sell" };
    const first = await user.api.post<HoldingPosition>("/user/holdings/trades", sell);
    expect(first.quantity).toBe(0);

    /*
      같은 매도를 두 번 보내는 일은 실제로 일어난다(응답이 느릴 때). 중복 검사가
      먼저 걸러 주지 않으면 초과 검사가 같은 매도를 두 번 세어, 멀쩡한 재전송이
      「보유분 초과」로 거부된다.
    */
    const again = await user.api.send("POST", "/user/holdings/trades", sell);
    expect(again.ok(), "같은 매도 재전송이 초과로 거부되면 안 된다").toBe(true);
    expect(
      await user.api.get<HoldingTrade[]>("/user/holdings/trades"),
      "재전송이 이력을 늘리면 안 된다",
    ).toHaveLength(2);
  });

  test("매도는 평단을 바꾸지 않는다", async ({ user }) => {
    await user.api.post("/user/holdings/trades", buy(TRADES.firstBuy));
    await user.api.post("/user/holdings/trades", buy(TRADES.secondBuy));

    const afterPartialSell = await user.api.post<HoldingPosition>("/user/holdings/trades", {
      ...buy({ quantity: 5, price: 999, tradeDate: "2026-01-08" }),
      side: "sell",
    });

    expect(afterPartialSell.quantity, "20 - 5 = 15").toBe(15);
    expect(
      afterPartialSell.avgPrice,
      "999에 팔았다고 평단이 따라 움직이면 손익이 매도가에 오염된다",
    ).toBe(TRADES.expected.afterSecondBuy.avgPrice);
  });
});

test.describe("중복 요청", () => {
  test("같은 매매를 두 번 보내도 이력은 한 줄만 남는다", async ({ user }) => {
    const payload = buy(TRADES.firstBuy);

    const firstResponse = await user.api.post<HoldingPosition>("/user/holdings/trades", payload);
    const secondResponse = await user.api.post<HoldingPosition>("/user/holdings/trades", payload);

    expect(
      secondResponse,
      "재시도가 수량을 두 배로 만들면 네트워크 한 번 끊길 때마다 자산이 늘어난다",
    ).toEqual(firstResponse);

    const trades = await user.api.get<HoldingTrade[]>("/user/holdings/trades");
    expect(trades).toHaveLength(1);

    const holdings = await user.api.get<UserHolding[]>("/user/holdings");
    expect(holdings[0]!.quantity).toBe(TRADES.firstBuy.quantity);
  });

  test("일괄 등록을 두 번 해도 종목당 한 줄만 유지된다", async ({ user }) => {
    const items = [{ symbol: S, quantity: 7, price: 150, currency: "USD", tradeDate: "2026-03-03" }];

    await user.api.post("/user/holdings/trades/bulk-register", { items });
    await user.api.post("/user/holdings/trades/bulk-register", { items });

    const trades = await user.api.get<HoldingTrade[]>("/user/holdings/trades");
    expect(trades, "일괄 등록 행은 갱신되어야지 쌓이면 안 된다").toHaveLength(1);

    const holdings = await user.api.get<UserHolding[]>("/user/holdings");
    expect(holdings[0]!.quantity, "두 번 등록해서 14주가 되면 안 된다").toBe(7);
  });

  /*
   * 일괄 등록은 **전부 저장되거나 아무것도 저장되지 않아야 한다.**
   *
   * 고쳐진 결함(2026-08-24, docs/PERFORMANCE_AUDIT.md P1-8): 예전에는 항목을
   * 하나씩 검증하며 그때그때 썼고 트랜잭션이 없었다. 그래서 목록 중간에 잘못된
   * 항목이 있으면 **그 앞의 종목만 등록된 상태로 남았다.** 스크린샷 임포트가
   * 한 번에 수십 종목을 넣는 경로라, 사용자는 일부만 들어온 포트폴리오를 보고도
   * 그것이 정상인지 알 수 없었다.
   */
  test("일괄 등록 중 잘못된 항목이 있으면 아무것도 저장되지 않는다", async ({ user }) => {
    const res = await user.api.send("POST", "/user/holdings/trades/bulk-register", {
      items: [
        { symbol: S, quantity: 5, price: 100, currency: "USD", tradeDate: "2026-03-03" },
        { symbol: SYMBOLS.watch, quantity: 3, price: 200, currency: "USD", tradeDate: "03-03-2026" },
      ],
    });
    expect(res.ok(), "잘못된 날짜가 섞였으면 요청은 실패해야 한다").toBe(false);

    expect(
      await user.api.get<HoldingTrade[]>("/user/holdings/trades"),
      "앞쪽 항목만 저장되면 사용자가 '일부만 들어온' 포트폴리오를 보게 된다",
    ).toHaveLength(0);
    expect(await user.api.get<UserHolding[]>("/user/holdings")).toHaveLength(0);
  });

  test("일괄 등록은 여러 종목을 한 번에 모두 저장한다", async ({ user }) => {
    await user.api.post("/user/holdings/trades/bulk-register", {
      items: [
        { symbol: S, quantity: 5, price: 100, currency: "USD", tradeDate: "2026-03-03" },
        { symbol: SYMBOLS.watch, quantity: 3, price: 200, currency: "USD", tradeDate: "2026-03-04" },
      ],
    });

    const trades = await user.api.get<HoldingTrade[]>("/user/holdings/trades");
    expect(trades, "두 종목 모두 이력이 남아야 한다").toHaveLength(2);

    const holdings = await user.api.get<UserHolding[]>("/user/holdings");
    expect(holdings, "두 종목 모두 파생 보유가 생겨야 한다").toHaveLength(2);
    const bySymbol = new Map(holdings.map((h) => [h.symbol, h]));
    expect(bySymbol.get(S)).toMatchObject({ quantity: 5, avgPrice: 100 });
    expect(bySymbol.get(SYMBOLS.watch)).toMatchObject({ quantity: 3, avgPrice: 200 });
  });

  test("한 요청 안에 같은 종목이 두 번 있으면 마지막 값 한 줄만 남는다", async ({ user }) => {
    /*
     * 배치 재작성(P1-8)에서 동작이 바뀌기 쉬운 지점이다. 예전에는 항목마다 DB를
     * 다시 조회했으므로 두 번째 항목이 방금 넣은 행을 발견해 갱신했다. 지금은
     * 트랜잭션 시작 시점에 한 번만 읽으므로, 방금 삽입한 행을 **기억해 두지
     * 않으면** 같은 종목이 두 줄이 된다.
     */
    await user.api.post("/user/holdings/trades/bulk-register", {
      items: [
        { symbol: S, quantity: 5, price: 100, currency: "USD", tradeDate: "2026-03-03" },
        { symbol: S, quantity: 9, price: 300, currency: "USD", tradeDate: "2026-03-03" },
      ],
    });

    const trades = await user.api.get<HoldingTrade[]>("/user/holdings/trades");
    expect(trades, "같은 종목이 두 줄로 들어가면 안 된다").toHaveLength(1);

    const holdings = await user.api.get<UserHolding[]>("/user/holdings");
    expect(holdings[0], "마지막 항목의 값이 남아야 한다").toMatchObject({
      quantity: 9,
      avgPrice: 300,
    });
  });

  test("일괄 등록을 다른 수량으로 다시 하면 덮어쓴다", async ({ user }) => {
    await user.api.post("/user/holdings/trades/bulk-register", {
      items: [{ symbol: S, quantity: 7, price: 150, currency: "USD", tradeDate: "2026-03-03" }],
    });
    await user.api.post("/user/holdings/trades/bulk-register", {
      items: [{ symbol: S, quantity: 9, price: 150, currency: "USD", tradeDate: "2026-03-03" }],
    });

    expect(await user.api.get<HoldingTrade[]>("/user/holdings/trades")).toHaveLength(1);
    const holdings = await user.api.get<UserHolding[]>("/user/holdings");
    expect(holdings[0]!.quantity).toBe(9);
  });
});

test.describe("입력 검증", () => {
  const invalid: Array<[name: string, body: Record<string, unknown>, expectInMessage: string]> = [
    ["수량 0", buy({ quantity: 0, price: 100, tradeDate: "2026-01-05" }), "quantity"],
    ["수량 음수", buy({ quantity: -1, price: 100, tradeDate: "2026-01-05" }), "quantity"],
    ["단가 음수", buy({ quantity: 1, price: -1, tradeDate: "2026-01-05" }), "price"],
    [
      "날짜 형식",
      buy({ quantity: 1, price: 100, tradeDate: "05-01-2026" }),
      "YYYY-MM-DD",
    ],
    ["심볼 없음", { side: "buy", quantity: 1, price: 100, currency: "USD" }, "symbol"],
  ];

  for (const [name, body, expectInMessage] of invalid) {
    test(`${name} 은 400이고 아무것도 저장되지 않는다`, async ({ user }) => {
      const message = await user.api.expectFailure("POST", "/user/holdings/trades", 400, body);
      expect(message).toContain(expectInMessage);

      expect(
        await user.api.get<HoldingTrade[]>("/user/holdings/trades"),
        "거부된 요청이 이력을 남기면 안 된다",
      ).toHaveLength(0);
      expect(await user.api.get<UserHolding[]>("/user/holdings")).toHaveLength(0);
    });
  }
});

test.describe("삭제", () => {
  test("매매를 지우면 파생 보유 수량이 즉시 다시 계산된다", async ({ user }) => {
    await user.api.post("/user/holdings/trades", buy(TRADES.firstBuy));
    await user.api.post("/user/holdings/trades", buy(TRADES.secondBuy));

    const trades = await user.api.get<HoldingTrade[]>("/user/holdings/trades");
    const secondTrade = trades.find((t) => t.price === TRADES.secondBuy.price);
    expect(secondTrade, "두 번째 매수가 조회돼야 한다").toBeDefined();

    const position = await user.api.del<HoldingPosition>(
      `/user/holdings/trades/${secondTrade!.id}?symbol=${S}`,
    );

    expect(position.quantity, "남은 것은 첫 매수 10주뿐").toBe(TRADES.firstBuy.quantity);
    expect(position.avgPrice, "평단도 첫 매수 단가로 되돌아가야 한다").toBe(TRADES.firstBuy.price);

    const holdings = await user.api.get<UserHolding[]>("/user/holdings");
    expect(holdings[0]!.quantity, "캐시가 삭제 전 값을 들고 있으면 안 된다").toBe(
      TRADES.firstBuy.quantity,
    );
  });

  test("종목 삭제는 보유와 매매 이력을 함께 지운다", async ({ user }) => {
    await user.api.post("/user/holdings/trades", buy(TRADES.firstBuy));

    await user.api.del(`/user/holdings/${S}`);

    expect(await user.api.get<UserHolding[]>("/user/holdings")).toHaveLength(0);
    expect(
      await user.api.get<HoldingTrade[]>("/user/holdings/trades"),
      "종목을 지웠는데 이력이 남으면 다음 동기화에서 보유가 되살아난다",
    ).toHaveLength(0);
  });
});

test.describe("현금 잔고", () => {
  test("음수 잔고는 저장되지 않고 기존 값이 유지된다", async ({ user }) => {
    await user.api.put("/user/cash-balances", { krw: 1_000_000, usd: 500 });

    // 현재 이 경로의 검증 실패는 매핑되지 않아 502로 나온다
    // (`handler/toss_error.go`의 기본 분기). 상태코드를 못 박는 대신 **결과**를
    // 검증한다 — 상태코드가 400으로 고쳐져도 이 테스트는 그대로 통과해야 한다.
    const rejected = await user.api.send("PUT", "/user/cash-balances", { krw: -1, usd: 0 });
    expect(rejected.ok(), "음수 잔고가 받아들여지면 안 된다").toBe(false);

    const after = await user.api.get<CashBalances>("/user/cash-balances");
    expect(after, "거부된 요청이 기존 잔고를 덮어쓰면 안 된다").toEqual({
      krw: 1_000_000,
      usd: 500,
    });
  });
});
