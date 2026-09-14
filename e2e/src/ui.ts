import type { Page } from "@playwright/test";

/**
 * 폼이 **하이드레이션을 마쳤는지** 기다린다.
 *
 * 왜 필요한가: 서버가 내려준 HTML에는 `<form>`이 이미 있고 버튼도 눌린다. 그런데
 * React가 아직 붙기 전이면 `onSubmit`의 `preventDefault`가 없어서 **브라우저가
 * 폼을 그대로 GET 전송**해 버린다. 실측으로 `/login` → `/login?` 로 이동하고
 * 입력값이 날아갔다. `next dev`는 라우트를 요청 시점에 컴파일하므로 이 창이
 * 몇 초까지 벌어진다.
 *
 * 왜 sleep이 아닌가: 기다려야 하는 조건이 정확히 무엇인지 알고 있기 때문이다 —
 * "React가 이 폼에 props를 붙였는가". `waitForTimeout`은 그 조건을 시간으로
 * 어림잡는 것이고, `networkidle`은 폴링이 계속 도는 이 앱에서는 아예 오지 않는다.
 *
 * 대가: React 내부 속성(`__reactProps$…`)에 기댄다. React가 이 이름을 바꾸면
 * 여기가 먼저 깨진다 — 그때는 이 함수 하나만 고치면 된다.
 */
export async function waitForHydration(page: Page, selector = "form"): Promise<void> {
  await page.waitForFunction(
    (sel) => {
      const el = document.querySelector(sel);
      if (!el) return false;
      return Object.keys(el).some((key) => key.startsWith("__reactProps$"));
    },
    selector,
    { timeout: 30_000 },
  );
}

/**
 * 보유 목록에서 한 종목의 항목을 집는다 — **보기 모드와 무관하게.**
 *
 * 보유 목록은 표(`role=row`)와 카드 둘로 그려지고, 기본값은 사용자 설정
 * (`localStorage: quant-port-holdings-view`)에 달려 있다. 2026-09-06에 기본값이
 * 표 → 카드로 바뀌자 `getByRole("row")`로 쓴 스펙 9개가 한꺼번에 빨개졌다 —
 * 화면은 멀쩡했고 데이터도 있었는데 **로케이터만 한쪽 보기를 알고 있었다.**
 *
 * 그래서 둘 다 맞춘다. 카드는 `aria-label="{이름} {심볼}"`인 `role=button`이고
 * (`holdings-list.tsx`의 `HoldingCards`), 표는 같은 텍스트를 담은 `role=row`다.
 * 어느 쪽이 기본이 되든 이 함수만 참이면 스펙은 그대로 살아 있다.
 */
export function holdingItem(page: Page, symbol: string) {
  const card = page.getByRole("button", { name: new RegExp(`(?:^| )${symbol}$`) });
  const row = page.getByRole("row").filter({ hasText: symbol });
  return card.or(row);
}
