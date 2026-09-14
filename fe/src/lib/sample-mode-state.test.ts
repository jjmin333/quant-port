import { describe, expect, it } from "vitest";
import {
  anyAutoVote,
  resolveSample,
  sampleBase,
  shouldDropOverride,
  type PathScopedFlag,
} from "@/lib/sample-mode-state";

const HOLDINGS = "/holdings";
const on = (pathname: string): PathScopedFlag => ({ pathname, value: true });
const off = (pathname: string): PathScopedFlag => ({ pathname, value: false });

const base = {
  pathname: HOLDINGS,
  fromUrl: false,
  override: null as PathScopedFlag,
  auto: null as PathScopedFlag,
};

describe("resolveSample", () => {
  it("아무것도 없으면 꺼져 있다", () => {
    expect(resolveSample(base)).toBe(false);
  });

  it("자동 켜기만으로 켜진다 — 묻지 않는다 (2026-09-14 요청 4)", () => {
    expect(resolveSample({ ...base, auto: on(HOLDINGS) })).toBe(true);
  });

  it("주소창 파라미터로도 켜진다 — 링크로 온 사람", () => {
    expect(resolveSample({ ...base, fromUrl: true })).toBe(true);
  });

  /*
    이 항목이 이 파일의 이유다. 자동 켜기가 사용자의 「예시 끄기」를 이기면, 그
    버튼은 눌러도 아무 일이 안 일어나는 버튼이 된다 — 예전에 정확히 그 모양의
    버그가 한 번 있었다(끄면 띠만 사라지고 남의 숫자가 화면에 남았다).
  */
  it("사용자가 끈 것이 자동 켜기를 이긴다", () => {
    expect(resolveSample({ ...base, auto: on(HOLDINGS), override: off(HOLDINGS) })).toBe(false);
  });

  it("사용자가 끈 것이 주소창도 이긴다", () => {
    expect(resolveSample({ ...base, fromUrl: true, override: off(HOLDINGS) })).toBe(false);
  });

  it("사용자가 켠 것은 아무것도 없어도 켜진다", () => {
    expect(resolveSample({ ...base, override: on(HOLDINGS) })).toBe(true);
  });

  it("다른 화면에서 켠 자동은 따라오지 않는다", () => {
    expect(resolveSample({ ...base, auto: on("/watchlist") })).toBe(false);
  });

  it("다른 화면에서 끈 것도 따라오지 않는다 — 여기 자동이 켜져 있으면 켜진다", () => {
    expect(resolveSample({ ...base, auto: on(HOLDINGS), override: off("/watchlist") })).toBe(true);
  });
});

describe("sampleBase", () => {
  it("사용자가 고른 값은 보지 않는다", () => {
    expect(sampleBase({ pathname: HOLDINGS, fromUrl: false, auto: on(HOLDINGS) })).toBe(true);
    expect(sampleBase({ pathname: HOLDINGS, fromUrl: true, auto: null })).toBe(true);
    expect(sampleBase({ pathname: HOLDINGS, fromUrl: false, auto: null })).toBe(false);
  });
});

describe("shouldDropOverride", () => {
  it("고른 값이 없으면 버릴 것도 없다", () => {
    expect(shouldDropOverride(base)).toBe(false);
  });

  it("다른 화면으로 떠났으면 버린다", () => {
    expect(shouldDropOverride({ ...base, override: on("/watchlist") })).toBe(true);
  });

  it("주소창이 이미 같은 말을 하면 버린다 — 임무 완료", () => {
    expect(shouldDropOverride({ ...base, fromUrl: true, override: on(HOLDINGS) })).toBe(true);
  });

  /*
    자동 켜기가 생기면서 나올 뻔한 버그 — 「끈 값」을 버리면 그 즉시 자동이 다시
    이겨 예시가 도로 켜진다. 밑바닥을 주소창만으로 보면 `false === false`가 되어
    버려지므로, 자동까지 함께 봐야 갈린다.
  */
  it("자동이 켜져 있는 동안 「끈 값」은 버리지 않는다", () => {
    expect(shouldDropOverride({ ...base, auto: on(HOLDINGS), override: off(HOLDINGS) })).toBe(
      false,
    );
  });

  it("자동이 꺼지면 그때 「끈 값」을 버린다 — 둘이 같은 말을 한다", () => {
    expect(shouldDropOverride({ ...base, override: off(HOLDINGS) })).toBe(true);
  });

  /*
    끄고 → 자동이 꺼져 override가 버려지고 → 다시 자동이 켜지는 흐름. 사용자가
    한 번 껐다는 사실은 이때 없어진다. 그래도 되는 이유는 그 사이에 「내 것이
    생겼다가 다시 0건이 됐다」는 뜻이고, 그건 새 상황이기 때문이다.
  */
  it("켠 값은 자동이 켜져 있으면 버린다 — 둘이 같은 말을 한다", () => {
    expect(shouldDropOverride({ ...base, auto: on(HOLDINGS), override: on(HOLDINGS) })).toBe(true);
  });
});

describe("anyAutoVote", () => {
  /*
    브라우저에서 실제로 잡은 고장이다 (2026-09-14). 한 화면에서 이 질문에 답하는
    곳이 둘인데 값을 한 칸에 덮어쓰고 있었다.

      껍데기(로그인 안 함) → 참  → 예시 켜짐 → 본문 클라이언트가 처음 마운트
      본문(목록 아직 안 옴)  → 거짓 → 예시 꺼짐 → 본문이 사라짐

    콘솔 로그가 정확히 `true → false → false` 순서였고, 화면에는 「예시로
    둘러보기」를 묻는 카드가 그대로 남았다 — 고치려던 바로 그 화면이.
  */
  it("「모른다」가 「없는 걸 안다」를 덮지 못한다", () => {
    const votes = [on(HOLDINGS), off(HOLDINGS)]; // 껍데기: 참 · 본문: 아직 로딩 중
    expect(anyAutoVote(votes, HOLDINGS)).toBe(true);
    // 넣은 순서가 반대여도 같다 — 마운트 순서에 답이 달라지면 안 된다.
    expect(anyAutoVote([off(HOLDINGS), on(HOLDINGS)], HOLDINGS)).toBe(true);
  });

  it("아무도 참이라 하지 않으면 거짓", () => {
    expect(anyAutoVote([off(HOLDINGS), off(HOLDINGS)], HOLDINGS)).toBe(false);
    expect(anyAutoVote([], HOLDINGS)).toBe(false);
    expect(anyAutoVote([null], HOLDINGS)).toBe(false);
  });

  it("다른 화면의 표는 세지 않는다", () => {
    // 모듈 전역이라, 떠난 화면의 표가 남아 있어도 여기에 영향을 주면 안 된다.
    expect(anyAutoVote([on("/watchlist")], HOLDINGS)).toBe(false);
    expect(anyAutoVote([on("/watchlist"), on(HOLDINGS)], HOLDINGS)).toBe(true);
  });

  it("합친 값이 resolveSample 로 이어진다", () => {
    const auto = anyAutoVote([on(HOLDINGS), off(HOLDINGS)], HOLDINGS)
      ? on(HOLDINGS)
      : null;
    expect(resolveSample({ ...base, auto })).toBe(true);
    // 사용자가 끈 것은 그래도 이긴다.
    expect(resolveSample({ ...base, auto, override: off(HOLDINGS) })).toBe(false);
  });
});
