import {
  RESEARCH_CAP_SYMBOLS,
  RESEARCH_DAILY_CAP,
  RESEARCH_SYMBOL_COOLDOWN_DAYS,
  SYMBOLS,
} from "@/data";
import { expect, test } from "@/fixtures";
import type { ResearchCall } from "@/types";

/**
 * 발행 제한 (INV-F1, F2).
 *
 * 이 두 규칙은 스팸 방지가 아니라 **성적표의 신뢰성** 장치다. 하루에 무한정
 * 발행할 수 있으면 아무 방향이나 뿌려 놓고 맞은 것만 남길 수 있고, 같은 종목을
 * 매일 다시 부를 수 있으면 한 종목에 대고 계속 다시 찍어 결국 맞힐 수 있다.
 */

function callBody(symbol: string, title: string) {
  return {
    symbol,
    title,
    direction: "buy" as const,
    horizonDays: 90,
    conviction: 50,
    bodyMd: "QA 발행 제한 검증",
    // 공개 피드를 오염시키지 않는다. 제한 로직은 가시성과 무관하다.
    visibility: "private" as const,
  };
}

test.describe("발행 제한", () => {
  test("같은 종목은 쿨다운이 끝나기 전에 다시 발행할 수 없다", async ({ user }) => {
    const first = await user.api.post<ResearchCall>(
      "/research/calls",
      callBody(SYMBOLS.research, "QA 첫 발행"),
    );
    expect(first.symbol).toBe(SYMBOLS.research);

    const res = await user.api.send(
      "POST",
      "/research/calls",
      callBody(SYMBOLS.research, "QA 재발행"),
    );
    expect(res.status(), "쿨다운 위반은 409로 알려야 한다").toBe(409);

    const body = (await res.json()) as { error: string; symbol: string; daysRemaining: number };
    expect(body.symbol).toBe(SYMBOLS.research);
    expect(
      body.daysRemaining,
      "작성 화면이 '언제부터 가능한지'를 말하려면 남은 일수가 필요하다",
    ).toBeGreaterThan(0);
    expect(body.daysRemaining).toBeLessThanOrEqual(RESEARCH_SYMBOL_COOLDOWN_DAYS);

    // 거부됐으면 콜은 하나뿐이어야 한다.
    const mine = await user.api.get<ResearchCall[]>("/research/mine");
    expect(mine.filter((c) => c.symbol === SYMBOLS.research)).toHaveLength(1);
  });

  test(`하루 ${RESEARCH_DAILY_CAP}건을 넘기면 ${RESEARCH_DAILY_CAP + 1}번째가 거부된다`, async ({
    user,
  }) => {
    // 발행 한 건마다 시세·점수·뉴스 스냅샷을 모은다 — 6건은 기본 타임아웃을 넘긴다.
    test.setTimeout(180_000);

    const symbols = RESEARCH_CAP_SYMBOLS.slice(0, RESEARCH_DAILY_CAP);
    for (const [index, symbol] of symbols.entries()) {
      const call = await user.api.post<ResearchCall>(
        "/research/calls",
        callBody(symbol, `QA 한도 ${index + 1}`),
      );
      expect(call.symbol).toBe(symbol);
    }

    const overflowSymbol = RESEARCH_CAP_SYMBOLS[RESEARCH_DAILY_CAP]!;
    const message = await user.api.expectFailure(
      "POST",
      "/research/calls",
      409,
      callBody(overflowSymbol, "QA 한도 초과"),
    );
    expect(message).toContain("한도");

    const mine = await user.api.get<ResearchCall[]>("/research/mine");
    expect(
      mine,
      `한도를 넘겨 저장되면 안 된다 — 현재 ${mine.length}건`,
    ).toHaveLength(RESEARCH_DAILY_CAP);
    expect(
      mine.map((c) => c.symbol),
      "거부된 종목이 저장돼 있으면 안 된다",
    ).not.toContain(overflowSymbol);
  });

  test("한 사람의 한도가 다른 사람을 막지 않는다", async ({ user, otherUser }) => {
    await user.api.post<ResearchCall>(
      "/research/calls",
      callBody(SYMBOLS.research, "QA 사용자 A"),
    );

    // A가 쿨다운에 걸린 바로 그 종목을 B는 발행할 수 있어야 한다 — 제한은
    // 계정별이지 종목별이 아니다.
    const bCall = await otherUser.api.post<ResearchCall>(
      "/research/calls",
      callBody(SYMBOLS.research, "QA 사용자 B"),
    );
    expect(bCall.userId).toBe(otherUser.id);
  });
});
