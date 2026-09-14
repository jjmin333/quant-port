import { SYMBOLS } from "@/data";
import { expect, test } from "@/fixtures";
import type { ResearchCall } from "@/types";

/**
 * 리서치 콜의 동결 계약 (INV-D1, D2, R14).
 *
 * 이 기능의 전부는 "발행한 뒤에는 못 고친다"는 약속이다. 저자가 발행 후 기준가나
 * 목표가를 손볼 수 있으면 적중률은 성적표가 아니라 광고가 된다
 * (`internal/model/research.go` 머리말).
 *
 * 그래서 여기서는 **고칠 수 있는 것과 없는 것을 한 번의 PATCH로 동시에 시도**한다.
 * 나눠서 시도하면 "서버가 모르는 필드라 무시했다"와 "지켰다"를 구분하지 못한다.
 */

const S = SYMBOLS.research;

/** 공개 피드를 오염시키지 않으려고 기본은 비공개다 — 콜은 삭제할 수 없다. */
async function publishPrivateCall(
  api: { post: <T>(p: string, b?: unknown) => Promise<T> },
  over: Record<string, unknown> = {},
): Promise<ResearchCall> {
  return api.post<ResearchCall>("/research/calls", {
    symbol: S,
    title: "QA 동결 검증",
    direction: "buy",
    horizonDays: 90,
    conviction: 60,
    bodyMd: "최초 본문",
    visibility: "private",
    ...over,
  });
}

test.describe("발행 후 동결", () => {
  test("수정 요청은 서술만 바꾸고 판정에 쓰이는 값은 하나도 건드리지 못한다", async ({ user }) => {
    const published = await publishPrivateCall(user.api);

    const frozen = {
      direction: published.direction,
      basePrice: published.basePrice,
      horizonDays: published.horizonDays,
      conviction: published.conviction,
      scoreAtPublish: published.scoreAtPublish,
      publishedAt: published.publishedAt,
      tradeDate: published.tradeDate,
      status: published.status,
    };

    // 고쳐도 되는 것과 고치면 안 되는 것을 한꺼번에 보낸다.
    const updated = await user.api.patch<ResearchCall>(`/research/calls/${published.id}`, {
      title: "QA 수정된 제목",
      bodyMd: "수정된 본문",
      visibility: "private",
      tags: ["qa"],
      // ↓ 전부 무시되어야 하는 것들
      direction: "sell",
      basePrice: 1,
      targetPrice: 9,
      stopPrice: 8,
      horizonDays: 365,
      conviction: 1,
      scoreAtPublish: 100,
      status: "target_hit",
      publishedAt: "2000-01-01T00:00:00Z",
      tradeDate: "2000-01-01",
    });

    expect(updated.title, "제목은 고칠 수 있어야 한다").toBe("QA 수정된 제목");
    expect(updated.bodyMd, "본문은 고칠 수 있어야 한다").toBe("수정된 본문");
    expect(updated.bodyRev, "본문을 고치면 리비전이 올라가야 한다").toBeGreaterThan(
      published.bodyRev,
    );

    expect(
      {
        direction: updated.direction,
        basePrice: updated.basePrice,
        horizonDays: updated.horizonDays,
        conviction: updated.conviction,
        scoreAtPublish: updated.scoreAtPublish,
        publishedAt: updated.publishedAt,
        tradeDate: updated.tradeDate,
        status: updated.status,
      },
      "판정에 쓰이는 값이 하나라도 바뀌면 적중률을 사후에 조작할 수 있다",
    ).toEqual(frozen);

    // 응답만이 아니라 **다시 읽었을 때도** 그대로여야 한다.
    const reread = await user.api.get<ResearchCall>(`/research/calls/${published.id}`);
    expect(reread.basePrice).toBe(frozen.basePrice);
    expect(reread.direction).toBe(frozen.direction);
  });

  test("콜은 삭제되지 않는다 — 철회해도 기록은 남는다", async ({ user }) => {
    const published = await publishPrivateCall(user.api);

    // 삭제 라우트 자체가 없다. 405든 404든, 성공(2xx)만 아니면 된다.
    const deleteAttempt = await user.api.send("DELETE", `/research/calls/${published.id}`);
    expect(
      deleteAttempt.ok(),
      "삭제가 가능하면 저자가 빗나간 콜만 지워 적중률을 만들 수 있다",
    ).toBe(false);

    const retracted = await user.api.post<ResearchCall>(`/research/calls/${published.id}/retract`);
    expect(retracted.status).toBe("retracted");
    expect(retracted.retractedAt, "철회 시각이 기록되어야 한다").toBeTruthy();

    // 철회해도 저자의 성적표에는 남는다 — 철회율이 의미를 가지려면 그래야 한다.
    const mine = await user.api.get<ResearchCall[]>("/research/mine");
    expect(mine.map((c) => c.id)).toContain(published.id);
    expect(mine.find((c) => c.id === published.id)!.status).toBe("retracted");

    // 철회된 콜의 동결 값도 그대로다.
    expect(retracted.basePrice).toBe(published.basePrice);
    expect(retracted.direction).toBe(published.direction);
  });
});

