import { SYMBOLS } from "@/data";
import { expect, test } from "@/fixtures";
import type { TestUser } from "@/test-user";
import type { HoldingTrade, UserHolding } from "@/types";
import { holdingItem } from "@/ui";

/**
 * 화면에서 보유 종목 추가 (브라우저 필수).
 *
 * 왜 브라우저인가: 이 폼은 **종목 검색 자동완성으로 심볼을 확정**해야만 수량·단가
 * 칸이 열리고 저장 버튼이 활성화된다. 사용자가 실제로 하는 일은 "NVDA를
 * 입력했다"가 아니라 "검색 결과에서 엔비디아를 골랐다"이고, 그 사이에 디바운스와
 * 통화 결정(USD/KRW), 입력 잠금 해제가 끼어 있다. API로는 이 경로가 없다.
 *
 * 저장 결과는 **화면 말고 API로** 확인한다 — "추가했습니다" 문구는 저장의 증거가
 * 아니다.
 */

const S = SYMBOLS.trade;
const QUANTITY = "12";
const PRICE = "150";

/**
 * 종목 추가 카드는 **사이드 레일 안에 있고, 레일은 보유가 0건이면 렌더되지 않는다**
 * (`holdings-page-client.tsx`의 `isEmpty` 분기가 안내 카드로 갈아끼운다).
 * 그래서 화면에서 추가하는 시나리오는 "이미 뭔가 갖고 있는 사람"이 전제다 —
 * 그 전제를 API로 만들고 시작한다.
 */
async function seedExistingHolding(user: TestUser): Promise<void> {
  await user.api.post("/user/holdings/trades", {
    symbol: SYMBOLS.watch,
    side: "buy",
    quantity: 1,
    price: 400,
    currency: "USD",
    tradeDate: "2026-02-02",
  });
}

/**
 * 추가 카드가 조작 가능한 상태가 될 때까지 기다린다.
 *
 * 여기서는 React 내부를 들여다보는 하이드레이션 탐지가 필요 없다. 이 카드는
 * **클라이언트에서만** 그려지므로(보유 목록을 받아 `isEmpty`를 판정한 뒤 레일이
 * 붙는다), 검색칸이 보인다는 사실 자체가 "React가 붙었고 데이터도 왔다"는 뜻이다.
 * 로그인 폼과 다른 점은 그쪽은 SSR 마크업이 먼저 눌리는 게 문제였다는 것이다.
 */
async function expectAddCardReady(page: import("@playwright/test").Page): Promise<void> {
  await expect(
    page.getByPlaceholder("종목명·코드 검색"),
    "종목 추가 카드가 나타나지 않았다 — 사이드 레일이 렌더되지 않은 것",
  ).toBeVisible();
}

async function pickSymbolInSearch(page: import("@playwright/test").Page): Promise<void> {
  await page.getByPlaceholder("종목명·코드 검색").fill(S);
  const option = page
    .getByRole("listbox", { name: "종목 검색 결과" })
    .getByRole("option")
    .filter({ hasText: S });
  await expect(option.first(), `${S} 가 검색 결과에 없다`).toBeVisible();
  await option.first().click();
}

