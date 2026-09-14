import { SYMBOLS } from "@/data";
import { expect, test } from "@/fixtures";

/**
 * 시세 · 뉴스 · 실적 · 점수 — **구조 불변식** (커버리지 구멍 G-1~G-4).
 *
 * # 왜 이 네 묶음이 비어 있었나
 *
 * 전부 외부 데이터에 기대는 화면이라 「값」을 단언할 수가 없다. 뉴스 목록은
 * 30분마다 내용이 바뀌고, 실적 달력은 배치가 채우며, 점수는 하루 한 번 다시
 * 계산된다. 그래서 「삼성전자가 1위다」 같은 단언은 내일 빨개진다.
 *
 * # 그래서 무엇을 보나
 *
 * **값이 아니라 규칙**을 본다. 데이터가 비어 있어도 참이어야 하는 것들이다:
 * 응답 봉투의 모양, 파라미터가 실제로 응답을 좁히는가, 두 곳에서 센 같은 수가
 * 일치하는가, 없는 것을 물었을 때 500이 아니라 빈 목록이 오는가.
 *
 * 이 규칙들이 깨지면 화면은 **조용히 빈칸을 그린다** — 오류를 내지 않으므로
 * 이 테스트가 없으면 아무도 모른다.
 */

/* ────────────────────────────── 시세 (G-1) ────────────────────────────── */

test.describe("시세", () => {
  test("batch는 물어본 심볼만, 물어본 만큼 돌려준다", async ({ anon }) => {
    const rows = await anon.get<Array<{ symbol: string; currency: string }>>(
      `/quotes/batch?symbols=${SYMBOLS.trade},005930`,
    );
    expect(rows.length, "요청한 두 종목이 다 와야 한다").toBe(2);

    const got = rows.map((r) => r.symbol).sort();
    expect(got).toEqual([SYMBOLS.trade, "005930"].sort());

    // 통화는 종목의 성질이지 요청의 성질이 아니다 — 섞이면 환산이 통째로 틀린다.
    expect(rows.find((r) => r.symbol === "005930")!.currency).toBe("KRW");
    expect(rows.find((r) => r.symbol === SYMBOLS.trade)!.currency).toBe("USD");
  });

  test("없는 심볼을 물어도 500이 아니다", async ({ anon }) => {
    const res = await anon.send("GET", "/quotes/batch?symbols=ZZZZZZZZZ");
    expect(res.status(), "모르는 심볼 하나가 목록 API를 세우면 안 된다").toBeLessThan(500);
  });

  test("terminal-snapshot은 없는 종목에 404로 답한다", async ({ anon }) => {
    const res = await anon.send("GET", "/quotes/ZZZZZZZZZ/terminal-snapshot");
    expect(res.status()).toBe(404);
  });

  /*
    존재하는 종목의 상세는 **비어 있더라도 200**이어야 한다.
    과거에 `/stocks/{없는심볼}/summary`가 500을 냈고, 그때 화면은 "시세 로딩 중…"에
    영원히 갇혔다(FE_FUNCTIONAL_QA BUG-05). 그 계열의 회귀를 막는다.
  */
  test("종목 상세 계열은 500을 내지 않는다", async ({ anon }) => {
    for (const path of [
      `/stocks/005930`,
      `/stocks/005930/summary`,
      `/stocks/005930/score`,
      `/stocks/005930/warnings`,
      `/stocks/005930/daily-closes`,
      `/stocks/ZZZZZZZZZ/score`,
    ]) {
      const res = await anon.send("GET", path);
      expect(res.status(), `${path} 가 5xx다 — 화면이 로딩에 갇힌다`).toBeLessThan(500);
    }
  });
  /*
    **없는 종목은 404지 500이 아니다.**

    실측(2026-09-06): `/stocks/ZZZZZZZZZ/summary` 가 **500**이었다. 서비스는 이미
    「모르는 심볼」을 알아보고 있었는데 핸들러가 모든 실패를 500으로 덮고 있었다.
    화면은 `/stocks/{symbol}` 의 404를 보고 「찾을 수 없습니다」를 제대로 그리므로
    사용자에게는 보이지 않았지만, **오타 하나가 5xx 알람을 울리고** 있었다.
  */
  test("없는 종목의 요약은 404다 — 500이면 오타가 장애로 보고된다", async ({ anon }) => {
    const res = await anon.send("GET", "/stocks/ZZZZZZZZZ/summary");
    expect(res.status()).toBe(404);

    // 있는 종목은 그대로 200이어야 한다 — 검사가 정상 경로를 막지 않는다.
    const ok = await anon.send("GET", "/stocks/005930/summary");
    expect(ok.status()).toBe(200);
  });
});

