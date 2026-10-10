import { expect, test } from "@/fixtures";

/**
 * **같은 종목의 가격은 어느 메뉴에서도 같아야 한다** (2026-09-23 QA P0-01, T03).
 *
 * # 무엇이 깨져 있었나
 *
 * AI 추천 랭킹(`/stocks/scores/top`)은 장중에 확정 종가를 실시간 시세로 덮고
 * 그 사실을 `priceAsOf: "live"`로 말했다. 종목 스크리너(`POST /stocks/screen`)는
 * 같은 `stock_daily_closes`를 같은 모양의 LATERAL로 읽으면서 **그 오버레이만
 * 빼먹었다.**
 *
 * 그래서 이 결함은 **장이 닫혀 있는 동안 스스로를 숨겼다.** 2026-09-23 02:05 KST
 * 실측에서 두 응답은 소수점 열여섯 자리까지 같았다(리튬포어스 073570 · 2095원 ·
 * +1.6990291262135921). 외부 QA가 장중에 본 「랭킹 +1.70% vs 스크리너 +0.49%」는
 * 그 시간대에만 보이는 얼굴이었다.
 *
 * # 그래서 값을 비교하는 것으로는 못 잡는다
 *
 * 두 응답의 숫자를 대조하는 단언은 **장 마감 중에는 고쳐도 안 고쳐도 통과한다** —
 * CI가 도는 시각이 대부분 그렇다. 그러면 이 테스트는 회귀를 못 잡는다.
 *
 * 그래서 값 비교와 **계약 비교를 함께** 한다. 계약 쪽이 진짜 자물쇠다: 스크리너
 * 행이 `basePrice`·`priceAsOf`를 들고 있어야 오버레이를 **할 수 있고**, 했다고
 * **말할 수 있다**. 그 두 칸이 사라지면 장이 닫혀 있어도 빨개진다.
 */

type ScoreTop = {
  symbol: string;
  name?: string;
  score?: number;
  closePrice?: number;
  changeRate?: number;
  basePrice?: number;
  priceAsOf?: string;
  tradeDate?: string;
};

type ScreenRow = {
  symbol: string;
  name?: string;
  score?: number;
  close?: number;
  changeRate?: number;
  basePrice?: number;
  priceAsOf?: string;
};

type ScreenResult = {
  tradeDate: string;
  rows: ScreenRow[];
};

/** 화면의 기본 조건과 같다 — `fe/src/lib/screener-api.ts`의 `defaultScreenRules()`. */
const DEFAULT_RULES = [{ field: "score", min: 70 }];

test.describe("가격 일관성 — AI 추천 랭킹 ↔ 종목 스크리너", () => {
  test("스크리너 행은 실시간 오버레이에 필요한 두 칸을 들고 있다", async ({ anon }) => {
    const result = await anon.post<ScreenResult>("/stocks/screen", {
      market: "domestic",
      rules: DEFAULT_RULES,
      limit: 50,
    });

    test.skip(result.rows.length === 0, "점수 배치가 아직 안 돌아 비교할 행이 없다");

    /*
      `priceAsOf`는 **언제나** 있어야 한다. 이 값이 없으면 화면은 「이 숫자가 언제
      것인가」를 말할 수 없고, 서버는 실시간으로 덮었다는 사실을 전할 수 없다.
    */
    for (const row of result.rows) {
      expect(
        row.priceAsOf,
        `${row.symbol}: priceAsOf가 없으면 화면이 기준 시점을 말할 수 없다`,
      ).toBeTruthy();
    }

    /*
      `basePrice`는 **종가를 아는 행이면** 있어야 한다. 화면이 SSE로 받은 가격에서
      등락률을 다시 낼 때 쓰는 기준가라, 없으면 그 행만 장이 열리는 순간 등락률이
      사라진다.
    */
    const priced = result.rows.filter((r) => typeof r.close === "number" && r.close > 0);
    expect(priced.length, "종가를 아는 행이 하나는 있어야 비교가 뜻을 가진다").toBeGreaterThan(0);
    for (const row of priced) {
      expect(
        row.basePrice,
        `${row.symbol}: basePrice가 없으면 실시간 등락률을 낼 수 없다`,
      ).toBeGreaterThan(0);
    }
  });

  test("두 응답이 겹치는 종목은 price·changeRate·기준 시점이 모두 같다", async ({ anon }) => {
    /*
      **같은 시각에 받는다.** 순서대로 받으면 그 사이에 틱이 지나가 두 응답이
      정당하게 달라질 수 있고, 그때 이 테스트는 없는 결함을 신고한다.
    */
    const [top, screen] = await Promise.all([
      anon.get<ScoreTop[]>("/stocks/scores/top?market=domestic&limit=100"),
      anon.post<ScreenResult>("/stocks/screen", {
        market: "domestic",
        rules: DEFAULT_RULES,
        limit: 100,
      }),
    ]);

    test.skip(
      top.length === 0 || screen.rows.length === 0,
      "점수 배치가 아직 안 돌아 비교할 행이 없다",
    );

    const byScreen = new Map(screen.rows.map((r) => [r.symbol, r]));
    const overlap = top.filter((t) => byScreen.has(t.symbol));

    expect(
      overlap.length,
      "두 목록 다 「종합 점수 순」이라 상위권은 겹쳐야 한다 — 0이면 자격 규칙이 갈린 것",
    ).toBeGreaterThan(0);

    for (const t of overlap) {
      const s = byScreen.get(t.symbol)!;
      const where = `${t.name ?? t.symbol}(${t.symbol})`;

      expect(s.close, `${where}: 가격이 화면마다 다르면 어느 쪽을 믿을지 알 수 없다`).toBe(
        t.closePrice,
      );
      expect(s.changeRate, `${where}: 등락률이 화면마다 다르다`).toBe(t.changeRate);
      /*
        기준 시점까지 같아야 한다. 값만 맞고 시점이 다르면 **우연히 같은 것**이고,
        장이 열리는 순간 갈린다 — 이 결함이 정확히 그 모양이었다.
      */
      expect(s.priceAsOf, `${where}: 같은 가격인데 기준 시점이 다르다`).toBe(t.priceAsOf);
      expect(s.basePrice, `${where}: 기준가가 다르면 실시간 등락률이 갈린다`).toBe(t.basePrice);
    }
  });

  test("점수의 기준일도 두 응답이 같다", async ({ anon }) => {
    const [top, screen] = await Promise.all([
      anon.get<ScoreTop[]>("/stocks/scores/top?market=domestic&limit=1"),
      anon.post<ScreenResult>("/stocks/screen", {
        market: "domestic",
        rules: DEFAULT_RULES,
        limit: 1,
      }),
    ]);

    test.skip(top.length === 0, "점수 배치가 아직 안 돌았다");

    // 둘 다 `LatestScoredDate`를 읽는다 — 갈리면 한쪽이 어제 점수를 보고 있다.
    expect(screen.tradeDate, "같은 배치의 결과인데 기준일이 다르다").toBe(top[0].tradeDate);
  });
});
