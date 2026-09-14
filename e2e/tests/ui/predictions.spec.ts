import type { Page } from "@playwright/test";

import { SYMBOLS } from "@/data";
import { expect, test } from "@/fixtures";

/**
 * 「목표가 예측」 — 스탬프와 목표가 관리를 합친 화면.
 *
 * 이 화면의 위험은 전부 **합치는 자리**에 있다. BE는 아직 두 테이블이라
 * (`stamps` / `user_price_targets`) 화면이 두 응답을 한 배열로 접는데, 그 접합이
 * 어긋나면 증상이 조용하다 — 목록이 반쪽만 그려지거나, 기한 없는 예측이 적중률
 * 분모에 섞여 들어가거나, 손절가 칸이 저장되지 않은 채 저장됐다고 보인다.
 * 어느 것도 화면이 빨개지지 않는다.
 *
 * 그래서 여기서 묻는 것은 넷이다:
 *   1. 두 출처가 **한 목록**에 함께 온다
 *   2. 기한이 **저장 경로를 가른다** (기한 있음 → 스탬프 / 무기한 → 목표가)
 *   3. 개봉 대기는 **결과를 가리고**, 열면 채점이 나온다
 *   4. 375px에서 목록이 **카드로 바뀐다** (행은 그 폭에서 성립하지 않는다)
 */

const LIST = "예측 목록";

/** dev 서버는 숨겨진 사본까지 렌더한다 — 실제로 누를 수 있는 것만 고른다. */
function visible(page: Page, name: string | RegExp) {
  return page.getByRole("button", { name }).filter({ visible: true }).first();
}

