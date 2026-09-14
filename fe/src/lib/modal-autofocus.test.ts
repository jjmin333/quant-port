import { describe, expect, it } from "vitest";
import { shouldSuppressModalAutoFocus } from "@/lib/modal-autofocus";

const env = (over: Partial<Parameters<typeof shouldSuppressModalAutoFocus>[0]>) => ({
  coarsePointer: false,
  finePointer: true,
  maxTouchPoints: 0,
  ...over,
});

describe("shouldSuppressModalAutoFocus", () => {
  it("폰·태블릿(손가락만)은 막는다", () => {
    expect(
      shouldSuppressModalAutoFocus(env({ coarsePointer: true, finePointer: false, maxTouchPoints: 5 })),
    ).toBe(true);
  });

  it("마우스 데스크톱은 그대로 둔다", () => {
    expect(shouldSuppressModalAutoFocus(env({}))).toBe(false);
  });

  it("터치 스크린 노트북(둘 다 있음)은 데스크톱으로 본다 — 물리 키보드가 있다", () => {
    expect(
      shouldSuppressModalAutoFocus(env({ coarsePointer: true, finePointer: true, maxTouchPoints: 10 })),
    ).toBe(false);
  });

  it("미디어 쿼리를 못 읽어도 터치만 있으면 막는다", () => {
    expect(
      shouldSuppressModalAutoFocus(env({ coarsePointer: false, finePointer: false, maxTouchPoints: 5 })),
    ).toBe(true);
  });

  it("포인터 정보가 아예 없으면 데스크톱으로 본다 — 서버 렌더가 여기 걸린다", () => {
    expect(
      shouldSuppressModalAutoFocus(env({ coarsePointer: false, finePointer: false, maxTouchPoints: 0 })),
    ).toBe(false);
  });
});
