import { expect, test } from "@/fixtures";
import { holdingItem } from "@/ui";

/**
 * 비로그인 방문자의 「내 투자」 (브라우저 필수).
 *
 * 왜 브라우저인가: 이 화면의 분기가 전부 **클라이언트에서** 일어난다.
 * `AuthRequiredShell`이 세션 로딩 → 안내 화면 → `?sample=1`이면 본문 통과로
 * 갈라지고, 예시 모드 상태는 URL 쿼리와 `useSyncExternalStore`로 동기화된다.
 * API 응답만 보면 이 분기가 전혀 보이지 않는다.
 *
 * 검증하는 결과: 방문자가 로그인 없이 **실제 예시 데이터가 채워진 화면**에
 * 도달하는가. 「예시 보기 버튼이 눌린다」가 아니다.
 */

/** `/api/v1/sample/holdings` 픽스처의 고정 항목 — 서버가 늘 같은 것을 준다. */
const SAMPLE_HOLDING = { name: "삼성전자", symbol: "005930" };

/**
 * 「예시로 둘러보기」 버튼은 **두 곳**에 있다 — 페이지 헤더와 안내 카드 안.
 * 방문자가 실제로 마주치는 흐름은 본문 카드 쪽이므로 main 안의 것을 고른다.
 * `.first()`로 넘기면 어느 쪽을 눌렀는지 테스트만 봐서는 알 수 없다.
 *
 * `visible: true`가 붙은 이유: `next dev`는 같은 헤더/띠를 숨겨진 사본과 함께
 * 렌더한다(프로덕션 빌드에서는 2개, dev에서는 4개 — 실측). 사용자가 실제로
 * 누를 수 있는 것만 고른다.
 */
function sampleButton(page: import("@playwright/test").Page) {
  return page
    .getByRole("main")
    .getByRole("button", { name: /예시로 둘러보기/ })
    .filter({ visible: true });
}

function sampleOffButton(page: import("@playwright/test").Page) {
  return page.getByRole("button", { name: /예시 끄기/ }).filter({ visible: true }).first();
}

