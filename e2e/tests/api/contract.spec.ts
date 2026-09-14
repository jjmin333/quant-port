import { SYMBOLS } from "@/data";
import { expect, test } from "@/fixtures";
import type { ResearchCall } from "@/types";

/**
 * FE ↔ BE 계약 (INV-J군).
 *
 * FE에는 테스트가 하나도 없다. 그래서 FE가 기대는 형태가 바뀌었을 때 알려 주는
 * 것이 여기밖에 없다 — 화면은 조용히 빈 칸을 그리지, 에러를 내지 않는다.
 */

type ResearchReturnEntry = {
  horizon: string;
  asOfDate?: string;
  return?: number;
  alpha?: number;
};

test.describe("응답 봉투", () => {
  test("성공은 data 봉투를 쓴다", async ({ anon }) => {
    for (const path of [
      "/quotes/indices",
      "/rankings?type=TOP_GAINERS&marketCountry=KR&count=5",
      "/research/calls",
      "/sample/holdings",
    ]) {
      const res = await anon.send("GET", path);
      const body = (await res.json()) as Record<string, unknown>;
      expect(Object.keys(body), `${path} 응답에 data 키가 없다`).toContain("data");
    }
  });

  test("실패는 error 봉투를 쓴다", async ({ anon }) => {
    const res = await anon.send("GET", "/user/holdings");
    expect(res.status()).toBe(401);
    const body = (await res.json()) as Record<string, unknown>;
    expect(Object.keys(body), "에러 응답에 error 키가 없으면 FE가 메시지를 못 읽는다").toContain(
      "error",
    );
    expect(typeof body.error).toBe("string");
  });
});

/*
 * 검증 실패는 4xx다 — 5xx가 아니다.
 *
 * 고쳐진 결함(2026-08-23): `writeServiceError`가 못 알아본 에러는 전부
 * `writeTossError`로 떨어지고 그 기본 분기가 **502 Bad Gateway**였다. 검증 실패
 * 6경로 중 4곳이 그 길로 새서, 사용자가 음수 잔고나 빈 심볼을 보내면 "상류
 * 게이트웨이 장애"로 보고됐다. 클라이언트는 재시도할 문제와 고쳐 보낼 문제를
 * 구분하지 못하고, 사용자 오타가 5xx 알람을 울렸다.
 *
 * 이제 서비스가 `invalidInput(...)`으로 표식을 붙이고 핸들러가 400으로 매핑한다
 * (`service/invalid_input.go`). 메시지는 그대로다 — 이미 화면에 노출되는 문자열이라.
 */
test.describe("검증 실패의 상태코드", () => {
  const INVALID_INPUTS: Array<{
    name: string;
    method: "POST" | "PUT";
    path: string;
    body: unknown;
    message: string;
  }> = [
    {
      name: "현금 잔고 음수",
      method: "PUT",
      path: "/user/cash-balances",
      body: { krw: -1, usd: 0 },
      message: "cash amount must be >= 0",
    },
    {
      name: "리서치 발행 빈 심볼",
      method: "POST",
      path: "/research/calls",
      body: { symbol: "", title: "x", direction: "buy", bodyMd: "x", visibility: "private" },
      message: "symbol required",
    },
    {
      name: "리서치 미리보기 빈 심볼",
      method: "POST",
      path: "/research/preview",
      body: { symbol: "", direction: "buy" },
      message: "symbol required",
    },
    {
      name: "스탬프 빈 심볼",
      method: "POST",
      path: "/stamps",
      body: { symbol: "", targetPrice: 10, horizonKind: "1w" },
      message: "symbol required",
    },
  ];

  for (const { name, method, path, body, message } of INVALID_INPUTS) {
    test(`${name} 은 400과 원래 메시지로 거부된다`, async ({ user }) => {
      const res = await user.api.send(method, path, body);
      expect(
        res.status(),
        `${method} ${path} → ${res.status()}. 5xx는 "서버가 고장났으니 재시도하라"는 뜻이라 ` +
          `클라이언트가 입력을 고칠 수 없고, 사용자 오타가 5xx 알람을 울린다`,
      ).toBe(400);
      expect(
        ((await res.json()) as { error: string }).error,
        "메시지는 화면에 그대로 노출된다 — 상태코드를 고치면서 문구가 바뀌면 안 된다",
      ).toBe(message);
    });
  }

  test("검증 실패여도 상태는 바뀌지 않는다", async ({ user }) => {
    // 상태코드와 무관하게 **거부는 거부여야 한다**. 이 단언은 502가 400으로
    // 고쳐져도 그대로 성립한다 — 결함과 독립적인 진짜 불변식이다.
    await user.api.put("/user/cash-balances", { krw: 1_000, usd: 2 });
    await user.api.send("PUT", "/user/cash-balances", { krw: -1, usd: 0 });
    expect(await user.api.get("/user/cash-balances")).toEqual({ krw: 1_000, usd: 2 });

    await user.api.send("POST", "/research/calls", {
      symbol: "",
      title: "x",
      direction: "buy",
      bodyMd: "x",
      visibility: "private",
    });
    expect(
      await user.api.get<unknown[]>("/research/mine"),
      "심볼 없는 콜이 저장되면 안 된다",
    ).toHaveLength(0);
  });
});

