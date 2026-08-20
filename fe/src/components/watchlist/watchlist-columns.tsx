"use client";

import { cn } from "@/lib/utils";

/**
 * 관심종목 목록의 칸 규격 — 헤더 줄과 데이터 행이 **같은 상수**를 본다.
 *
 * 08-14 A안: 라벨을 행마다 인쇄하지 않고 목록 맨 위 한 줄로 올렸다. 그러면 헤더와 행의
 * 칸 폭이 1px이라도 어긋나는 순간 표가 아니라 그냥 어긋난 두 줄이 된다. 폭을 여기 한
 * 곳에만 적어 두 쪽이 갈라질 수 없게 한다.
 *
 * **칸은 전부 늘어난다**(고정 폭 아님). 처음에는 모든 칸을 고정 폭으로 잡았는데, 그러면
 * 넓은 화면에서 남는 폭이 전부 현재가 오른쪽에 몰려 카드의 40%가 빈 채로 남았다. 지금은
 * `flex-[n]` 비율로 나눠 갖고 `min-w`가 바닥을 잡는다 — 어느 폭에서든 카드를 다 쓴다.
 * 헤더도 같은 클래스를 쓰므로 늘어나도 함께 늘어난다.
 *
 * 브레이크포인트가 `@`(컨테이너)인 이유: 목록이 놓이는 본문 컬럼 폭은 화면 폭을 따라가지
 * 않는다 — 1280px 화면에서도 사이드바와 우측 레일 300px을 빼면 600px대다.
 *
 * 좁아질수록 지워지는 순서(덜 중요한 것부터): 전월(@5xl) → 전주(@4xl) → 담은 뒤·드래그
 * 핸들(@3xl) → 전일(@2xl). 어느 칸이 사라져도 행을 누르면 퀵룩에 전부 있다.
 */
export const WL_COL = {
  grip: "hidden w-6 shrink-0 @3xl:block",
  name: "min-w-0 flex-[2.4] @2xl:flex-[1.7]",
  score: "w-9 shrink-0",
  range: "min-w-[3.25rem] flex-[1.1] @2xl:min-w-[9.5rem] @2xl:flex-[1.9]",
  prevDay: "hidden min-w-[3.75rem] flex-1 text-right @2xl:block",
  prevWeek: "hidden min-w-[3.75rem] flex-1 text-right @4xl:block",
  prevMonth: "hidden min-w-[3.75rem] flex-1 text-right @5xl:block",
  since: "hidden min-w-[4.5rem] flex-1 text-right @3xl:block",
  price: "min-w-[6rem] flex-[1.3] text-right @2xl:min-w-[7rem]",
  remove: "w-7 shrink-0",
} as const;

/** 행 안쪽(드래그 핸들과 삭제 사이) 공통 flex. 헤더도 같은 간격을 쓴다. */
export const WL_ROW_INNER = "flex min-w-0 flex-1 items-stretch gap-1.5 @2xl:gap-3";
export const WL_ROW_OUTER = "flex items-center gap-1.5 @2xl:gap-3";

/**
 * 값 한 칸의 세로 크기 — 52주 블록이 가장 크고, 나머지는 그 안에서 가운데 선다.
 *
 * 가운데 정렬을 `flex justify-center`가 아니라 **패딩**으로 하는 이유: 칸 상수에는
 * 접힘을 위해 `@2xl:block` 같은 display가 들어 있고, 그게 컴포넌트가 붙인 `flex`를
 * 이겨서 `justify-center`가 조용히 무효가 된다(실제로 전일·전주·전월·담은 뒤만 값이
 * 6px 위로 붙어 있었다). 패딩은 display가 무엇이든 같게 동작한다.
 * 44 = 6(패딩) + 19(값) + 13(보조) + 6(패딩).
 */
export const WL_CELL_H = "h-11";
const WL_CELL_PAD = "py-[6px]";

/**
 * 값 한 칸 — **값 줄과 보조 줄의 높이를 고정**한다.
 *
 * 어떤 칸은 보조 줄이 있고(담은 뒤의 날짜, 현재가의 등락) 어떤 칸은 없다. 높이를 내용에
 * 맡기면 행이 세로 가운데를 맞추면서 1줄짜리 칸의 값이 2줄짜리 칸의 값보다 몇 px 아래로
 * 내려간다 — 표처럼 보이려고 헤더를 세웠는데 정작 숫자들이 계단이 된다. 보조 줄이 없어도
 * 자리를 비워 둬서 모든 값이 같은 선 위에 온다.
 */
export function WatchlistCell({
  className,
  value,
  sub,
  valueClassName,
  subClassName,
  title,
}: {
  className?: string;
  value: React.ReactNode;
  sub?: React.ReactNode;
  valueClassName?: string;
  subClassName?: string;
  title?: string;
}) {
  return (
    <span className={cn(WL_CELL_H, WL_CELL_PAD, className)} title={title}>
      <span
        className={cn(
          "block h-[19px] truncate text-[13px] font-semibold leading-[19px] tabular-nums",
          valueClassName,
        )}
      >
        {value}
      </span>
      <span
        className={cn(
          "block h-[13px] truncate text-[10px] leading-[13px] text-[var(--text-muted)]",
          subClassName,
        )}
      >
        {sub ?? " "}
      </span>
    </span>
  );
}

/**
 * 목록 헤더 한 줄.
 *
 * 스크롤을 따라 붙지 않는다 — 이 화면의 스크롤 주인은 `PageScrollShell`이고 목록 위에는
 * 알림·툴바가 있어서, 헤더만 붙여 두면 그 사이에 끼어 떠 있는 줄이 된다.
 */
/**
 * 점수 배지 칸 — 배지도 다른 칸의 **값 줄과 같은 19px 자리**에 앉는다.
 *
 * 배지를 칸 한가운데에 두면 22px짜리가 44px의 정중앙에 서서 옆 숫자들보다 5px 아래로
 * 내려간다. 표에서는 그 5px이 보인다.
 */
export function WatchlistScoreCell({
  className,
  children,
}: {
  className?: string;
  children: React.ReactNode;
}) {
  return (
    <span className={cn(WL_CELL_H, WL_CELL_PAD, className)}>
      <span className="flex h-[19px] items-center justify-center">{children}</span>
      <span className="block h-[13px]" />
    </span>
  );
}

export function WatchlistListHeader() {
  return (
    <div
      className={cn(
        WL_ROW_OUTER,
        // 좌우 13px = 행의 `px-3`(12px) + 카드 테두리 1px. 12px로 두면 헤더 라벨이
        // 행의 값보다 정확히 1px 왼쪽에 서고, 표에서는 그 1px이 보인다.
        "border-b px-[13px] pb-2 text-[10px] font-semibold tracking-wide text-[var(--text-muted)]",
      )}
      aria-hidden
    >
      <span className={WL_COL.grip} />
      <span className={WL_ROW_INNER}>
        <span className={WL_COL.name}>종목</span>
        <span className={cn(WL_COL.score, "text-center")}>점수</span>
        <span className={WL_COL.range}>
          52주<span className="hidden @2xl:inline"> 위치</span>
        </span>
        <span className={WL_COL.prevDay}>전일</span>
        <span className={WL_COL.prevWeek}>전주</span>
        <span className={WL_COL.prevMonth}>전월</span>
        <span className={WL_COL.since}>담은 뒤</span>
        <span className={WL_COL.price}>현재가</span>
      </span>
      <span className={WL_COL.remove} />
    </div>
  );
}
