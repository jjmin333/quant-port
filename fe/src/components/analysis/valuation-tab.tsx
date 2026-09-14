"use client";

import { useMemo } from "react";
import { FundamentalsIndicatorsPanel } from "@/components/analysis/fundamentals-section";
import { Card } from "@/components/ui/card";
import type { FundamentalsData } from "@/lib/fundamentals-types";
import { mapFundamentalsToPanel } from "@/lib/fundamentals-mapper";
import type { QuoteDetail, StockInfo } from "@/lib/quotes-types";
import { applyLiveValuationMultiples } from "@/lib/valuation-multiples";

type ValuationTabProps = {
  stock: StockInfo;
  fundamentals: FundamentalsData | null;
  quote: QuoteDetail | null;
  loading: boolean;
};

/**
 * 밸류에이션 탭 — **한 화면이다.**
 *
 * 예전에는 「배수」와 「지표」 두 서브탭이 있었고 기본이 「배수」였다. 세어 보니
 * 배수 화면의 지표 5개 중 4개(PER·PBR·FWD PER·CY PER)가 지표 화면과 그대로
 * 중복이었고, 고유한 것은 PSR 하나뿐이었다 — 그건 지표 쪽으로 옮겼다.
 *
 * 나머지 콘텐츠는 전부 지어낸 값이라 함께 사라졌다: 스파크라인은
 * `buildTrendSeries(value, `${symbol}-${label}`)`로 **심볼 문자열을 해시로 시드 삼은
 * 의사난수** 12점이었고, 그 위에서 5년 중앙값·최소·최대를 구해 「과거 5년 대비
 * Low/Mid/High」를 판정했다. 「산업 대비」의 업종 중앙값은 `med5y * 1.05`였고,
 * 「NTM 추정」은 `value * 0.92`였다. 장식이 아니라 **판정**을 지어낸 숫자에서
 * 뽑고 있었다는 점에서 재무제표 카드의 가짜 추세선보다 나빴다.
 *
 * 진짜 PER·PBR 분기 추이가 필요하면 재무제표 탭의 「PER · PBR (분기말 종가 기준)」
 * 카드에 있다 — 서버가 일봉 × 분기 재무로 계산한 값이다.
 */
export function ValuationTab({
  stock,
  fundamentals,
  quote,
  loading,
}: ValuationTabProps) {
  // 주가만 실시간으로 갈아 끼운다 — 재무는 배치 주기, 주가는 5초라 배수가
  // 하루 종일 어제 값에 묶여 있으면 안 된다.
  const liveFundamentals = useMemo(
    () => applyLiveValuationMultiples(fundamentals, quote),
    [fundamentals, quote],
  );

  const panel = useMemo(
    () => (liveFundamentals ? mapFundamentalsToPanel(liveFundamentals, stock) : null),
    [liveFundamentals, stock],
  );

  if (loading) {
    return (
      <Card className="flex min-h-[200px] items-center justify-center p-6 text-cell text-[var(--text-muted)]">
        밸류에이션 데이터 로딩 중…
      </Card>
    );
  }

  /*
    「재무제표·발행주식수는 실시간이 아니며…」 각주는 뺐다(2026-09-03 사용자 결정 §10).
    갱신 주기는 카드의 `FY…` 표기와 커버리지 등급 배지가 이미 말하고 있고, 그 한
    줄은 모든 종목·모든 방문에 똑같이 붙어 아무도 읽지 않는 자리를 차지하고 있었다.
  */
  return <FundamentalsIndicatorsPanel stock={stock} data={liveFundamentals} panel={panel} />;
}
