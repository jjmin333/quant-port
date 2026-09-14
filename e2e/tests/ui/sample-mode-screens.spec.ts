import type { Page } from "@playwright/test";

import { expect, test } from "@/fixtures";
import { holdingItem } from "@/ui";

/**
 * 예시 모드 — 「내 투자」 다섯 화면 공통 계약 (회귀 방어).
 *
 * 왜 이 파일이 생겼나: 예시 모드의 진실을 주소창에서 **모듈 스토어**로 옮기는
 * 수정을 했는데(`components/common/sample-mode.tsx`), 그 훅을 홀딩스·관심종목·
 * 스탬프·메모장·목표가 **다섯 화면이 공유**한다. 결함은 홀딩스에서 발견했지만
 * 수정의 영향은 다섯 곳 전부에 간다. 한 화면만 보고 넘어가면 나머지 넷이 조용히
 * 깨진다.
 *
 * 화면마다 다른 것은 픽스처와 문구뿐이고, 지켜야 하는 계약은 같다:
 *   1. 비로그인은 안내 화면을 본다
 *   2. 예시를 켜면 공개 픽스처가 실제로 그려진다 (+ 예시임을 알리는 띠)
 *   3. 예시를 끄면 **데이터도 띠도 함께 사라지고** 안내 화면으로 돌아온다
 *
 * 3번이 고쳤던 결함이다 — 껐는데 남의 숫자가 표시 없이 남아 있었다.
 */

type SampleScreen = {
  name: string;
  path: string;
  /** 비로그인 안내 화면의 제목 (`lib/menu-copy.ts`). */
  emptyHeadline: RegExp;
  /** 예시 픽스처에만 있는, 화면에 실제로 그려지는 값. */
  fixtureText: string;
};

const SCREENS: SampleScreen[] = [
  {
    name: "내 포트폴리오",
    path: "/holdings",
    emptyHeadline: /흩어져 있는 보유 종목을 한 화면으로 모읍니다/,
    fixtureText: "삼성전자",
  },
  {
    name: "관심 종목",
    path: "/watchlist",
    emptyHeadline: /지금 눈에 들어온 종목을, 담은 가격과 함께 남겨둡니다/,
    fixtureText: "삼성전자",
  },
  {
    name: "스탬프",
    path: "/stamps",
    emptyHeadline: /지금 생각을 적어두면, 나중에 맞았는지 알려줍니다/,
    fixtureText: "삼성전자",
  },
  {
    name: "메모장",
    path: "/journal",
    emptyHeadline: /샀던 이유는 생각보다 빨리 잊힙니다/,
    fixtureText: "삼성전자 — 왜 담았나",
  },
  {
    name: "목표가 관리",
    path: "/targets",
    emptyHeadline: /가격은 오르내릴 때가 아니라 조용할 때 정해두는 게 낫습니다/,
    fixtureText: "삼성전자",
  },
  /*
   * 「목표가 예측」은 위 두 화면(스탬프·목표가 관리)을 합친 새 메뉴다. 예시 모드에서
   * **픽스처 두 벌을 함께** 읽는 유일한 화면이라, 한쪽만 오면 조용히 반쪽짜리
   * 목록이 그려진다 — 같은 계약을 여기에도 건다.
   */
  {
    name: "목표가 예측",
    path: "/predictions",
    emptyHeadline: /얼마까지, 언제까지 갈 거라고 보십니까/,
    fixtureText: "삼성전자",
  },
];

/**
 * `next dev`는 헤더·띠를 숨겨진 사본과 함께 렌더한다(프로덕션 빌드 2개 / dev 4개,
 * 실측). 사용자가 실제로 누를 수 있는 것만 고른다.
 */
function sampleOnButton(page: Page) {
  return page.getByRole("button", { name: /예시로 둘러보기|^예시$/ }).filter({ visible: true }).first();
}

function sampleOffButton(page: Page) {
  return page.getByRole("button", { name: /예시 끄기/ }).filter({ visible: true }).first();
}

/*
 * 로그인 사용자의 예시 토글.
 *
 * 위 다섯 묶음은 전부 비로그인이다. 그런데 모드 전환 시 데이터를 버리도록 고친
 * 곳(`holdings-page-client`)은 **로그인 사용자에게도 똑같이 돈다**. 여기서 잘못
 * 되면 증상이 반대 방향으로 나타난다 — 예시를 끈 뒤 내 데이터가 안 돌아오거나,
 * 예시를 켰는데 내 종목이 섞여 보인다. 어느 쪽도 비로그인 테스트로는 안 잡힌다.
 */