/* ────────────────────────────── 뉴스 (G-2) ────────────────────────────── */

test.describe("뉴스", () => {
  test("limit이 목록 길이를 실제로 좁힌다", async ({ anon }) => {
    const three = await anon.get<unknown[]>("/news/domestic?limit=3");
    expect(three.length).toBeLessThanOrEqual(3);

    const ten = await anon.get<unknown[]>("/news/domestic?limit=10");
    expect(
      ten.length,
      "limit을 늘렸는데 목록이 안 늘면 파라미터가 무시되고 있다",
    ).toBeGreaterThanOrEqual(three.length);
  });

  test("기사에는 화면이 그리는 칸이 다 있다", async ({ anon }) => {
    const [first] = await anon.get<
      Array<{ id: string; market: string; headline: string; publishedAt: string }>
    >("/news/domestic?limit=1");
    test.skip(!first, "뉴스가 한 건도 없으면 검증할 것이 없다");
    if (!first) return;

    // 하나라도 빠지면 카드가 조용히 빈칸을 그린다.
    expect(first.id, "id 없이는 모달도 조회 기록도 못 연다").toBeTruthy();
    expect(first.headline).toBeTruthy();
    expect(first.market).toBe("domestic");
    expect(
      Number.isNaN(Date.parse(first.publishedAt)),
      "publishedAt이 날짜로 안 읽히면 「N시간 전」이 NaN이 된다",
    ).toBe(false);
  });

  test("국내와 해외는 서로의 기사를 섞지 않는다", async ({ anon }) => {
    const us = await anon.get<Array<{ market: string }>>("/news/us?limit=10");
    test.skip(us.length === 0, "해외 뉴스가 비어 있다");
    expect(
      us.every((a) => a.market === "us"),
      "해외 목록에 국내 기사가 섞이면 시장 전환이 의미를 잃는다",
    ).toBe(true);
  });

  /*
    키워드는 **집계**다. 화면이 「SK하이닉스 67」처럼 건수를 함께 그리므로,
    건수가 없거나 0이면 배지가 「SK하이닉스 0」이 된다.
  */
  test("키워드 집계는 건수를 함께 준다", async ({ anon }) => {
    const res = await anon.get<{
      keywords: Array<{ keyword: string; count: number }>;
      articleCount: number;
    }>("/news/domestic/keywords");

    expect(Array.isArray(res.keywords)).toBe(true);
    for (const k of res.keywords) {
      expect(k.keyword).toBeTruthy();
      expect(k.count, `${k.keyword} 의 건수가 0 이하다`).toBeGreaterThan(0);
    }
    expect(res.articleCount).toBeGreaterThanOrEqual(0);
  });

  test("모르는 시장 이름은 5xx가 아니다", async ({ anon }) => {
    const res = await anon.send("GET", "/news/nowhere/keywords");
    expect(res.status()).toBeLessThan(500);
  });
});

/* ────────────────────────────── 실적 (G-3) ────────────────────────────── */

test.describe("실적 달력", () => {
  /*
    **빈 달은 정상이다.** 국내 실적은 2·5·8·11월에 몰려 있어 9월은 0건이 맞다
    (화면도 그렇게 말한다). 그러니 빈 달이 오류가 되면 안 되고, 반대로 빈 달에
    500이 나면 화면은 달을 넘길 수단까지 잃는다.
  */
  test("일정이 없는 달도 빈 목록으로 정상 응답한다", async ({ anon }) => {
    for (const month of ["2026-09", "2031-01"]) {
      const res = await anon.send("GET", `/earnings/calendar?market=domestic&month=${month}`);
      expect(res.status(), `${month} 달력이 5xx다`).toBe(200);
      const body = (await res.json()) as { data: unknown[] };
      expect(Array.isArray(body.data), `${month} 이 배열이 아니다`).toBe(true);
    }
  });

  test("달을 옮기면 응답도 그 달을 따라간다", async ({ anon }) => {
    const rows = await anon.get<Array<{ eventDate?: string; date?: string }>>(
      "/earnings/calendar?market=domestic&month=2026-08",
    );
    test.skip(rows.length === 0, "8월도 비어 있으면 이 단언은 아무것도 증명하지 않는다");

    for (const r of rows) {
      const d = r.eventDate ?? r.date ?? "";
      expect(d.startsWith("2026-08"), `8월을 물었는데 ${d} 가 왔다`).toBe(true);
    }
  });

  test("요약·서프라이즈·통계는 비어 있어도 200이다", async ({ anon }) => {
    for (const path of [
      "/earnings/calendar/summary?market=domestic&month=2026-09",
      "/earnings/surprises?market=domestic",
      "/earnings/stats?market=domestic",
      "/earnings/highlights?market=domestic",
    ]) {
      const res = await anon.send("GET", path);
      expect(res.status(), `${path} 가 5xx다`).toBeLessThan(500);
    }
  });
});

