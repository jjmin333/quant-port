"use client";

import { useCallback, useEffect, useState } from "react";
import { cn } from "@/lib/utils";

/**
 * 칩 줄 접기 — **기본은 전부 펼친 상태**이고, 자리를 너무 차지할 때만 접힌다.
 *
 * # 왜 바뀌었나 (2026-09-03 사용자 결정)
 *
 * 예전에는 개수로 접었다(브리핑 6개·키워드 8개). 그래서 칩이 일곱 개인 날에도 접혀
 * 있었다 — 한 줄에 다 들어가는데도 「전체 보기 +2」를 눌러야 나머지가 보였다. 접는
 * 이유는 자리를 아끼는 것인데, 아낄 자리가 없을 때도 접고 있었던 셈이다.
 *
 * 지금은 **그려 놓고 잰다.** 칩을 전부 그린 뒤 실제로 몇 줄이 됐는지 보고, 상한을
 * 넘을 때만 그 높이로 자른다. 넘치지 않으면 아무 일도 일어나지 않고 버튼도 안 뜬다.
 *
 * # 반응형
 *
 * 상한은 줄 수로 정한다 — 좁은 화면 2줄, 640px부터 3줄. 폭이 좁을수록 같은 칩이 더
 * 많은 줄을 먹으므로, 개수가 아니라 **줄 수**로 정해야 화면마다 비슷한 높이가 된다.
 * 줄 높이는 첫 칩에서 재고 줄 간격은 계산된 `row-gap`에서 읽는다 — 부르는 쪽이
 * `gap-1`을 쓰든 `gap-1.5`를 쓰든 맞는다.
 *
 * # 하이드레이션
 *
 * 측정은 **전부 이펙트 안에서** 한다. 서버 렌더와 첫 클라이언트 렌더는 둘 다 「전부
 * 펼친 상태」로 같고, 자르는 일은 마운트 뒤에 일어난다 — 화면 폭을 렌더 중에 읽어
 * 개수를 정하던 방식(`IndexTagChips` 주석 참고)이 피하려던 그 문제가 여기엔 없다.
 *
 * # 버튼은 여기서 그리지 않는다
 *
 * 「전체 보기」는 **섹션 제목 바로 오른쪽**에 선다(2026-09-03 결정). 칩 줄 아래
 * 오른쪽 끝에 두면 눈이 칩을 다 훑은 뒤에야 그것이 있다는 걸 알게 되고, 카드마다
 * 위치가 제각각이 된다. 그래서 이 훅은 상태만 들고, 버튼(`ChipCollapseToggle`)은
 * 부르는 쪽이 제목 옆에 놓는다. 규약은 `docs/UI_SPEC.md`에 적혀 있다.
 */

/** 좁은 화면(<640px)에서 허용할 줄 수. */
const COLLAPSED_ROWS_COMPACT = 2;
/** 640px 이상에서 허용할 줄 수. */
const COLLAPSED_ROWS_WIDE = 3;
/** 위 둘을 가르는 폭 — Tailwind `sm`과 같은 값이다. */
const WIDE_BREAKPOINT_PX = 640;

export type ChipCollapseState = {
  /** 칩 줄에 다는 콜백 ref. `ChipCollapseRow`가 대신 달아 준다. */
  attachRow: (node: HTMLDivElement | null) => void;
  expanded: boolean;
  /** 접었을 때 가려지는 칩 수. 0이면 접을 것이 없다 — 버튼을 그리지 않는다. */
  hiddenCount: number;
  toggle: () => void;
  /** 접힌 높이(px). 넘치지 않거나 펼쳐 두었으면 `undefined`. */
  maxHeight: number | undefined;
};

function collapsedRowCount(fixed?: number): number {
  if (fixed != null) return fixed;
  if (typeof window === "undefined") return COLLAPSED_ROWS_WIDE;
  return window.innerWidth >= WIDE_BREAKPOINT_PX ? COLLAPSED_ROWS_WIDE : COLLAPSED_ROWS_COMPACT;
}

/**
 * 칩 줄의 접힘 상태. `signature`가 바뀌면 다시 잰다 — 목록이 바뀌면 줄 수도 바뀐다.
 *
 * 펼침 상태는 목록이 바뀌어도 유지한다. 시세가 갱신돼 카드가 다시 그려질 때마다
 * 펼쳐 둔 것이 도로 접히면 읽던 자리를 잃는다.
 */