test.describe("예시 모드 — 로그인 사용자", () => {
  const MY_SYMBOL = "TSLA";
  const FIXTURE_SYMBOL = "005930";

  test("내 데이터와 예시 데이터가 섞이지 않고, 끄면 내 것이 돌아온다", async ({
    user,
    userPage,
  }) => {
    await user.api.post("/user/holdings/trades", {
      symbol: MY_SYMBOL,
      side: "buy",
      quantity: 4,
      price: 300,
      currency: "USD",
      tradeDate: "2026-05-05",
    });

    const mine = holdingItem(userPage, MY_SYMBOL);
    const fixture = holdingItem(userPage, FIXTURE_SYMBOL);

    await userPage.goto("/holdings");
    await expect(mine, "내 보유 종목이 보여야 한다").toBeVisible();

    // 예시 켜기 — 내 것은 사라지고 픽스처만 보여야 한다.
    await sampleOnButton(userPage).click();
    await expect(userPage).toHaveURL(/sample=1/);
    await expect(fixture, "예시 픽스처가 그려져야 한다").toBeVisible();
    await expect(
      mine,
      "예시 화면에 내 종목이 섞이면 어느 숫자가 내 것인지 알 수 없다",
    ).toHaveCount(0);

    // 예시 끄기 — 내 것이 돌아오고 픽스처는 사라져야 한다.
    await sampleOffButton(userPage).click();
    await expect(userPage).toHaveURL(/\/holdings$/);
    await expect(
      mine,
      "예시를 껐는데 내 데이터가 안 돌아왔다 — 모드 전환 시 초기화만 하고 재적재가 안 된 것",
    ).toBeVisible();
    await expect(
      fixture,
      "예시를 껐는데 픽스처가 남아 있다",
    ).toHaveCount(0);
  });

  test("목표가 화면: 예시를 껐을 때 내 목표가가 다시 적재된다", async ({ user, userPage }) => {
    /*
     * 이 경로가 목표가 스토어 수정의 가장 아픈 자리다.
     *
     * 스토어는 구독 시점에 한 번만 적재하는데, 예시 모드에서는 그 적재를 건너뛴다
     * (인증 경로를 부르지 않으려고). 「목표가 관리」 화면은 예시를 켜고 끄는 동안
     * **계속 구독 상태**라 새 구독이 일어나지 않는다 — 재시도 장치가 없으면 예시를
     * 끈 뒤 내 목표가가 영영 비어 있게 된다. 보유 화면은 카드가 새로 마운트돼서
     * 이 증상이 안 나타나므로, 여기서만 잡힌다.
     */
    await user.api.post("/user/price-targets", {
      symbol: MY_SYMBOL,
      targetPrice: 500,
      note: "QA 목표가",
    });

    const mine = userPage.getByText("QA 목표가");
    const fixture = userPage.getByText(/HBM 실적 반영 시 재평가/);

    /*
     * **예시 모드로 바로 진입해야** 이 테스트에 의미가 있다. `/targets`를 먼저
     * 열면 스토어가 그때 실데이터를 다 받아 `loaded`가 서 버려서, 이후 토글은
     * 재적재를 필요로 하지 않는다 — 재시도 장치를 지워도 통과하는 테스트가 된다.
     * 예시로 진입하면 스토어는 적재를 건너뛴 상태로 남는다.
     */
    await userPage.goto("/targets?sample=1");
    await expect(fixture, "예시 픽스처 목표가가 보여야 한다").toBeVisible();
    await expect(mine, "예시 화면에 내 목표가가 섞이면 안 된다").toHaveCount(0);

    await sampleOffButton(userPage).click();
    await expect(userPage).toHaveURL(/\/targets$/);
    await expect(
      mine,
      "예시를 껐는데 내 목표가가 안 돌아왔다 — 예시 중 건너뛴 적재의 재시도가 없는 것",
    ).toBeVisible();
    await expect(fixture, "예시를 껐는데 픽스처가 남아 있다").toHaveCount(0);
  });
});

for (const screen of SCREENS) {
  test.describe(`예시 모드 — ${screen.name}`, () => {
    test("비로그인은 안내 화면을 본다", async ({ page }) => {
      await page.goto(screen.path);
      await expect(
        page.getByRole("heading", { name: screen.emptyHeadline }),
        `${screen.path} 가 안내 화면을 보여주지 않는다`,
      ).toBeVisible();
    });

    test("예시를 켜면 공개 픽스처가 실제로 그려진다", async ({ page }) => {
      await page.goto(`${screen.path}?sample=1`);

      await expect(
        page.getByText(screen.fixtureText).first(),
        `${screen.path} 예시 화면에 픽스처("${screen.fixtureText}")가 없다 — ` +
          `빈 화면이면 공개 픽스처를 못 읽은 것`,
      ).toBeVisible();
      await expect(
        page.getByText(/예시 데이터를 보고 있습니다/),
        "예시임을 알리는 띠가 없으면 방문자가 남의 데이터를 자기 것으로 읽는다",
      ).toBeVisible();
    });

    test("예시를 끄면 데이터와 띠가 함께 사라진다", async ({ page }) => {
      await page.goto(`${screen.path}?sample=1`);
      await expect(page.getByText(screen.fixtureText).first()).toBeVisible();

      await sampleOffButton(page).click();

      await expect(page).toHaveURL(new RegExp(`${screen.path}$`));
      await expect(
        page.getByRole("heading", { name: screen.emptyHeadline }),
        "예시를 껐는데 안내 화면으로 돌아오지 않았다",
      ).toBeVisible();
      await expect(
        page.getByText(screen.fixtureText),
        `예시를 껐는데 픽스처("${screen.fixtureText}")가 남아 있다 — ` +
          `방문자가 남의 숫자를 아무 표시 없이 자기 것으로 읽는다`,
      ).toHaveCount(0);
      await expect(page.getByText(/예시 데이터를 보고 있습니다/)).toHaveCount(0);
    });

    test("예시 화면은 사용자 데이터 API를 부르지 않는다", async ({ page }) => {
      // 세션 확인(`/auth/me`·`/auth/refresh`)은 모든 방문자에게 도는 정상 경로다.
      const sessionProbes = ["/api/v1/auth/me", "/api/v1/auth/refresh"];
      const userDataCalls: string[] = [];
      page.on("response", (res) => {
        const path = new URL(res.url()).pathname;
        if (!path.startsWith("/api/v1/")) return;
        if (sessionProbes.includes(path)) return;
        if (res.status() === 401) userDataCalls.push(path);
      });

      await page.goto(`${screen.path}?sample=1`);
      await expect(page.getByText(screen.fixtureText).first()).toBeVisible();

      expect(
        [...new Set(userDataCalls)],
        `${screen.path} 예시 화면이 인증 경로를 불렀다: ${userDataCalls.join(", ")}`,
      ).toEqual([]);
    });
  });
}
