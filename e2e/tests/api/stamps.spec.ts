import { SYMBOLS, STAMP_COOLDOWN_SECONDS } from "@/data";
import { expect, test } from "@/fixtures";
import type { Stamp, StampState } from "@/types";

/**
 * 스탬프 (INV-S군, D3).
 *
 * 스탬프의 핵심은 "그 순간을 봉인한다"이다. 그래서 검증할 것은 **서버가 무엇을
 * 클라이언트에게서 받지 않는가**다 — 가격도, 방향(stance)도 받지 않는다. 받는
 * 순간 사용자가 유리한 값을 적어 넣을 수 있고 채점이 무의미해진다.
 */

const S = SYMBOLS.stamp;

async function currentPrice(api: { get: <T>(p: string) => Promise<T> }): Promise<number> {
  const quote = await api.get<{ currentPrice: number }>(`/quotes/${S}`);
  expect(quote.currentPrice, `${S} 시세가 있어야 스탬프를 찍을 수 있다`).toBeGreaterThan(0);
  return quote.currentPrice;
}

test.describe("방향은 서버가 정한다", () => {
  test("목표가가 현재가보다 충분히 높으면 강세, 낮으면 약세로 봉인된다", async ({
    user,
    otherUser,
  }) => {
    const price = await currentPrice(user.api);

    const bullish = await user.api.post<Stamp>("/stamps", {
      symbol: S,
      targetPrice: Math.round(price * 1.2 * 100) / 100,
      horizonKind: "1w",
      conviction: 4,
      // 클라이언트가 반대 방향을 우겨 넣어도 무시되어야 한다.
      stance: "bearish",
    });
    expect(
      bullish.stance,
      "클라이언트가 보낸 stance가 반영되면 '목표가는 위인데 약세'인 스탬프가 생긴다",
    ).toBe("bullish");
    expect(bullish.price, "가격은 서버가 캡처한 값이어야 한다").toBeGreaterThan(0);

    const bearish = await otherUser.api.post<Stamp>("/stamps", {
      symbol: S,
      targetPrice: Math.round(price * 0.8 * 100) / 100,
      horizonKind: "1w",
      stance: "bullish",
    });
    expect(bearish.stance).toBe("bearish");
  });

  test("목표가가 현재가의 ±3% 안이면 관망으로 봉인된다", async ({ user }) => {
    const price = await currentPrice(user.api);
    const stamp = await user.api.post<Stamp>("/stamps", {
      symbol: S,
      targetPrice: Math.round(price * 1.01 * 100) / 100,
      horizonKind: "1w",
    });
    expect(stamp.stance, "1% 차이는 관망 밴드 안이다").toBe("neutral");
  });

  test("클라이언트가 보낸 가격은 캡처를 덮어쓰지 못한다", async ({ user }) => {
    const price = await currentPrice(user.api);
    const stamp = await user.api.post<Stamp>("/stamps", {
      symbol: S,
      targetPrice: Math.round(price * 1.1 * 100) / 100,
      horizonKind: "1w",
      // 서버가 이 값을 믿으면 목표가 오차·알파가 전부 조작 가능해진다.
      price: 1,
      prevClose: 1,
    });
    expect(stamp.price, "봉인 가격이 클라이언트 값(1)이 되면 안 된다").not.toBe(1);
    expect(Math.abs(stamp.price - price) / price).toBeLessThan(0.5);
  });
});

test.describe("목표가 범위", () => {
  test("현재가의 10배를 넘는 목표가는 거부되고 스탬프가 만들어지지 않는다", async ({ user }) => {
    const price = await currentPrice(user.api);

    const res = await user.api.send("POST", "/stamps", {
      symbol: S,
      targetPrice: price * 50,
      horizonKind: "1w",
    });
    expect(res.ok(), "자릿수 오타를 그대로 받으면 평균 오차 통계가 영구히 망가진다").toBe(false);

    expect(await user.api.get<Stamp[]>("/stamps"), "거부된 스탬프가 저장되면 안 된다").toHaveLength(
      0,
    );
  });

  test("목표가가 없으면 스탬프를 찍을 수 없다", async ({ user }) => {
    const res = await user.api.send("POST", "/stamps", { symbol: S, horizonKind: "1w" });
    expect(res.ok(), "목표가 없는 스탬프는 채점할 수 없다 — 북마크와 같아진다").toBe(false);
    expect(await user.api.get<Stamp[]>("/stamps")).toHaveLength(0);
  });
});