test.describe("종목 직접 추가", () => {
  test("검색으로 고른 종목이 매매 이력 · 파생 보유 · 표에 같은 값으로 도착한다", async ({
    userPage,
    user,
  }) => {
    await seedExistingHolding(user);
    await userPage.goto("/holdings");
    await expectAddCardReady(userPage);

    await pickSymbolInSearch(userPage);

    await userPage.getByLabel("수량").fill(QUANTITY);
    await userPage.getByLabel(/매수 단가/).fill(PRICE);

    /*
     * 매수일은 접근 가능한 이름이 없다 — `trade-date-field.tsx`의 라벨이 <span>이라
     * input과 연결돼 있지 않다. 날짜를 테스트가 따로 계산하면 자정 근처에서 화면과
     * 어긋나므로, **화면이 들고 있는 값을 읽어** 그대로 기대값으로 쓴다. 덤으로
     * "화면에 보인 날짜로 저장된다"까지 검증된다.
     */
    const dateInput = userPage.locator('input[type="date"]').first();
    const shownTradeDate = await dateInput.inputValue();
    expect(shownTradeDate, "매수일 기본값이 비어 있다").toMatch(/^\d{4}-\d{2}-\d{2}$/);

    await userPage.getByRole("button", { name: "보유에 추가" }).click();
    await expect(userPage.getByText(new RegExp(`${QUANTITY}주를 추가했습니다`))).toBeVisible();

    // 진짜 검증 — 저장된 매매 이력이 폼과 정확히 일치한다.
    const trades = await user.api.get<HoldingTrade[]>(`/user/holdings/trades?symbol=${S}`);
    expect(trades, "화면은 성공이라 했는데 이력이 한 줄이 아니다").toHaveLength(1);
    expect(trades[0]).toMatchObject({
      symbol: S,
      side: "buy",
      quantity: Number(QUANTITY),
      price: Number(PRICE),
      tradeDate: shownTradeDate,
      // 미국 종목을 골랐으면 통화는 폼이 USD로 정했어야 한다 — KRW로 저장되면
      // 평가금액이 환율 배수만큼 틀린다.
      currency: "USD",
    });

    // 파생 보유가 이력과 같은 값을 말한다.
    const holdings = await user.api.get<UserHolding[]>("/user/holdings");
    const added = holdings.find((h) => h.symbol === S);
    expect(added, "매매는 남았는데 보유가 안 생기면 두 표가 어긋난 것").toBeDefined();
    expect(added).toMatchObject({
      symbol: S,
      quantity: Number(QUANTITY),
      avgPrice: Number(PRICE),
      costCurrency: "USD",
      source: "trades",
    });

    // 그리고 그 값이 표에 그려진다.
    const row = holdingItem(userPage, S);
    await expect(row).toBeVisible();
    await expect(row, "표의 수량이 저장값과 달라 보인다").toContainText(QUANTITY);
  });

  test("같은 값으로 한 번 더 저장해도 보유 수량이 두 배가 되지 않는다", async ({
    userPage,
    user,
  }) => {
    /*
     * 사용자가 두 번 누르는 일은 실제로 일어난다(응답이 느릴 때). 그때 보유가
     * 24주가 되면 화면상으로는 성공 두 번이지만 자산은 틀린다.
     */
    await seedExistingHolding(user);
    await userPage.goto("/holdings");
    await expectAddCardReady(userPage);

    for (let attempt = 0; attempt < 2; attempt += 1) {
      await pickSymbolInSearch(userPage);
      await userPage.getByLabel("수량").fill(QUANTITY);
      await userPage.getByLabel(/매수 단가/).fill(PRICE);
      await userPage.getByRole("button", { name: "보유에 추가" }).click();
      await expect(userPage.getByText(new RegExp(`${QUANTITY}주를 추가했습니다`))).toBeVisible();
    }

    const trades = await user.api.get<HoldingTrade[]>(`/user/holdings/trades?symbol=${S}`);
    expect(
      trades,
      `같은 매매가 ${trades.length}줄 저장됐다 — 재시도가 자산을 늘리면 안 된다`,
    ).toHaveLength(1);

    const holdings = await user.api.get<UserHolding[]>("/user/holdings");
    expect(
      holdings.find((h) => h.symbol === S)!.quantity,
      "두 번 눌러 24주가 되면 안 된다",
    ).toBe(Number(QUANTITY));
  });

  test("종목을 고르기 전에는 수량 · 단가 · 저장이 모두 잠겨 있다", async ({ userPage, user }) => {
    await seedExistingHolding(user);
    await userPage.goto("/holdings");
    await expectAddCardReady(userPage);

    // 심볼이 확정되지 않은 채로 저장되면 어느 종목인지 모르는 매매가 남는다.
    await expect(userPage.getByRole("button", { name: "보유에 추가" })).toBeDisabled();
    await expect(userPage.getByLabel("수량")).toBeDisabled();
    await expect(userPage.getByLabel(/매수 단가/)).toBeDisabled();

    expect(
      await user.api.get<HoldingTrade[]>(`/user/holdings/trades?symbol=${S}`),
      "아무것도 안 했는데 매매가 생겼다",
    ).toHaveLength(0);
  });
});