/* ──────────────────────────── 점수 · 랭킹 (G-4) ──────────────────────────── */

test.describe("점수와 랭킹", () => {
  test("상위 점수는 순위대로 오고 limit을 지킨다", async ({ anon }) => {
    const rows = await anon.get<Array<{ rank: number; score: number; symbol: string }>>(
      "/stocks/scores/top?market=domestic&limit=5",
    );
    test.skip(rows.length === 0, "점수가 아직 계산되지 않았다");

    expect(rows.length).toBeLessThanOrEqual(5);

    // 순위는 1부터 빈틈없이, 점수는 내림차순이어야 화면의 「N위」가 말이 된다.
    rows.forEach((r, i) => {
      expect(r.rank, `${i}번째 행의 순위가 ${r.rank}다`).toBe(i + 1);
      expect(r.symbol).toBeTruthy();
    });
    for (let i = 1; i < rows.length; i++) {
      expect(
        rows[i]!.score,
        `${i}위(${rows[i - 1]!.score})보다 ${i + 1}위(${rows[i]!.score})가 높다`,
      ).toBeLessThanOrEqual(rows[i - 1]!.score);
    }
  });

  /*
    시장 어휘는 `domestic|overseas`다(BE `handler/score.go`). BE는 모르는 값을
    **조용히 domestic으로 눕힌다** — 관대한 기본값이라 문서에도 그렇게 적혀 있다.
    그래서 `market=US`로 물으면 국내 종목이 온다(실측). 오류가 아니라 규약이므로
    여기서는 규약대로 부른다.
  */
  test("시장을 바꾸면 다른 목록이 온다", async ({ anon }) => {
    const kr = await anon.get<Array<{ symbol: string }>>("/stocks/scores/top?market=domestic&limit=5");
    const us = await anon.get<Array<{ symbol: string }>>("/stocks/scores/top?market=overseas&limit=5");
    test.skip(kr.length === 0 || us.length === 0, "한쪽 시장의 점수가 비어 있다");

    const overlap = kr.filter((k) => us.some((u) => u.symbol === k.symbol));
    expect(overlap, "국내와 해외 랭킹에 같은 종목이 있으면 시장 필터가 안 먹은 것").toHaveLength(0);
  });

  test("점수는 0~100 안에 있고 등급이 함께 온다", async ({ anon }) => {
    const rows = await anon.get<Array<{ score: number; grade: string }>>(
      "/stocks/scores/top?market=domestic&limit=10",
    );
    test.skip(rows.length === 0, "점수가 아직 계산되지 않았다");
    for (const r of rows) {
      expect(r.score).toBeGreaterThanOrEqual(0);
      expect(r.score).toBeLessThanOrEqual(100);
      expect(r.grade, "등급이 없으면 화면의 배지가 빈칸이 된다").toBeTruthy();
    }
  });

  test("지수 점수는 지수마다 하나씩만 온다", async ({ anon }) => {
    const rows = await anon.get<Array<{ indexId: string; score: number }>>("/indices/scores");
    test.skip(rows.length === 0, "지수 점수가 비어 있다");

    const ids = rows.map((r) => r.indexId);
    expect(
      new Set(ids).size,
      "같은 지수가 두 줄이면 화면이 어느 쪽을 그릴지 알 수 없다",
    ).toBe(ids.length);
  });

  test("모르는 시장은 5xx가 아니다", async ({ anon }) => {
    const res = await anon.send("GET", "/stocks/scores/top?market=NOWHERE&limit=3");
    expect(res.status()).toBeLessThan(500);
  });
});