test.describe("비로그인 안내", () => {
  test("보호 메뉴는 로그인 요구 대신 무엇을 해주는 곳인지 설명한다", async ({ page }) => {
    await page.goto("/holdings");

    await expect(
      page.getByRole("heading", { name: /흩어져 있는 보유 종목을 한 화면으로 모읍니다/ }),
      "비로그인 방문자가 '로그인이 필요합니다'만 보면 가입 여부를 정할 근거가 없다",
    ).toBeVisible();

    // 다음 동작 두 개가 실제 링크·버튼으로 있어야 한다.
    await expect(page.getByRole("link", { name: /로그인하고 시작하기/ })).toBeVisible();
    await expect(sampleButton(page)).toBeVisible();

    // 로그인 링크는 돌아올 곳을 들고 간다 — 이게 없으면 로그인 후 홈으로 떨어진다.
    await expect(page.getByRole("link", { name: /로그인하고 시작하기/ })).toHaveAttribute(
      "href",
      /next=%2Fholdings/,
    );
  });

  test("예시 모드는 로그인 없이 진짜 데이터가 채워진 화면을 보여준다", async ({ page }) => {
    await page.goto("/holdings");
    await sampleButton(page).click();

    // 상태가 URL에 남는다 — 새로고침·뒤로가기·공유가 모두 이 위에서 돈다.
    await expect(page, "예시 모드가 URL에 없으면 새로고침하면 사라진다").toHaveURL(/\?sample=1/);

    // 안내 화면이 아니라 **본문**이 그려져야 한다.
    const row = holdingItem(page, SAMPLE_HOLDING.symbol);
    await expect(
      row,
      `예시 픽스처의 ${SAMPLE_HOLDING.name}(${SAMPLE_HOLDING.symbol}) 이 표에 있어야 한다 — ` +
        `빈 화면이면 예시 모드가 공개 픽스처를 못 읽은 것`,
    ).toBeVisible();
    await expect(row).toContainText(SAMPLE_HOLDING.name);

    // 남의 데이터를 내 것으로 읽는 사고를 막는 띠가 반드시 있어야 한다.
    await expect(
      page.getByText(/예시 데이터를 보고 있습니다/),
      "예시임을 알리는 띠가 없으면 방문자가 남의 포트폴리오를 자기 것으로 읽는다",
    ).toBeVisible();
  });

  test("예시 모드는 새로고침해도 유지된다", async ({ page }) => {
    await page.goto("/holdings?sample=1");
    await expect(holdingItem(page, SAMPLE_HOLDING.symbol)).toBeVisible();

    await page.reload();
    await expect(
      holdingItem(page, SAMPLE_HOLDING.symbol),
      "URL에 상태를 둔 이유가 새로고침 내성이다",
    ).toBeVisible();
  });

  /*
   * 고쳐진 결함 #1 (2026-08-23) — 예시를 꺼도 예시 데이터가 화면에 남았다.
   *
   * 두 군데가 겹쳐 있었다.
   *  1) `useSampleMode`가 주소창을 진실로 삼고 `useSearchParams`와 OR로 합쳤다.
   *     `router.replace`가 URL을 바꾸기 전에 notify가 먼저 돌아 구독자들이 여전히
   *     sample=1을 읽었고, OR은 한쪽만 참이어도 참이라 "켜짐"으로 눌러앉았다.
   *  2) 실데이터 적재 실패 경로가 `setHoldings((prev) => prev ?? [])`로 **이전 값을
   *     유지**했다. 비로그인이라 전부 401이었고, 그래서 예시 포트폴리오가 남았다.
   *
   * 지금은 토글 순간 모드를 동기로 확정하고(OR 제거), 모드가 바뀌면 데이터를
   * 먼저 버린다.
   */
  test("예시를 끄면 예시 데이터가 화면에서 사라진다", async ({ page }) => {
    await page.goto("/holdings?sample=1");
    await expect(holdingItem(page, SAMPLE_HOLDING.symbol)).toBeVisible();
    await expect(page.getByText(/예시 데이터를 보고 있습니다/)).toBeVisible();

    await sampleOffButton(page).click();

    await expect(page).toHaveURL(/\/holdings$/);
    await expect(
      page.getByRole("heading", { name: /흩어져 있는 보유 종목을 한 화면으로 모읍니다/ }),
      "예시를 껐는데 안내 화면으로 돌아오지 않았다",
    ).toBeVisible();
    await expect(
      holdingItem(page, SAMPLE_HOLDING.symbol),
      "예시를 껐는데 예시 데이터가 남아 있다 — 방문자가 남의 숫자를 자기 것으로 읽는다",
    ).toHaveCount(0);
    await expect(
      page.getByText(/예시 데이터를 보고 있습니다/),
      "띠도 함께 사라져야 한다",
    ).toHaveCount(0);
  });

  /*
   * 아래 둘은 **위 수정이 새로 만든 위험**을 막는다.
   *
   * 모드를 모듈 전역 값으로 들고 있게 되면서, 기록해 두지 않으면 한 화면에서 켠
   * 예시가 다른 화면까지 따라가고 뒤로가기로 돌아왔을 때 주소창과 어긋난다.
   * override에 pathname을 함께 담고 popstate에서 버리는 것이 그 대비이고, 이
   * 두 테스트가 그 대비를 지킨다.
   */
  test("한 화면에서 켠 예시가 다른 화면으로 새지 않는다", async ({ page }) => {
    await page.goto("/holdings");
    await sampleButton(page).click();
    await expect(page).toHaveURL(/sample=1/);
    await expect(holdingItem(page, SAMPLE_HOLDING.symbol)).toBeVisible();

    await page.getByRole("link", { name: "관심 종목" }).first().click();

    await expect(page).toHaveURL(/\/watchlist$/);
    await expect(
      page.getByRole("heading", { name: /지금 눈에 들어온 종목을/ }),
      "예시가 경로를 넘어 따라오면 다른 화면이 남의 데이터로 채워진다",
    ).toBeVisible();

    /*
     * 그리고 **되돌아왔을 때도** 새지 않아야 한다. 모드를 모듈 전역 값으로
     * 들고 있기 때문에, 떠날 때 버리지 않으면 죽은 값이 주소창을 이긴다.
     * push 이동은 popstate를 발생시키지 않아 그쪽 정리에 걸리지 않는다.
     */
    await page.getByRole("link", { name: "내 포트폴리오" }).first().click();

    await expect(page).toHaveURL(/\/holdings$/);
    await expect(
      page.getByRole("heading", { name: /흩어져 있는 보유 종목을 한 화면으로 모읍니다/ }),
      "sample=1 없이 돌아왔는데 예시가 켜져 있다 — 떠날 때 override가 안 버려진 것",
    ).toBeVisible();
    await expect(holdingItem(page, SAMPLE_HOLDING.symbol)).toHaveCount(0);
  });

  test("다른 화면에 갔다 뒤로 돌아오면 주소창이 다시 진실이 된다", async ({ page }) => {
    /*
     * 토글은 `router.replace`라 히스토리 항목을 남기지 않는다(의도된 설계 —
     * 예시를 켰다 껐다 한 것이 뒤로가기 스택을 채우면 안 된다). 그래서 popstate를
     * 검증하려면 **다른 화면으로 push 이동한 뒤** 돌아와야 한다.
     */
    await page.goto("/holdings");
    await sampleButton(page).click();
    await expect(page).toHaveURL(/sample=1/);
    await expect(holdingItem(page, SAMPLE_HOLDING.symbol)).toBeVisible();

    await page.getByRole("link", { name: "관심 종목" }).first().click();
    await expect(page).toHaveURL(/\/watchlist$/);

    await page.goBack();

    await expect(page).toHaveURL(/\/holdings\?sample=1$/);
    await expect(
      holdingItem(page, SAMPLE_HOLDING.symbol),
      "주소창에 sample=1이 있는데 예시가 꺼진 채라면 죽은 override가 주소창을 이긴 것",
    ).toBeVisible();
  });

  /*
   * 고쳐진 결함 #2 (2026-08-23) — 예시 화면이 인증 경로를 불렀다.
   *
   * 사이드 레일의 목표가 카드가 쓰는 `usePriceTargets()`는 모듈 싱글턴 스토어라
   * (`lib/price-targets.ts`) 구독하는 순간 조건 없이 `/user/price-targets`를 읽는다
   * — 예시 모드도 세션도 보지 않는다. 지금은 예시 모드에서 그 카드를 세우지
   * 않는다(같은 화면의 평가금액 차트와 같은 처리).
   *
   * `/auth/me`·`/auth/refresh`의 401은 결함이 아니다 — AuthProvider가 모든 방문자에
   * 대해 세션을 한 번 확인하는 정상 경로다. 그래서 제외하고 센다.
   */
  const SESSION_PROBES = ["/api/v1/auth/me", "/api/v1/auth/refresh"];

  test("예시 화면은 사용자 데이터 API를 부르지 않는다", async ({ page }) => {
    const userDataCalls: string[] = [];
    page.on("response", (res) => {
      const path = new URL(res.url()).pathname;
      if (!path.startsWith("/api/v1/")) return;
      if (SESSION_PROBES.includes(path)) return;
      if (res.status() === 401) userDataCalls.push(path);
    });

    await page.goto("/holdings?sample=1");
    await expect(holdingItem(page, SAMPLE_HOLDING.symbol)).toBeVisible();

    expect(
      userDataCalls,
      `예시 화면이 인증이 필요한 사용자 데이터 경로를 불렀다: ${userDataCalls.join(", ")}`,
    ).toEqual([]);
  });
});