test.describe("쿨다운", () => {
  test("같은 종목은 연달아 찍히지 않고, 남은 시간이 상태로 보고된다", async ({ user }) => {
    const price = await currentPrice(user.api);
    const target = Math.round(price * 1.1 * 100) / 100;

    const first = await user.api.post<Stamp>("/stamps", {
      symbol: S,
      targetPrice: target,
      horizonKind: "1w",
    });
    expect(first.id).toBeGreaterThan(0);

    await user.api.expectFailure("POST", "/stamps", 409, {
      symbol: S,
      targetPrice: target,
      horizonKind: "1w",
    });

    const state = await user.api.get<StampState>(`/stamps/state?symbol=${S}`);
    expect(state.count, "찍힌 것은 한 개여야 한다").toBe(1);
    expect(
      state.cooldownRemainingSeconds,
      "화면이 '몇 초 뒤 가능'을 말하려면 남은 시간이 필요하다",
    ).toBeGreaterThan(0);
    expect(state.cooldownRemainingSeconds).toBeLessThanOrEqual(STAMP_COOLDOWN_SECONDS);

    expect(await user.api.get<Stamp[]>("/stamps")).toHaveLength(1);
  });

  test("다른 종목은 쿨다운에 걸리지 않는다", async ({ user }) => {
    const price = await currentPrice(user.api);
    await user.api.post<Stamp>("/stamps", {
      symbol: S,
      targetPrice: Math.round(price * 1.1 * 100) / 100,
      horizonKind: "1w",
    });

    const other = await user.api.get<{ currentPrice: number }>(`/quotes/${SYMBOLS.watch}`);
    const second = await user.api.post<Stamp>("/stamps", {
      symbol: SYMBOLS.watch,
      targetPrice: Math.round(other.currentPrice * 1.1 * 100) / 100,
      horizonKind: "1w",
    });

    expect(second.symbol).toBe(SYMBOLS.watch);
    expect(await user.api.get<Stamp[]>("/stamps")).toHaveLength(2);
  });
});

test.describe("수정 범위", () => {
  test("메모 수정은 봉인된 가격과 방향을 건드리지 못한다", async ({ user }) => {
    const price = await currentPrice(user.api);
    const stamp = await user.api.post<Stamp>("/stamps", {
      symbol: S,
      targetPrice: Math.round(price * 1.2 * 100) / 100,
      horizonKind: "1w",
      conviction: 2,
      note: "최초 메모",
    });

    const updated = await user.api.patch<Stamp>(`/stamps/${stamp.id}`, {
      conviction: 5,
      note: "고친 메모",
      tags: ["qa"],
      // ↓ 무시되어야 하는 것들
      price: 1,
      targetPrice: 1,
      stance: "bearish",
      stampedAt: "2000-01-01T00:00:00Z",
    });

    expect(updated.note).toBe("고친 메모");
    expect(updated.conviction).toBe(5);
    expect(updated.price, "봉인 가격이 수정으로 바뀌면 안 된다").toBe(stamp.price);
    expect(updated.targetPrice, "목표가는 메모 수정 경로로 바뀌면 안 된다").toBe(stamp.targetPrice);
    expect(updated.stance, "방향은 목표가에서 파생된다 — 임의로 못 바꾼다").toBe(stamp.stance);
    expect(updated.stampedAt).toBe(stamp.stampedAt);
  });

  test("유예 시간 안에서는 예측을 고칠 수 있고, 방향이 다시 파생된다", async ({ user }) => {
    const price = await currentPrice(user.api);
    const stamp = await user.api.post<Stamp>("/stamps", {
      symbol: S,
      targetPrice: Math.round(price * 1.2 * 100) / 100,
      horizonKind: "1w",
    });
    expect(stamp.stance).toBe("bullish");

    const revised = await user.api.patch<Stamp>(`/stamps/${stamp.id}/prediction`, {
      targetPrice: Math.round(price * 0.8 * 100) / 100,
      horizonKind: "1m",
    });

    expect(revised.stance, "목표가를 아래로 고쳤으면 방향도 따라와야 한다").toBe("bearish");
    expect(revised.horizonKind).toBe("1m");
    expect(revised.price, "예측을 고쳐도 봉인 가격은 그대로다").toBe(stamp.price);
    expect(revised.dueAt, "기간을 바꿨으면 마감도 다시 계산되어야 한다").not.toBe(stamp.dueAt);
  });
});

test.describe("소유권", () => {
  test("남의 스탬프는 읽지도 지우지도 못하고, 원본이 남는다", async ({ user, otherUser }) => {
    const price = await currentPrice(user.api);
    const stamp = await user.api.post<Stamp>("/stamps", {
      symbol: S,
      targetPrice: Math.round(price * 1.1 * 100) / 100,
      horizonKind: "1w",
    });

    await otherUser.api.expectFailure("GET", `/stamps/${stamp.id}`, 404);
    await otherUser.api.send("DELETE", `/stamps/${stamp.id}`);

    expect(
      await user.api.get<Stamp[]>("/stamps"),
      "남의 삭제 요청이 스탬프를 지웠다",
    ).toHaveLength(1);
  });
});
