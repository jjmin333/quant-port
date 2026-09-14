"use client";

import { useState } from "react";
import { MajorEarningsCard } from "@/components/earnings/major-earnings-card";
import { MiniToggle } from "@/components/ui/mini-toggle";
import { useEarningsHighlights } from "@/hooks/use-earnings-highlights";
import type { EarningsMarket } from "@/lib/earnings-types";

/**
 * 주요 실적 일정 — 홈에서 **시간축을 가진 유일한 칸**.
 *
 * 나머지 세 칸(점수 랭킹·거래대금·오늘의 키워드)은 전부 "지금"이다. 그래서 실적
 * 발표를 발표 당일 뉴스로 알게 되는 일이 생긴다.
 *
 * **그림과 규칙은 `MajorEarningsCard`에 있다** (2026-09-05 사용자 요청 7). 실적 화면
 * 레일의 「다가오는 대형 발표」가 같은 응답의 같은 칸을 그리고 있어서 한 부품으로
 * 모았다. 이 파일에 남은 것은 **홈에만 있는 것 둘**이다: 시장 토글과, 그 토글이
 * 목록을 통째로 갈아 끼운다는 사실.
 *
 * 시장 토글은 **이 카드만의 것**이다. 옆 칸 거래대금 상위와 심볼을 공유하면 그쪽
 * 국내/해외를 바꾸는 순간 이 목록도 따라가, "국내 거래대금을 보면서 해외 실적을
 * 보는" 조합이 불가능해진다.
 */
export function DashboardEventsCard({ onPick }: { onPick: (symbol: string) => void }) {
  /*
   * 기본값이 해외인 이유: 국내는 예정 일정을 제공하는 소스가 없어 이 카드가 여는
   * 순간 보여줄 수 있는 게 **작년 발표일로 추정한 날짜**뿐이다. 실제 확정 일정이
   * 있는 쪽을 먼저 보여주고, 추정치는 눌러서 보게 둔다.
   */
  const [market, setMarket] = useState<EarningsMarket>("us");
  const highlights = useEarningsHighlights(market);

  return (
    <MajorEarningsCard
      variant="home"
      market={market}
      events={highlights?.big ?? null}
      onPick={(event) => onPick(event.symbol)}
      href={market === "us" ? "/earnings/us" : "/earnings/domestic"}
      action={
        <MiniToggle
          options={[
            { id: "domestic" as const, label: "국내" },
            { id: "us" as const, label: "해외" },
          ]}
          value={market}
          onChange={setMarket}
        />
      }
    />
  );
}
