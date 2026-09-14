"use client";

import {
  RANKING_DURATION_OPTIONS,
  RANKING_TYPE_OPTIONS,
  rankingRefreshHint,
} from "@/lib/market-radar-config";
import {
  RadarChips,
  RadarMarketTabs,
  RadarSegmented,
  RadarToolbarShell,
} from "@/components/market-radar/radar-toolbar-parts";
import {
  formatRankingClock,
  rankingClock,
  rankingClockHint,
} from "@/lib/market-radar-clock";
import type { MarketCountry, MarketRadarParams } from "@/lib/market-radar-types";
import { useMarketDataSettings } from "@/hooks/use-market-data-settings";
import { QUOTE_CHANGE_BASIS_LABEL } from "@/lib/quotes-types";
import { cn } from "@/lib/utils";

type MarketRadarToolbarProps = {
  params: MarketRadarParams;
  rankedAt: string | null;
  refreshedAt?: string | null;
  /** 마지막으로 가격을 받아 온 시각. 장중이면 시계가 이 값을 찍는다. */
  pricesAt?: string | null;
  /**
   * 이 시장이 닫힌 시각(ISO). 마감 중에만 온다 — 그때는 시계가 이 값을 **날짜와
   * 함께** 찍는다(`lib/market-radar-clock`).
   */
  sessionClosedAt?: string | null;
  onChange: (next: MarketRadarParams) => void;
  compact?: boolean;
};

const MARKET_OPTIONS: readonly { id: MarketCountry; label: string }[] = [
  { id: "KR", label: "국내" },
  { id: "US", label: "해외" },
];

/*
시장 · 정렬 · 기간.

# 왜 탭 + 칩인가

**국내와 해외는 필터가 아니라 서로 다른 목록이다.** 탭이 그 사실을 말한다. 그
아래 한 판은 「이 목록을 어떻게 자를까」만 담는다 — 정렬 기준은 둘뿐이라 세그먼트,
기간은 일곱이라 가로 스크롤 칩이다.

예전에는 국내/해외 세그먼트 옆에 **OS 기본 `<select>` 두 개**가 있었다. 앱의 다른
버튼과 높이(32 vs 28px)·모서리·글꼴이 어긋났고, 무엇보다 **가장 많이 쓰는
「실시간/1일/1주」가 펼쳐야만 보였다.**

# 모양은 이 파일이 쥐지 않는다

버튼·탭·칩의 생김새는 `radar-toolbar-parts`에 있고, AI 추천 랭킹의 필터 줄이 같은
것을 쓴다 — 두 화면의 목록이 같아 보여야 한다는 요구가 클래스 복사로는 유지되지
않기 때문이다(그 파일 머리글 참조).

# 안내 문단을 지운 자리

「전일 대비 등락률 · 실시간 거래대금/거래량 순」이라는 문장이 목록 위에 상시로
있었다. 목록을 열 때마다 읽을 문장이 아니다. 다만 **무엇 대비 등락률인지**는
어딘가 남아야 오해가 없으므로, 갱신 시각 칩의 `title`로 넘겼다 — 물었을 때만 답한다.
*/
export function MarketRadarToolbar({
  params,
  rankedAt,
  refreshedAt,
  pricesAt = null,
  sessionClosedAt = null,
  onChange,
  compact,
}: MarketRadarToolbarProps) {
  const { settings } = useMarketDataSettings();

  function patch(partial: Partial<MarketRadarParams>) {
    onChange({ ...params, ...partial });
  }

  /*
    시계는 **화면의 숫자가 만들어진 시각**을 찍는다 — 장중이면 마지막 시세 응답,
    마감이면 **장이 닫힌 시각**이다(`lib/market-radar-clock`). 예전에는 언제나
    스냅샷 시각이라 1초마다 움직이는 목록 위에서 5분째 같은 시각이 서 있었고,
    마감 중에는 서버 폴러가 방금 돈 시각이 「지금 만들어진 값」으로 읽혔다.
  */
  const clock = rankingClock({ pricesAt, sessionClosedAt, refreshedAt, rankedAt });
  const clockLabel = clock ? formatRankingClock(clock.at, clock.basis) : "";

  return (
    <RadarToolbarShell
      compact={compact}
      tabs={
        <RadarMarketTabs
          options={MARKET_OPTIONS}
          value={params.market}
          onChange={(market) => patch({ market })}
          compact={compact}
        />
      }
    >
      <div className="flex min-w-0 items-center justify-between gap-2">
        <RadarSegmented
          options={RANKING_TYPE_OPTIONS}
          value={params.type}
          onChange={(type) => patch({ type })}
          label="정렬 기준"
        />

        {clock && clockLabel && (
          <span
            title={`${QUOTE_CHANGE_BASIS_LABEL} · ${rankingClockHint(
              clock.basis,
              rankingRefreshHint(params.duration, settings.rankingsPollRealtimeMinutes),
            )}`}
            className="flex shrink-0 items-center gap-1 text-micro tabular-nums text-[var(--text-muted)]"
          >
            {/* 초록 점은 「지금 살아 있다」는 뜻이다 — 마감 세션의 스냅샷 시각
                옆에 켜 두면 멈춘 값이 도는 것처럼 보인다. */}
            <span
              aria-hidden="true"
              className={cn(
                "size-1.5 shrink-0 rounded-full",
                clock.basis === "prices" ? "bg-emerald-500" : "bg-[var(--text-muted)]",
              )}
            />
            {clockLabel}
          </span>
        )}
      </div>

      {/* 기간 일곱 개. 좁은 레일에서는 가로로 스크롤한다 — 접으면 가장 많이 쓰는
          셋(실시간·1일·1주)까지 숨는다. */}
      <RadarChips
        options={RANKING_DURATION_OPTIONS}
        value={params.duration}
        onChange={(duration) => patch({ duration })}
        label="기간"
      />
    </RadarToolbarShell>
  );
}