test.describe("라우트 우선순위", () => {
  /*
   * 고정 경로가 `{symbol}` / `{id}` 패턴에 먹히면 조용히 다른 핸들러가 응답한다.
   * 200이 나오기 때문에 화면은 "빈 데이터"로 보이고, 원인은 라우터 등록 순서다.
   * chi는 등록 순서에 의존하므로 라우트를 추가할 때마다 깨질 수 있다.
   */
  test("/stocks/earnings/batch 는 'earnings'라는 종목으로 해석되지 않는다", async ({ anon }) => {
    const data = await anon.get<Record<string, unknown>>(
      `/stocks/earnings/batch?symbols=${SYMBOLS.trade}`,
    );
    expect(
      Object.keys(data),
      "심볼별 맵이 와야 한다 — 단일 종목 응답이면 {symbol} 라우트에 먹힌 것",
    ).toContain(SYMBOLS.trade);
  });

  test("/user/holdings/trades 는 'trades'라는 종목으로 해석되지 않는다", async ({ user }) => {
    const rows = await user.api.get<unknown[]>("/user/holdings/trades");
    expect(Array.isArray(rows), "매매 이력 목록(배열)이 와야 한다").toBe(true);
  });

  test("/user/holdings/valuation 은 'valuation'이라는 종목으로 해석되지 않는다", async ({
    user,
  }) => {
    const series = await user.api.get<{ points: unknown[]; exchangeRate: number }>(
      "/user/holdings/valuation?days=30",
    );
    expect(series, "평가액 시리즈 형태여야 한다").toHaveProperty("points");
    expect(Array.isArray(series.points)).toBe(true);
  });
});

test.describe("없는 값은 0이 아니다", () => {
  test("아직 종가가 없는 구간의 수익률은 값 자체가 없어야 한다", async ({ user }) => {
    const call = await user.api.post<ResearchCall & { returns: ResearchReturnEntry[] }>(
      "/research/calls",
      {
        symbol: SYMBOLS.research,
        title: "QA 결측 표현",
        direction: "buy",
        horizonDays: 90,
        bodyMd: "발행 직후에는 발행 이후 종가가 없다",
        visibility: "private",
      },
    );

    expect(call.returns.length, "1W/1M/3M 버킷은 항상 내려와야 한다").toBeGreaterThan(0);

    for (const entry of call.returns) {
      if (entry.asOfDate === undefined) {
        expect(
          entry.return,
          `${entry.horizon}: 근거 종가가 없는데 수익률이 있다 — ` +
            `0으로 채우면 화면이 '변동 없음'이라고 거짓말한다`,
        ).toBeUndefined();
        expect(entry.alpha, `${entry.horizon}: 근거 없는 알파`).toBeUndefined();
      } else {
        expect(
          typeof entry.return,
          `${entry.horizon}: 근거 종가는 있는데 수익률이 없다`,
        ).toBe("number");
      }
    }
  });
});

