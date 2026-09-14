import { expect, test } from "@/fixtures";
import type { Api } from "@/api-client";

/** 카탈로그 검색 응답의 한 행. 이 스펙이 쓰는 필드만 적는다. */
type StockInfo = {
  symbol: string;
  name: string;
  securityType?: string;
  assetType?: string;
};

/**
 * 종목 검색 랭킹 — 2026-08-26 QA에서 잡은 세 가지를 굳힌다.
 *
 * 원래 증상:
 *
 *  1. **같은 검색어에 호출마다 다른 결과.** 「삼성」 결과가 한 칸씩 밀렸다. 원인은
 *     `english_name`에 한글이 그대로 복사된 행만 +1,200점을 더 받는 것이었고, 검색이
 *     1위 종목의 카탈로그를 보강하며 그 값을 영문으로 고쳐 놓는 바람에 **검색할수록
 *     순위가 흘러갔다**.
 *  2. **「삼성」 상위 20개가 전부 ETN** — 삼성전자는 21번째였다.
 *  3. **종목코드로 못 찾음.** `005930`이 0건이었다(라틴 질의를 전부
 *     `asset_type != 'domestic'`로 몰아냈다).
 *
 * 비로그인으로도 부를 수 있는 공개 경로라 계정이 필요 없다.
 */

type SearchResponse = { data: StockInfo[] | null; total: number };

async function search(api: Api, q: string, limit = 8): Promise<StockInfo[]> {
  const rows = await api.get<StockInfo[] | null>(
    `/stocks/search?limit=${limit}&offset=0&q=${encodeURIComponent(q)}`,
  );
  return rows ?? [];
}

test.describe("종목 검색 랭킹", () => {
  test("같은 검색어는 몇 번을 불러도 같은 순서를 준다", async ({ anon }) => {
    const runs: string[][] = [];
    for (let i = 0; i < 4; i++) {
      runs.push((await search(anon, "삼성")).map((s) => s.symbol));
    }
    for (let i = 1; i < runs.length; i++) {
      expect(
        runs[i] ?? [],
        `${i + 1}번째 호출이 첫 호출과 다르다 — 검색이 자기 데이터를 바꾸고 있을 수 있다`,
      ).toEqual(runs[0] ?? []);
    }
  });

  test("그룹명으로 찾으면 본주가 파생상품보다 앞에 온다", async ({ anon }) => {
    const rows = await search(anon, "삼성");
    expect(rows.length, "「삼성」에 결과가 있어야 한다").toBeGreaterThan(0);
    expect(rows[0]?.symbol, "삼성전자가 1위여야 한다").toBe("005930");

    /*
     * 상위권이 시가총액 순인지 — ETN이 본주 사이에 끼어들지 않는지 본다.
     * 파생상품을 목록에서 빼지는 않는다(그러면 찾을 방법이 사라진다). 뒤로 갈 뿐이다.
     */
    const top5 = rows.slice(0, 5);
    for (const row of top5) {
      expect(
        (row.securityType ?? "").toUpperCase(),
        `${row.symbol}(${row.name})가 상위 5위 안에 있으면 안 된다`,
      ).not.toContain("ETN");
    }
  });

  test("파생상품도 여전히 검색된다 — 뒤로 밀 뿐 걷어내지 않는다", async ({ anon }) => {
    const rows = await search(anon, "삼성 인버스");
    expect(rows.length, "「삼성 인버스」로는 ETN이 나와야 한다").toBeGreaterThan(0);
    expect(rows.some((r) => (r.securityType ?? "").toUpperCase().includes("ETN"))).toBe(true);
  });

  test("국내 종목코드로 찾을 수 있다", async ({ anon }) => {
    const exact = await search(anon, "005930");
    expect(exact.length, "`005930`은 삼성전자를 찾아야 한다").toBeGreaterThan(0);
    expect(exact[0]?.symbol).toBe("005930");

    // 접두어도 통해야 한다 — 코드를 다 치기 전에 후보가 보이는 것이 자동완성의 요점이다.
    const prefix = await search(anon, "00593");
    expect(prefix.map((s) => s.symbol)).toContain("005930");
  });

  test("미국 티커 검색은 그대로다", async ({ anon }) => {
    const rows = await search(anon, "NVDA");
    expect(rows[0]?.symbol, "정확히 일치하는 티커가 1위여야 한다").toBe("NVDA");
  });
});