test.describe("목표가 예측 — 두 기능의 통합", () => {
  test("기한 있는 예측과 무기한 예측이 한 목록에 함께 나온다", async ({ user, userPage }) => {
    /*
     * 사전 조건은 API로 만든다 — 검증 대상은 **합쳐진 목록**이지 등록 폼이 아니다.
     * 스탬프는 만기가 있는 쪽, 목표가는 무기한 쪽으로 들어와야 한다.
     */
    await user.api.post("/user/price-targets", {
      symbol: SYMBOLS.watch,
      targetPrice: 999,
      currency: "USD",
      note: "무기한 예측 QA",
    });
    await user.api.post("/stamps", {
      symbol: SYMBOLS.stamp,
      targetPrice: 999,
      horizonKind: "1m",
      conviction: 4,
      note: "기한 예측 QA",
    });

    await userPage.goto("/predictions");

    await expect(
      userPage.getByText("무기한 예측 QA").first(),
      "목표가에서 온 예측이 목록에 없다 — 두 출처 중 한쪽만 접힌 것",
    ).toBeVisible();
    await expect(
      userPage.getByText("기한 예측 QA").first(),
      "스탬프에서 온 예측이 목록에 없다",
    ).toBeVisible();

    // 무기한은 채점 대상이 아니다. 목록에 그 사실이 적혀 있어야 한다.
    await expect(
      userPage.getByText("무기한").filter({ visible: true }).first(),
      "무기한 예측에 기한 표시가 없다 — 채점 여부를 화면에서 알 수 없게 된다",
    ).toBeVisible();
  });

  test("적중률 분모에는 기한 있는 예측만 들어간다", async ({ user, userPage }) => {
    /*
     * 무기한 예측은 기다리면 언젠가 목표에 닿을 수 있다. 같은 분모에 넣으면
     * 적중률이 부풀어 오르므로, 화면은 「적중률」과 「무기한 도달」을 나란히 두되
     * 합치지 않는다. 방금 만든 예측은 아직 판정 전이라 분모가 0이어야 한다.
     */
    await user.api.post("/user/price-targets", {
      symbol: SYMBOLS.watch,
      targetPrice: 999,
      currency: "USD",
      note: "무기한 예측 QA",
    });

    await userPage.goto("/predictions");

    await expect(
      userPage.getByText(/판정 0건 중 0건 적중/),
      "판정 전인데 적중률 분모가 0이 아니다 — 무기한 예측이 섞여 들어갔다",
    ).toBeVisible();
    await expect(
      userPage.getByText("무기한 도달"),
      "무기한 도달률 칸이 없다 — 두 숫자가 합쳐졌을 수 있다",
    ).toBeVisible();
  });

  test("기한을 고르면 손절가·2차 목표 칸이 잠긴다", async ({ userPage }) => {
    /*
     * BE가 아직 갈라져 있어 **손절가·2차 목표는 무기한 예측에만 저장된다.**
     * 칸을 조용히 무시하면 사용자는 적어 둔 줄 안다 — 잠그고 이유를 적는 것이
     * 이 화면이 지켜야 할 계약이다.
     */
    await userPage.goto("/predictions");
    await visible(userPage, "예측 세우기").click();

    const dialog = userPage.getByRole("dialog");
    await expect(dialog).toBeVisible();

    await dialog.getByPlaceholder(/종목명·티커 검색/).fill("MSFT");
    await userPage.getByRole("option").first().click();

    await dialog.getByRole("button", { name: /자세히/ }).click();

    // 기본 기한은 1개월(= 기한 있음)이라 두 칸이 잠겨 있어야 한다.
    const stop = dialog.getByLabel("손절가");
    await expect(stop, "기한 있는 예측인데 손절가 칸이 열려 있다").toBeDisabled();
    await expect(
      dialog.getByText(/손절가·2차 목표는 무기한 예측에만/),
      "왜 잠겼는지 화면이 말하지 않는다",
    ).toBeVisible();

    // 무기한으로 바꾸면 열려야 한다.
    await dialog.getByRole("button", { name: "무기한", exact: true }).click();
    await expect(stop, "무기한인데 손절가 칸이 잠겨 있다").toBeEnabled();
    await expect(
      dialog.getByText(/채점하지 않습니다/),
      "무기한이 채점되지 않는다는 사실이 화면에 없다",
    ).toBeVisible();
  });

  test("무기한으로 저장하면 목표가로, 기한을 정하면 스탬프로 들어간다", async ({
    user,
    userPage,
  }) => {
    await userPage.goto("/predictions");
    await visible(userPage, "예측 세우기").click();

    const dialog = userPage.getByRole("dialog");
    await dialog.getByPlaceholder(/종목명·티커 검색/).fill(SYMBOLS.watch);
    await userPage.getByRole("option").first().click();

    await dialog.getByRole("button", { name: "무기한", exact: true }).click();
    await dialog.getByRole("button", { name: /예측 저장/ }).click();
    await expect(dialog.getByText(/예측을 저장했습니다/)).toBeVisible();

    const targets = await user.api.get<{ symbol: string }[]>("/user/price-targets");
    expect(
      targets.some((t) => t.symbol === SYMBOLS.watch),
      "무기한 예측이 목표가 테이블에 저장되지 않았다",
    ).toBe(true);

    const stamps = await user.api.get<{ symbol: string }[]>("/stamps");
    expect(
      stamps.some((s) => s.symbol === SYMBOLS.watch),
      "무기한 예측인데 스탬프로도 저장됐다 — 한 번의 저장이 두 곳에 남으면 목록에 두 번 나온다",
    ).toBe(false);
  });

  test("개봉 대기는 결과를 가리고, 열어야 채점이 나온다", async ({ userPage }) => {
    /*
     * 만기가 지난 예측은 예시 픽스처에만 있다(실계정으로는 한 달을 기다려야 한다).
     * 가리는 규칙 자체가 이 화면에 다시 들어올 이유이므로 픽스처로 검증한다.
     */
    await userPage.goto("/predictions?sample=1");

    await expect(userPage.getByText("개봉할 때가 됐습니다")).toBeVisible();
    await expect(
      userPage.getByText(/결과는 열어야 보입니다 — 그때 뭐라고 했는지/).first(),
      "결과가 가려져 있지 않다 — 개봉이 사건이 아니라 숫자 확인이 된다",
    ).toBeVisible();

    await visible(userPage, "열어서 채점 보기").click();

    await expect(
      userPage.getByText("벤치마크 대비").first(),
      "열었는데 채점이 나오지 않는다",
    ).toBeVisible();
    await expect(
      visible(userPage, "그때와 지금 대조"),
      "열린 카드에서 상세로 가는 길이 없다",
    ).toBeVisible();
  });

  test("삭제는 확인을 한 번만 받고, 두 출처가 같은 흐름을 쓴다", async ({ user, userPage }) => {
    /*
     * 상세 모달은 출처에 따라 다른 본문을 그린다(스탬프 상세 vs 무기한 상세).
     * 삭제 확인을 각자 두면 한쪽만 확인을 두 번 받는 화면이 된다 — 실제로 그렇게
     * 만들었다가 되돌린 자리라 회귀 방어를 건다.
     */
    await user.api.post("/user/price-targets", {
      symbol: SYMBOLS.watch,
      targetPrice: 999,
      currency: "USD",
      note: "삭제 대상 QA",
    });

    await userPage.goto("/predictions");
    await userPage.getByText("삭제 대상 QA").first().click();

    const dialog = userPage.getByRole("dialog");
    await expect(dialog.getByText("가격 사다리")).toBeVisible();
    await dialog.getByRole("button", { name: "삭제" }).click();

    /*
     * 앱 공통 확인 대화상자(`useConfirm` → AlertDialog)가 뜨고, 거기서 한 번만
     * 확인한다. **`alertdialog`로 좁혀서 집는다** — 상세 모달에도 「삭제」 버튼이
     * 있어서 이름만으로 고르면 확인을 안 거치고 같은 버튼을 다시 누를 수 있다.
     */
    const confirm = userPage.getByRole("alertdialog");
    await expect(confirm.getByText(/이 예측을 삭제할까요/)).toBeVisible();
    await confirm.getByRole("button", { name: "삭제" }).click();

    await expect(
      userPage.getByText("삭제 대상 QA"),
      "삭제했는데 목록에 남아 있다",
    ).toHaveCount(0);

    const targets = await user.api.get<{ symbol: string }[]>("/user/price-targets");
    expect(targets.some((t) => t.symbol === SYMBOLS.watch), "서버에서 지워지지 않았다").toBe(
      false,
    );
  });

  test("375px에서 등록 모달이 화면 밖으로 나가지 않는다", async ({ userPage }) => {
    await userPage.setViewportSize({ width: 375, height: 812 });
    await userPage.goto("/predictions");
    await visible(userPage, "예측 세우기").click();

    const dialog = userPage.getByRole("dialog");
    await expect(dialog).toBeVisible();
    await dialog.getByPlaceholder(/종목명·티커 검색/).fill(SYMBOLS.watch);
    await userPage.getByRole("option").first().click();
    await dialog.getByRole("button", { name: /자세히/ }).click();

    const box = await dialog.boundingBox();
    expect(box, "모달을 못 찾았다").not.toBeNull();
    expect(box!.x, "모달이 왼쪽으로 잘렸다").toBeGreaterThanOrEqual(0);
    expect(box!.x + box!.width, "모달이 오른쪽으로 넘쳤다").toBeLessThanOrEqual(375);

    const overflow = await userPage.evaluate(
      () => document.documentElement.scrollWidth - window.innerWidth,
    );
    expect(overflow, "모달을 열었더니 가로 스크롤이 생겼다").toBeLessThanOrEqual(0);
  });

  test("375px에서는 목록이 카드로 바뀐다", async ({ userPage }) => {
    /*
     * 행은 375px에서 성립하지 않는다 — 가격 축도, 게이지도, 숫자 세 칸도 들어갈
     * 폭이 없다. 좁은 화면에서 행을 억지로 유지하면 가로 스크롤이 생기고,
     * 그건 이 앱에서 가장 눈에 띄는 회귀다.
     */
    await userPage.setViewportSize({ width: 375, height: 812 });
    await userPage.goto("/predictions?sample=1");
    await expect(userPage.getByText(LIST)).toBeVisible();

    const overflow = await userPage.evaluate(
      () => document.documentElement.scrollWidth - window.innerWidth,
    );
    expect(overflow, "375px에서 가로 스크롤이 생겼다").toBeLessThanOrEqual(0);

    // 카드에만 있는 「자세히」 버튼이 보여야 한다 (행에는 없다).
    await expect(
      visible(userPage, "자세히"),
      "375px인데 카드가 아니라 행이 그려졌다",
    ).toBeVisible();
  });
});
