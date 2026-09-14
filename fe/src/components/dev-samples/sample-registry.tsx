"use client";

import type { LayoutLabValue } from "@/lib/layout-lab";

/**
 * 시안 목록 — **2차** (2026-09-06 사용자 피드백 반영).
 *
 * # 1차에서 무엇이 틀렸나
 *
 * 화면을 새로 그린 목업을 냈고, 평가는 「현재보다 훨씬 저 퀄리티」였다. 맞다 —
 * 실제 관심종목 행 하나에는 컨테이너 쿼리로 접히는 칸 아홉, 52주 그라데이션 막대,
 * 점수 배지 톤, 값 플래시가 들어 있는데 목업에는 그중 아무것도 없었다. 비교 대상이
 * 아니라 열화판이었다.
 *
 * # 2차는 실제 화면이다
 *
 * 각 시안은 **실서비스 컴포넌트를 그대로** 그린다(같은 예시 데이터 `?sample=1`).
 * 다른 것은 껍데기 한 겹뿐이라, 눈에 보이는 차이가 정확히 「배치를 바꾼 결과」다.
 * 기능·카드·필드는 하나도 지우지 않았다.
 */

export type SampleVariant = {
  id: string;
  label: string;
  /** 이 시안이 실험대에 넘길 값. 「현재」는 기본값 그대로다. */
  lab: Partial<LayoutLabValue>;
  note: string;
};

export type SamplePage = {
  id: string;
  label: string;
  request: number;
  /** 시안이 띄울 실서비스 경로(예시 데이터 모드). */
  path: string;
  problem: string;
  variants: SampleVariant[];
};

/** 다섯 화면이 공유하는 세 가지 — 레일을 좁은 화면에서 어디에 세우나. */
const RAIL_VARIANTS: SampleVariant[] = [
  { id: "current", label: "현재", lab: {}, note: "레일이 본문 아래로 전부 펼쳐진다" },
  {
    id: "fold",
    label: "A · 접힘",
    lab: { rail: "fold" },
    note: "본문 아래 접힌 한 칸. 열면 지금과 똑같은 것이 나온다",
  },
  {
    id: "sheet",
    label: "B · 시트",
    lab: { rail: "sheet" },
    note: "본문 위 한 줄 → 화면을 덮는 시트. 레일이 처음으로 전체 폭을 쓴다",
  },
];

