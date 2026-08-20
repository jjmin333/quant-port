"use client";

import { AddHoldingCard } from "@/components/holdings/add-holding-card";
import { HoldingsNewsCard } from "@/components/holdings/holdings-news-card";
import { HoldingsScheduleCard } from "@/components/holdings/holdings-schedule-card";
import { HoldingsTargetsCard } from "@/components/holdings/holdings-targets-card";
import { ScreenshotImportWizard } from "@/components/holdings/screenshot-import-wizard";
import { IndexPulseCard } from "@/components/market/index-pulse-card";
import { useSymbolQuickLook } from "@/components/stocks/symbol-quick-look-modal";
import type { HoldingMetrics } from "@/lib/holdings-calc";

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
}: {
  /** 비중 높은 순으로 정렬돼 들어온다 — 일정·뉴스가 상위 몇 종목만 조회한다. */
  metrics: HoldingMetrics[];
  onAdded: () => void;
  exchangeRate: number;
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

  return (
    <>
      <AddHoldingCard onAdded={onAdded} exchangeRate={exchangeRate} />
      <ScreenshotImportWizard placement="rail" onCommitted={onAdded} />
      {ranked.length > 0 && <HoldingsScheduleCard symbols={ranked} onPick={quickLook.open} />}
      <HoldingsTargetsCard metrics={metrics} onPick={quickLook.open} />
      {ranked.length > 0 && <HoldingsNewsCard symbols={ranked} />}
      <IndexPulseCard />

      {quickLook.modal}
    </>
  );
}