test.describe("공개/비공개 경계의 비대칭", () => {
  test("업로드된 이미지는 비로그인도 읽을 수 있고, 올리는 것만 로그인이 필요하다", async ({
    anon,
  }) => {
    // 공개 리포트 본문에 박히는 이미지라, 비로그인 독자가 못 읽으면 글이 깨진다.
    const res = await anon.send("GET", "/media/does-not-exist.png");
    expect(
      res.status(),
      "없는 파일은 404여야 한다 — 401이면 이미지 경로 전체가 잠긴 것",
    ).toBe(404);

    await anon.expectFailure("POST", "/media/images", 401);
  });

  /*
   * 샘플 응답에는 **실시간 시세로 계산되는 필드가 섞여 있다**(price, changeRate,
   * 수익률 등). 응답 전체를 비교하면 두 호출 사이 몇 초 만에 시세가 움직여
   * 실패하는데, 그건 세션 의존이 아니다. 그래서 픽스처가 소유한 필드만 고른다 —
   * 여기서 물어야 하는 것은 "로그인 여부가 예시 데이터를 바꾸는가"다.
   */
  const SAMPLE_STABLE_FIELDS: Record<string, string[]> = {
    "/sample/holdings": ["id", "symbol", "quantity", "avgPrice", "costCurrency", "source"],
    "/sample/watchlist": ["id", "symbol", "addedPrice", "createdAt"],
    "/sample/stamps/summary": ["total", "symbols", "dueCount", "judged", "hits"],
  };

  function pick(value: unknown, fields: string[]): unknown {
    if (Array.isArray(value)) return value.map((row) => pick(row, fields));
    if (value === null || typeof value !== "object") return value;
    const source = value as Record<string, unknown>;
    return Object.fromEntries(fields.filter((f) => f in source).map((f) => [f, source[f]]));
  }

  test("샘플 데이터는 세션과 무관하게 같은 픽스처를 준다", async ({ anon, user }) => {
    for (const [path, fields] of Object.entries(SAMPLE_STABLE_FIELDS)) {
      const asAnon = pick(await anon.get<unknown>(path), fields);
      const asUser = pick(await user.api.get<unknown>(path), fields);

      expect(
        JSON.stringify(asAnon),
        `${path} 픽스처가 비어 있으면 이 비교가 아무것도 증명하지 않는다`,
      ).not.toBe("[]");
      expect(
        asUser,
        `${path} 가 로그인 여부에 따라 달라지면 예시 화면이 사용자 데이터를 섞어 보여준다`,
      ).toEqual(asAnon);
    }
  });
});

test.describe("건강 상태", () => {
  test("/ready 는 의존성별 상태를 함께 알려준다", async ({ anon }) => {
    const res = await anon.ctx.get("/ready");
    const body = (await res.json()) as { status: string; checks: Record<string, string> };

    expect(res.status(), `준비되지 않은 상태: ${JSON.stringify(body)}`).toBe(200);
    expect(body.checks, "어떤 의존성이 죽었는지 알려주지 않으면 장애 대응이 느려진다").toHaveProperty(
      "mysql",
    );
    expect(body.checks).toHaveProperty("redis");
    expect(body.checks.mysql).toBe("ok");
  });
});

/*
카탈로그에 없는 심볼은 사용자 데이터가 되지 못한다 (2026-09-06 결정).

실측으로 드러난 모습: `POST /watchlist {"symbol":"ZZZZZZZZZ"}` 가 **201**이었고,
관심 목록에 이름도 시세도 없는 행이 하나 생겼다. 화면은 그것을 종목인 것처럼
한 줄 그린다. 매매 기록도 같아서, 아무 문자열이나 포지션이 될 수 있었다.

「없는 종목」은 입력 형식이 틀린 것(400)이 아니라 **그런 것이 없다**이므로 404다.
*/
test.describe("없는 종목", () => {
  const NOPE = "ZZZZZZZZZ";

  test("관심 종목에 담기지 않는다", async ({ user }) => {
    const res = await user.api.send("POST", "/watchlist", { symbol: NOPE });
    expect(res.status(), "없는 종목이 관심 목록에 담기면 안 된다").toBe(404);

    const list = await user.api.get<Array<{ symbol: string }>>("/watchlist");
    expect(
      list.some((w) => w.symbol === NOPE),
      "거부됐는데 행이 생기면 목록에 빈 줄이 남는다",
    ).toBe(false);
  });

  test("매매 이력에 기록되지 않는다", async ({ user }) => {
    const res = await user.api.send("POST", "/user/holdings/trades", {
      symbol: NOPE,
      side: "buy",
      quantity: 1,
      price: 100,
      tradeDate: "2026-01-05",
    });
    expect(res.status(), "없는 종목으로 포지션이 생기면 안 된다").toBe(404);

    expect(await user.api.get<unknown[]>("/user/holdings/trades")).toHaveLength(0);
    expect(await user.api.get<unknown[]>("/user/holdings")).toHaveLength(0);
  });

  test("있는 종목은 그대로 담긴다 — 검사가 정상 경로를 막지 않는다", async ({ user }) => {
    const res = await user.api.send("POST", "/watchlist", { symbol: SYMBOLS.watch });
    expect(res.ok(), `${SYMBOLS.watch} 는 카탈로그에 있어야 한다`).toBe(true);

    const list = await user.api.get<Array<{ symbol: string }>>("/watchlist");
    expect(list.some((w) => w.symbol === SYMBOLS.watch)).toBe(true);
  });
});
