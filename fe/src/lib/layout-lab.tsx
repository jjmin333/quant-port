"use client";

import { createContext, useCallback, useContext, useSyncExternalStore } from "react";

/**
 * **레이아웃 실험대** (2026-09-06 사용자 요청 4~9 · 2차).
 *
 * # 왜 목업이 아니라 이것인가
 *
 * 1차 시안은 화면을 새로 그린 목업이었고, 사용자 평가는 「현재보다 훨씬 저 퀄리티」였다.
 * 맞는 지적이다 — 실제 화면의 행 하나에는 컨테이너 쿼리로 접히는 칸 아홉(`WL_COL`),
 * 52주 그라데이션 막대, 점수 배지 톤, 값 플래시가 들어 있는데 목업은 그중 아무것도
 * 갖고 있지 않았다. **비교 대상이 아니라 열화판을 만든 것이다.**
 *
 * 그래서 방향을 뒤집는다: 시안은 **실제 화면 그대로**(같은 컴포넌트·같은 예시 데이터)
 * 두고, 바꾸는 것은 **껍데기 한 겹**뿐이다. 그러면 사용자가 보는 차이가 정확히
 * 「배치를 바꾼 결과」가 된다.
 *
 * # 무엇을 바꾸는가
 *
 * 실측한 문제는 다섯 화면이 같았다 — `PageColumns`의 레일이 `xl` 미만에서 본문 아래로
 * 통째로 쏟아진다(관심 1,587px · 스탬프 1,616px · 목표가 1,131px · 리포트 1,032px ·
 * 메모장 812px). 그리고 **그 레일 첫 칸이 그 화면의 주 동작**이다(종목 추가 · 메모 검색 ·
 * 목표가 추가 폼 331px). 좁은 화면에서 이 화면의 할 일이 목록 전체 아래에 있다.
 *
 * 세 가지를 견준다. 어느 쪽도 카드를 지우거나 줄이지 않는다 — **레일이 언제 어디에
 * 서 있느냐**만 다르다.
 *
 * - `stack` — 지금. 본문 아래로 전부 펼쳐진다.
 * - `fold` — 본문 아래, **접힌 한 칸**. 열면 지금과 완전히 같은 것이 나온다.
 * - `sheet` — 본문 **위**에 「관련 정보」 한 줄. 누르면 화면을 덮는 시트로 열려
 *   레일이 좁은 화면에서 처음으로 **전체 폭·전체 높이**를 갖는다.
 *
 * `xl` 이상에서는 셋 다 지금과 똑같이 오른쪽 기둥으로 선다 — 사용자가 PC 배치는
 * 만족한다고 했으므로 거기서 달라질 이유가 없다.
 *
 * # 지울 때
 *
 * 확정안이 정해지면 이 파일과 `src/app/dev/samples`, 그리고 `page-columns.tsx`·
 * `earnings-client.tsx`의 `useRailLayout`/`useCalendarLayout` 분기를 함께 걷는다.
 * 기본값은 전부 「지금 그대로」라, 걷어도 화면은 변하지 않는다.
 */

export type RailLayout = "stack" | "fold" | "sheet";

/** 실적 화면의 달력 형태 — 월 격자(지금) / 날짜 띠. */
export type CalendarLayout = "grid" | "strip";

export type LayoutLabValue = {
  rail: RailLayout;
  calendar: CalendarLayout;
};

const DEFAULT: LayoutLabValue = { rail: "stack", calendar: "grid" };

const LayoutLabContext = createContext<LayoutLabValue>(DEFAULT);

/** 실험대 밖(=실서비스)에서는 언제나 「지금 그대로」를 답한다. */
export function useRailLayout(): RailLayout {
  return useContext(LayoutLabContext).rail;
}

export function useCalendarLayout(): CalendarLayout {
  return useContext(LayoutLabContext).calendar;
}

export function LayoutLab({
  value,
  children,
}: {
  value: Partial<LayoutLabValue>;
  children: React.ReactNode;
}) {
  return (
    <LayoutLabContext.Provider value={{ ...DEFAULT, ...value }}>
      {children}
    </LayoutLabContext.Provider>
  );
}

/**
 * 지금 레일이 **기둥으로 서 있는 폭인가**(`xl` = 1280px 이상).
 *
 * 두 배치를 동시에 그려 두고 CSS로 하나를 감추는 방법은 쓰지 않는다 — 레일 컴포넌트는
 * 실적 일정·뉴스 같은 데이터를 스스로 받아 오므로, 두 벌을 그리면 **요청이 두 배로**
 * 나가고 시안이 재는 값이 실제와 달라진다.
 *
 * 서버 스냅샷은 `true`(데스크톱)다. 이 훅은 실험대 안에서만 쓰이고, 그 화면은
 * 프레임 안에서 클라이언트가 곧바로 실제 폭을 덮는다.
 */
export function useIsRailColumn(): boolean {
  const subscribe = useCallback((onChange: () => void) => {
    if (typeof window === "undefined" || !window.matchMedia) return () => {};
    const mq = window.matchMedia("(min-width: 1280px)");
    mq.addEventListener("change", onChange);
    return () => mq.removeEventListener("change", onChange);
  }, []);
  const get = useCallback(() => {
    if (typeof window === "undefined" || !window.matchMedia) return true;
    return window.matchMedia("(min-width: 1280px)").matches;
  }, []);
  return useSyncExternalStore(subscribe, get, () => true);
}
