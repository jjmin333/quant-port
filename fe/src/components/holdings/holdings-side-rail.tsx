"use client";

import { AddHoldingCard } from "@/components/holdings/add-holding-card";
import { SymbolNewsCard } from "@/components/stocks/symbol-news-card";
import { PersonalScheduleCard } from "@/components/earnings/personal-schedule-card";
import { HoldingsTargetsCard } from "@/components/holdings/holdings-targets-card";
import { ScreenshotImportWizard } from "@/components/holdings/screenshot-import-wizard";
import { IndexPulseCard } from "@/components/market/index-pulse-card";
import { useSymbolQuickLook } from "@/components/stocks/symbol-quick-look-modal";
import { useUpcomingEvents } from "@/hooks/use-upcoming-events";
import type { HoldingMetrics } from "@/lib/holdings-calc";

/**
 * 일정을 조회할 종목 수 — **비중 높은 순**으로 이만큼.
 *
 * 요청 수가 아니라(배치 하나다) 응답 크기와 서버가 한 요청에서 도는 종목 수를 묶는다.
 * 비중이 작은 종목의 실적일은 알아도 할 일이 없다.
 */
const SCHEDULE_SYMBOL_LIMIT = 12;

/**
 * 보유종목 화면의 우측 레일.
 *
 * 08-12 개편으로 **본문과 겹치던 카드를 전부 뺐다**:
 *  - 보유 요약(총 평가금액) → 본문 상단 `HoldingsSummaryCard`. 레일은 `xl`(1280px)
 *    아래에서 본문 밑으로 쌓이는데, 이 화면에서 제일 먼저 보는 값이 맨 아래로 내려가면
 *    안 된다.
 *  - 비중 상위 5 → 본문 구성 도넛의 범례가 같은 말을 한다.
 *  - 오늘 많이 움직인 종목(등락률 순) → 본문 "오늘 손익 기여"(금액 순)로 교체. 금액이
 *    같은 질문에 더 쓸모 있는 답을 준다.
 *
 * 남은 자리는 **본문이 답하지 않는 것**으로 채운다 — 종목 추가·스크린샷 가져오기,
 * 앞으로의 일정, 내가 걸어둔 목표가, 보유 종목 뉴스, 시장 지수.
 */
export function HoldingsSideRail({
  metrics,
  onAdded,
  exchangeRate,
  sample = false,
}: {
  /** 비중 높은 순으로 정렬돼 들어온다 — 일정·뉴스가 상위 몇 종목만 조회한다. */
  metrics: HoldingMetrics[];
  onAdded: () => void;
  exchangeRate: number;
  /** 예시 모드에서는 인증이 필요한 카드를 세우지 않는다 — 아래 목표가 카드 참고. */
  sample?: boolean;
}) {
  const ranked = metrics.map((m) => ({
    symbol: m.symbol,
    name: m.name,
    market: (m.currency === "USD" ? "us" : "domestic") as "domestic" | "us",
  }));

  /*
   * 레일 안에서 종목을 누르면 **모달로 연다** — 관심종목 레일·홈과 같은 규칙이다.
   * 예전에는 목표가 행만 `/analysis`로 떠났고 일정 행은 아예 눌리지 않았다.
   * 「종목 분석 보기」·「목표가 관리」는 각각 모달 바닥과 카드 제목 옆에 남아 있다.
   */
  const quickLook = useSymbolQuickLook();

  /*
    일정은 **배치 한 번**으로 받는다 (2026-09-05 사용자 요청 7).

    예전 `HoldingsScheduleCard`는 상위 12종목을 종목당 한 번씩 불렀다 — 보유 화면을
    여는 것만으로 요청 열두 개다. 관심·뉴스 화면은 09-04에 `useUpcomingEvents`(배치
    하나)로 옮겼는데 이 카드만 남아 있었다.
  */
  const { events } = useUpcomingEvents(ranked, SCHEDULE_SYMBOL_LIMIT);

  return (
    <>
      <AddHoldingCard onAdded={onAdded} exchangeRate={exchangeRate} />
      <ScreenshotImportWizard placement="rail" onCommitted={onAdded} />
      {/* 관심 화면·뉴스 레일과 같은 부품 — 2026-09-05 사용자 요청 7 */}
      {ranked.length > 0 && <PersonalScheduleCard events={events} onPick={quickLook.open} />}
      {/*
        목표가 카드는 예시 모드에서 세우지 않는다.

        이 카드가 쓰는 `usePriceTargets()`는 모듈 싱글턴 스토어라(lib/price-targets.ts)
        누가 구독하는 순간 조건 없이 `/user/price-targets`를 읽는다 — 예시 모드도
        세션 유무도 보지 않는다. 그래서 비로그인 예시 화면에서 401이 났고,
        `session-fetch`가 401마다 `/auth/refresh`를 한 번 더 붙여 요청이 두 배가 됐다.

        같은 화면의 평가금액 차트를 감춘 것과 같은 판단이다(아래 본문의
        `{!sample && …}`) — 인증 뒤에 있는 카드를 예시에 세우면 빈 칸만 남는다.
        스토어를 예시 인식으로 바꾸는 쪽은 일부러 피했다: 그 스토어는 「목표가 관리」
        화면과 공유되는데, 거기는 이미 `/sample/price-targets` 픽스처로 따로 가르고
        있어(targets-client.tsx) 스토어에까지 예시 데이터를 담으면 모드를 끈 뒤에도
        예시가 남는, 지금 고치는 것과 **같은 종류의 버그**가 생긴다.
      */}
      {!sample && <HoldingsTargetsCard metrics={metrics} onPick={quickLook.open} />}
      {/* 관심 화면과 같은 부품 — 2026-09-05 사용자 요청 7 */}
      {ranked.length > 0 && (
        <SymbolNewsCard symbols={ranked} title="보유 종목 뉴스" subjectLabel="보유 비중 상위" />
      )}
      <IndexPulseCard />

      {quickLook.modal}
    </>
  );
}