export function useChipCollapse(
  signature: string | number,
  options?: {
    /**
     * 줄 수를 폭과 무관하게 못박는다. 접히지 않고 **한 줄로 자르는** 자리에 쓴다 —
     * 종목 개요의 AI 요약 태그가 그렇다(넘치는 태그는 오른쪽부터 조용히 사라진다).
     */
    rows?: number;
  },
): ChipCollapseState {
  const fixedRows = options?.rows;
  /*
    줄 엘리먼트는 **콜백 ref + state**로 잡는다(`useRef`가 아니라).

    이유가 둘 있다. 하나는 이 훅이 상태 묶음을 통째로 돌려주는데, 그 안에 ref 객체를
    넣으면 `ref={state.attachRow}`가 「렌더 중 ref 접근」으로 잡힌다(react-hooks/refs).
    다른 하나는 이쪽이 더 정확하다는 것이다 — 엘리먼트가 붙는 순간 이펙트가 다시
    돌아, 칩이 나중에 그려지는 카드(로딩 → 데이터)에서도 첫 측정을 놓치지 않는다.
  */
  const [row, setRow] = useState<HTMLDivElement | null>(null);
  const [expanded, setExpanded] = useState(false);
  const [limit, setLimit] = useState<number | null>(null);
  const [hiddenCount, setHiddenCount] = useState(0);

  const measure = useCallback(() => {
    if (!row) return;
    const chips = Array.from(row.children) as HTMLElement[];
    if (chips.length === 0) {
      setLimit(null);
      setHiddenCount(0);
      return;
    }

    const gap = Number.parseFloat(window.getComputedStyle(row).rowGap) || 0;
    const rowHeight = chips[0].offsetHeight;
    if (rowHeight <= 0) return;

    const rows = collapsedRowCount(fixedRows);
    const max = rows * rowHeight + (rows - 1) * gap;

    /*
      잘리는 칩을 **센다**. 접혀 있어도 `overflow: hidden`은 배치를 없애지 않으므로
      가려진 칩의 좌표도 그대로 읽힌다 — 접힌 상태에서도 같은 답이 나온다.
    */
    const top = row.getBoundingClientRect().top;
    let hidden = 0;
    for (const chip of chips) {
      if (chip.getBoundingClientRect().bottom - top > max + 1) hidden += 1;
    }

    setHiddenCount(hidden);
    setLimit(hidden > 0 ? max : null);
  }, [fixedRows, row]);

  useEffect(() => {
    if (!row) return;
    /*
      **두 가지 계기로 잰다.**

      ① 프레임 하나 뒤에 한 번. 배치가 끝난 뒤라야 줄 수를 셀 수 있고, 목록이 바뀌면
         (`signature`) 다시 돌아야 한다 — 접혀 있는 동안 줄의 높이는 상한에 묶여
         있어서 내용이 바뀌어도 아래 관찰자가 깨어나지 않는다.
      ② 칸 폭이 바뀔 때. 창 크기만 보면 사이드 레일 접힘 같은 레이아웃 변화를 놓친다.
    */
    const frame = window.requestAnimationFrame(() => measure());
    const observer =
      typeof ResizeObserver === "undefined" ? null : new ResizeObserver(() => measure());
    observer?.observe(row);
    return () => {
      window.cancelAnimationFrame(frame);
      observer?.disconnect();
    };
  }, [measure, row, signature]);

  return {
    attachRow: setRow,
    expanded,
    hiddenCount,
    toggle: () => setExpanded((v) => !v),
    /*
      첫 렌더에는 `limit`이 아직 `null`이라 상한이 없다 — 서버 렌더와 첫 클라이언트
      렌더가 둘 다 「전부 펼친 상태」로 같고, 자르는 일은 측정이 끝난 다음 렌더에서
      일어난다.
    */
    maxHeight: !expanded && limit != null ? limit : undefined,
  };
}

/** 칩이 사는 줄. 접힘은 높이로만 일어나므로 칩은 언제나 전부 그려진다. */
export function ChipCollapseRow({
  state,
  className,
  title,
  children,
}: {
  state: ChipCollapseState;
  className?: string;
  /** 잘린 칩까지 마우스로 읽히게 하는 툴팁(넘김은 선택). */
  title?: string;
  children: React.ReactNode;
}) {
  // 두 값을 먼저 꺼내 둔다 — `ref={state.…}`와 `style={{…state.…}}`를 같은 객체에서
  // 바로 읽으면 컴파일러가 그 객체를 통째로 ref로 보고 「렌더 중 ref 접근」으로 막는다.
  const { attachRow, maxHeight } = state;
  return (
    <div
      ref={attachRow}
      style={{ maxHeight }}
      title={title}
      className={cn("flex flex-wrap items-center gap-1.5 overflow-hidden", className)}
    >
      {children}
    </div>
  );
}

/**
 * 「전체 보기 +N」 / 「접기」. **섹션 제목 오른쪽**에 둔다.
 *
 * 펼치기 전에 몇 개가 더 있는지 말한다 — 「전체 보기」만으로는 누를 값어치를 알 수
 * 없어 대개 안 누른다.
 */
export function ChipCollapseToggle({
  state,
  className,
}: {
  state: ChipCollapseState;
  className?: string;
}) {
  /*
    넘칠 것이 없으면 버튼도 없다 — 펼쳐 둔 상태에서도 마찬가지다. `hiddenCount`는
    「지금 접으면 몇 개가 가려지나」라서, 창이 넓어져 다 들어가면 0이 된다.
  */
  if (state.hiddenCount === 0) return null;
  return (
    <button
      type="button"
      onClick={state.toggle}
      className={cn(
        "shrink-0 whitespace-nowrap text-cell font-medium text-[var(--brand)] hover:underline",
        className,
      )}
    >
      {state.expanded ? "접기" : `전체 보기 +${state.hiddenCount}`}
    </button>
  );
}
