import { defineConfig, devices } from "@playwright/test";

import { FE_URL } from "./src/env";

const isCI = !!process.env.CI;

export default defineConfig({
  testDir: "./tests",
  globalSetup: "./global-setup.ts",

  fullyParallel: true,
  forbidOnly: isCI,

  /*
   * 재시도는 0이 기본이다.
   *
   * 흔한 설정(`retries: 2`)은 실패를 통계로 덮는다. 이 스위트는 sleep이 없고
   * 계정을 공유하지 않으므로 재시도로 살아나는 실패는 대부분 **진짜 경합**이거나
   * 외부 시세 지연이고, 그건 알아야 하는 정보다. CI에서만 1회를 허용하는 것은
   * 시세 API가 순간적으로 느릴 때 파이프라인 전체를 세우지 않기 위해서다.
   */
  retries: isCI ? 1 : 0,

  /*
   * 워커 상한 2 (`E2E_WORKERS`로 조절).
   *
   * BE 레이트 리밋은 **비로그인 요청을 IP당 30 req/s(버스트 60)** 로 센다
   * (`internal/middleware/ratelimit.go`). 테스트마다 가입 1~2회가 `/auth/*`를
   * 치고 그 경로는 언제나 IP 버킷이라, 워커를 늘리면 테스트가 아니라 리미터에
   * 걸려 429로 죽는다(실측: 4워커에서 재현).
   *
   * `Api.send`가 429를 물러섰다 다시 보내므로 4워커에서도 대개 통과하지만,
   * 기본값은 재시도에 기대지 않는 쪽으로 둔다 — 재시도가 흔해지면 그게 실행
   * 시간의 대부분이 되고, 진짜 지연을 가린다.
   */
  workers: Number(process.env.E2E_WORKERS ?? 2),

  /* 리서치 발행·스탬프 생성은 시세·뉴스·점수 스냅샷을 모으므로 기본 30초로는 빠듯하다. */
  timeout: 60_000,
  expect: { timeout: 10_000 },

  reporter: isCI
    ? [["github"], ["html", { open: "never" }]]
    : [["list"], ["html", { open: "never" }]],

  use: {
    trace: "retain-on-failure",
    screenshot: "only-on-failure",
    video: "off",
    /* 느린 시세 백필이 걸린 요청도 통과시키되, 무한정 기다리지는 않는다. */
    actionTimeout: 15_000,
    navigationTimeout: 30_000,
  },

  projects: [
    {
      /*
       * API 계층. 브라우저를 띄우지 않는다 — 여기서 검증하는 것은 전부 HTTP
       * 계약이고, 같은 것을 UI로 다시 도는 것은 중복이자 flaky의 주된 출처다.
       */
      name: "api",
      testDir: "./tests/api",
    },
    {
      /* 브라우저를 반드시 거쳐야만 검증되는 시나리오만. */
      name: "ui",
      testDir: "./tests/ui",
      use: {
        ...devices["Desktop Chrome"],
        baseURL: FE_URL,
        /*
         * 뷰포트를 명시한다. 보유 화면은 `xl`(1280px)에서 사이드 레일이 본문 옆으로
         * 붙는데, 종목 추가 폼이 그 레일 안에 있다. 기본값(1280)은 경계선 위라
         * 브레이크포인트가 한 칸만 움직여도 레이아웃이 바뀐다.
         */
        viewport: { width: 1440, height: 900 },
      },
    },
  ],
});
