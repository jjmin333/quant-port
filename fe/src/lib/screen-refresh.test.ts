import { describe, expect, it } from "vitest";
import { NEWS_REFRESH_MS, SCREEN_REFRESH_MS, SCREEN_SLOW_REFRESH_MS } from "@/lib/screen-refresh";
import { DEFAULT_CACHE_TTL_MS } from "@/lib/fetch-cache";
import { EARNINGS_TTL } from "@/lib/earnings-api";
import { NEWS_AGGREGATE_TTL } from "@/lib/news-api";

/*
갱신 주기와 **클라이언트 캐시 TTL의 관계**를 잠근다.

주기가 캐시 TTL보다 짧거나 같으면 그 회차는 조용히 캐시로 떨어진다 — 요청은 나가지
않고 화면은 그대로인데, 코드만 보면 「5분마다 갱신한다」고 적혀 있다. 화면이 낡아 있던
증상을 고치러 와서 **고친 척만 남기는** 실패라, 값 하나를 만질 때 여기서 걸린다.
*/
describe("화면 갱신 주기", () => {
  it("빠른 주기는 기본 GET 캐시보다 길다", () => {
    expect(SCREEN_REFRESH_MS).toBeGreaterThan(DEFAULT_CACHE_TTL_MS);
  });

  it("느린 주기는 실적 하이라이트 캐시보다 길다", () => {
    expect(SCREEN_SLOW_REFRESH_MS).toBeGreaterThan(EARNINGS_TTL);
  });

  it("느린 주기가 빠른 주기보다 짧아지지 않는다", () => {
    expect(SCREEN_SLOW_REFRESH_MS).toBeGreaterThanOrEqual(SCREEN_REFRESH_MS);
  });

  /*
    뉴스 주기는 세 값 사이에 끼어 있다 — 아래로는 두 캐시(30초·60초)보다 길어야
    왕복이 일어나고, 위로는 5분보다 짧아야 따로 둔 값이 된다.
  */
  it("뉴스 주기는 두 클라이언트 캐시보다 길다", () => {
    expect(NEWS_REFRESH_MS).toBeGreaterThan(DEFAULT_CACHE_TTL_MS);
    expect(NEWS_REFRESH_MS).toBeGreaterThan(NEWS_AGGREGATE_TTL);
  });

  it("뉴스 주기는 일반 화면 주기보다 빠르다", () => {
    expect(NEWS_REFRESH_MS).toBeLessThan(SCREEN_REFRESH_MS);
  });
});
