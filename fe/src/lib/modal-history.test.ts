import { beforeEach, describe, expect, it, vi } from "vitest";
import {
  MODAL_HISTORY_FLAG,
  isModalHistoryEntry,
  leaveModal,
  resetLeaveModalMark,
  shouldUnwindModalEntry,
} from "@/lib/modal-history";

beforeEach(() => {
  resetLeaveModalMark();
});

describe("shouldUnwindModalEntry", () => {
  it("우리가 쌓은 항목이 맨 위면 걷어낸다", () => {
    expect(shouldUnwindModalEntry({ [MODAL_HISTORY_FLAG]: true })).toBe(true);
  });

  it("Next 라우터가 트리를 얹어도 표식은 남는다 — 실측한 상태 객체", () => {
    expect(
      shouldUnwindModalEntry({
        [MODAL_HISTORY_FLAG]: true,
        __NA: true,
        __PRIVATE_NEXTJS_INTERNALS_TREE: { tree: ["", {}], renderedSearch: "" },
      }),
    ).toBe(true);
  });

  it("모달 안에서 다른 화면으로 떠났으면 손대지 않는다 — 항목 2의 고장", () => {
    // useLoginGate가 /login을 push한 뒤의 상태. 표식이 없다.
    expect(
      shouldUnwindModalEntry({
        __NA: true,
        __PRIVATE_NEXTJS_INTERNALS_TREE: { tree: ["", {}], renderedSearch: "" },
      }),
    ).toBe(false);
  });

  it("항목을 쌓은 적 없는 첫 진입(null)은 되돌리지 않는다", () => {
    expect(shouldUnwindModalEntry(null)).toBe(false);
    expect(shouldUnwindModalEntry(undefined)).toBe(false);
  });

  it("표식이 true가 아니면 남의 항목으로 본다", () => {
    expect(shouldUnwindModalEntry({ [MODAL_HISTORY_FLAG]: "yes" })).toBe(false);
    expect(shouldUnwindModalEntry({ [MODAL_HISTORY_FLAG]: false })).toBe(false);
  });

  it("객체가 아닌 상태도 삼킨다 — pushState는 아무 값이나 받는다", () => {
    expect(isModalHistoryEntry("quantPortModal")).toBe(false);
    expect(isModalHistoryEntry(42)).toBe(false);
  });
});

describe("leaveModal", () => {
  it("닫기를 그대로 부른다", () => {
    const close = vi.fn();
    leaveModal(close);
    expect(close).toHaveBeenCalledTimes(1);
  });

  it("표식이 아직 살아 있어도 되돌리지 않는다 — 항목 1의 고장", () => {
    // Next `<Link>`가 아직 히스토리를 만지기 전. 상태만 보면 「우리 것」으로 보인다.
    leaveModal(() => {});
    expect(shouldUnwindModalEntry({ [MODAL_HISTORY_FLAG]: true })).toBe(false);
  });

  it("건너뛰는 것은 **한 번뿐**이다 — 다음 모달은 제 항목을 걷어낸다", () => {
    leaveModal(() => {});
    shouldUnwindModalEntry({ [MODAL_HISTORY_FLAG]: true });
    expect(shouldUnwindModalEntry({ [MODAL_HISTORY_FLAG]: true })).toBe(true);
  });

  it("떠나지 않고 닫은 모달은 영향을 받지 않는다", () => {
    expect(shouldUnwindModalEntry({ [MODAL_HISTORY_FLAG]: true })).toBe(true);
  });
});