test.describe("입력 정규화", () => {
  test("허용되지 않는 기간은 기본값 90일로 눕혀진다", async ({ user }) => {
    const call = await publishPrivateCall(user.api, { horizonDays: 45 });
    expect(
      call.horizonDays,
      "자유로운 기간을 허용하면 '같은 기간' 비교가 성립하지 않는다",
    ).toBe(90);
  });

  test("확신도는 1~100으로 눌린다", async ({ user, otherUser }) => {
    const tooHigh = await publishPrivateCall(user.api, { conviction: 900 });
    expect(tooHigh.conviction, "100을 넘는 확신도는 100으로 잘려야 한다").toBe(100);

    const missing = await publishPrivateCall(otherUser.api, { conviction: 0 });
    expect(missing.conviction, "확신도를 안 주면 기본 50").toBe(50);
  });
});

test.describe("가시성", () => {
  test("비공개 콜은 남에게도 비로그인에게도 존재하지 않는다", async ({ user, otherUser, anon }) => {
    const call = await publishPrivateCall(user.api);

    await otherUser.api.expectFailure("GET", `/research/calls/${call.id}`, 404);
    await anon.expectFailure("GET", `/research/calls/${call.id}`, 404);

    // 작성자 본인에게는 보인다.
    const own = await user.api.get<ResearchCall>(`/research/calls/${call.id}`);
    expect(own.id).toBe(call.id);
  });

  test("남의 콜은 수정도 철회도 되지 않고, 원본은 그대로다", async ({ user, otherUser }) => {
    const call = await publishPrivateCall(user.api, { visibility: "public" });

    await otherUser.api.expectFailure("PATCH", `/research/calls/${call.id}`, 404, {
      title: "탈취",
      bodyMd: "탈취",
    });
    await otherUser.api.expectFailure("POST", `/research/calls/${call.id}/retract`, 404);

    const intact = await user.api.get<ResearchCall>(`/research/calls/${call.id}`);
    expect(intact.title, "남이 제목을 바꿀 수 있으면 안 된다").toBe(call.title);
    expect(intact.status, "남이 철회할 수 있으면 안 된다").toBe("open");
  });

  test("공개 콜은 비로그인 피드에 나타나고, 비공개로 되돌리면 사라진다", async ({ user, anon }) => {
    const call = await publishPrivateCall(user.api, {
      visibility: "public",
      title: `QA 공개 피드 ${Date.now()}`,
    });

    const feed = await anon.get<ResearchCall[]>(`/research/calls?symbol=${S}&limit=100`);
    expect(
      feed.map((c) => c.id),
      "공개로 발행했는데 피드에 없으면 기능이 성립하지 않는다",
    ).toContain(call.id);

    await user.api.patch(`/research/calls/${call.id}`, {
      title: call.title,
      bodyMd: call.bodyMd,
      visibility: "private",
    });

    const afterHide = await anon.get<ResearchCall[]>(`/research/calls?symbol=${S}&limit=100`);
    expect(afterHide.map((c) => c.id)).not.toContain(call.id);
  });
});

test.describe("평가", () => {
  test("자기 콜은 평가할 수 없다", async ({ user }) => {
    const call = await publishPrivateCall(user.api);
    const message = await user.api.expectFailure(
      "POST",
      `/research/calls/${call.id}/ratings`,
      409,
      { clarity: 5, evidence: 5, risk: 5 },
    );
    expect(message).toContain("자기 콜");
  });

  test("조회수는 명시적인 조회 기록으로만 오른다", async ({ user, otherUser }) => {
    const call = await publishPrivateCall(user.api, { visibility: "public" });

    // 상세를 여러 번 읽어도 카운터는 그대로여야 한다 — 안 그러면 리렌더를 센다.
    await otherUser.api.get<ResearchCall>(`/research/calls/${call.id}`);
    await otherUser.api.get<ResearchCall>(`/research/calls/${call.id}`);
    const afterReads = await user.api.get<ResearchCall>(`/research/calls/${call.id}`);
    expect(
      afterReads.viewCount,
      "상세 조회가 카운터를 올리면 조회수는 렌더 횟수가 된다",
    ).toBe(call.viewCount);

    await otherUser.api.post(`/research/calls/${call.id}/view`);
    const afterView = await user.api.get<ResearchCall>(`/research/calls/${call.id}`);
    expect(afterView.viewCount).toBe(call.viewCount + 1);

    // 같은 뷰어가 곧바로 다시 기록해도 30분 창 안에서는 오르지 않는다.
    await otherUser.api.post(`/research/calls/${call.id}/view`);
    const afterDuplicate = await user.api.get<ResearchCall>(`/research/calls/${call.id}`);
    expect(
      afterDuplicate.viewCount,
      "같은 뷰어의 반복 조회가 그대로 세지면 새로고침으로 조회수를 만들 수 있다",
    ).toBe(call.viewCount + 1);
  });
});