export const SAMPLE_PAGES: SamplePage[] = [
  /*
    2026-09-07 관심종목 **전면 개편** 시안.

    앞의 여섯 화면과 성격이 다르다 — 저쪽은 「레일을 어디에 세우나」였고 여기는 화면
    자체를 다시 짠다. 그래도 규칙은 같다: 시세·점수·일정은 실서비스와 같은 경로로 받고
    조립도 같은 함수를 쓴다(`buildWatchlistRows`). 넷이 공유하는 몸통은
    `components/watchlist/lab/watchlist-lab-client.tsx` 하나이고, 시안마다 다른 것은
    **무엇을 어디에 놓느냐**뿐이다.
  */
  {
    id: "watchlist-v2",
    label: "관심 종목 개편",
    request: 1,
    path: "/watchlist",
    problem:
      "행 아홉 칸 중 셋(전일·전주·전월)이 같은 종류의 숫자라, 열두 종목이면 등락률 서른여섯 개를 읽고 무엇이 중요한지는 스스로 판단해야 한다. 담은 뒤 수익률은 좁은 화면에서 가장 먼저 접히고, 내가 이 종목에 적어 둔 목표가·메모·스탬프는 화면 어디에도 없다.",
    variants: [
      { id: "current", label: "현재", lab: {}, note: "지금 화면 그대로 — 비교 기준" },
      {
        id: "signal",
        label: "A · 신호 목록",
        lab: {},
        note: "전일·전주·전월 → 최근 흐름 + 신호 칩. 말할 거리가 있는 종목만 두 줄이 된다",
      },
      {
        id: "board",
        label: "B · 카드 보드",
        lab: {},
        note: "한 종목 = 카드 한 장. 목록이 못 담던 것(흐름·목표가·신호)이 다 들어간다",
      },
      {
        id: "brief",
        label: "C · 오늘의 브리핑",
        lab: {},
        note: "A의 행을 「지금 볼 것 / 움직인 것 / 조용한 것」 세 칸으로 나눈다",
      },
      {
        id: "bench",
        label: "D · 워크벤치",
        lab: {},
        note: "왼쪽 목록 · 오른쪽 상세. 모달을 열고 닫지 않고 여럿을 견준다",
      },
    ],
  },
  /*
    2026-09-07 메모장 **전면 개편** 시안.

    관심종목 개편과 성격이 같다 — 레일 위치가 아니라 화면 자체를 다시 짠다. 몸통은
    `components/journal/lab/journal-lab-client.tsx` 하나이고, 그리는 조각은 전부
    실서비스의 것이다(상세 모달·작성 폼·태그 칩·종목 아바타·스파크라인·퀵룩).
    행만 시안용 픽스처를 쓰는 이유는 `components/journal/lab/fixture.ts` 머리말.
  */
  {
    id: "journal-v2",
    label: "메모장 개편",
    request: 2,
    path: "/journal",
    problem:
      "1280px에서 가장 넓은 칸이 들어오자마자 「왼쪽에서 메모를 고르면 여기에 펼쳐집니다」로 비어 있다. 메모장에서 가장 자주 하는 일(쓰기)은 목록 카드 안쪽의 점선 버튼이라 좁은 화면에서는 묻힌다. 그리고 이 앱의 메모가 다른 메모장과 다른 유일한 지점(저장 순간 박제되는 작성 시점 가격)이 행 끝의 작은 % 하나로만 쓰인다.",
    variants: [
      { id: "current", label: "현재", lab: {}, note: "지금 화면 그대로 — 비교 기준" },
      {
        id: "feed",
        label: "A · 기록장",
        lab: {},
        note: "맨 위가 작성 자리, 아래는 월별 연대기. 「답이 오고 있는 메모」 띠가 먼저 선다",
      },
      {
        id: "desk",
        label: "B · 종목 노트북",
        lab: {},
        note: "메모를 종목 아래로 묶는다. 카드 머리에 현재가·최근 흐름·그 종목 글의 평균 성과",
      },
      {
        id: "review",
        label: "C · 복기 보드",
        lab: {},
        note: "성적표가 첫 화면. 크게 맞은 순서 · 태그별 성적 · 아직 답을 안 본 옛 메모",
      },
    ],
  },
  {
    id: "earnings",
    label: "실적 일정",
    request: 4,
    path: "/earnings/us",
    problem:
      "스코어보드 228px + 월 격자 345px = 573px(375×812의 70%)을 쓰고 나서야 첫 회사가 나온다. 사용자 결정: 스코어보드와 아래 상세 표는 유지하고 달력만 눕힌다.",
    variants: [
      { id: "current", label: "현재", lab: {}, note: "월 격자 345px" },
      {
        id: "strip",
        label: "달력만 띠로",
        lab: { calendar: "strip" },
        note: "격자와 같은 것을 말하는 가로 띠(약 100px) · 「달력」으로 원래 격자를 편다",
      },
      {
        id: "strip-fold",
        label: "띠 + 레일 접힘",
        lab: { calendar: "strip", rail: "fold" },
        note: "위의 띠에 「다가오는 대형 발표」 레일 접힘까지 더한 것",
      },
    ],
  },
  {
    id: "watchlist",
    label: "관심 종목",
    request: 5,
    path: "/watchlist",
    problem:
      "본문 540px에 레일 1,587px(카드 7장)이 아래로 쏟아진다. 이 화면의 주 동작인 「종목 추가」가 그 레일 첫 칸이라 목록 전체를 지나야 닿는다.",
    variants: RAIL_VARIANTS,
  },
  {
    id: "stamps",
    label: "스탬프",
    request: 6,
    path: "/stamps",
    problem:
      "본문 1,230px + 레일 1,616px(카드 8장) = 뷰포트 3.5개. 시간이 걸린 「개봉 대기·예정」이 레일 안쪽이라 놓치기 쉽다.",
    variants: RAIL_VARIANTS,
  },
  {
    id: "journal",
    label: "메모장",
    request: 7,
    path: "/journal",
    problem:
      "본문 455px에 레일 812px. 그 레일 첫 칸이 「메모 검색」이라, 메모가 쌓일수록 검색창이 목록 아래로 밀려난다.",
    variants: RAIL_VARIANTS,
  },
  {
    id: "targets",
    label: "목표가",
    request: 8,
    path: "/targets",
    problem:
      "본문 667px + 레일 1,131px. 레일 첫 칸이 「목표가 추가」 폼(331px)이다.",
    variants: RAIL_VARIANTS,
  },
  {
    id: "reports",
    label: "리포트",
    request: 9,
    path: "/reports",
    problem:
      "필터 여덟 개가 첫 화면에서 두 줄 94px을 쓰고, 레일 1,032px(카드 6장)이 본문 1,998px 아래에 붙는다.",
    variants: RAIL_VARIANTS,
  },
];

export function findSample(pageId: string, variantId: string) {
  const page = SAMPLE_PAGES.find((p) => p.id === pageId);
  const variant = page?.variants.find((v) => v.id === variantId);
  return { page, variant };
}
