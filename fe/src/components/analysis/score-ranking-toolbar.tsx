"use client";

import {
  RadarChips,
  RadarMarketTabs,
  RadarSegmented,
  RadarToolbarShell,
} from "@/components/market-radar/radar-toolbar-parts";
import {
  SCORE_RANKING_AXIS_OPTIONS,
  SCORE_RANKING_DURATION_OPTIONS,
  SCORE_RANKING_MARKET_OPTIONS,
  scoreRankingCaption,
  type ScoreRankingParams,
} from "@/lib/score-ranking-config";

type ScoreRankingToolbarProps = {
  params: ScoreRankingParams;
  /** 이 목록이 근거로 삼은 점수일. 아직 모르면 빈 문자열. */
  tradeDate: string;
  onChange: (next: ScoreRankingParams) => void;
  compact?: boolean;
  /** 주면 시장 탭 끝에 「국내 ETF」가 붙는다(2026-09-24). 누르면 ETF 점검 목록으로 바뀐다. */
  onEtf?: () => void;
};

/*
AI 추천 랭킹의 필터 줄 — **실시간 차트와 같은 부품**으로 만든다.

시장 탭 · 정렬 기준 세그먼트 · 기간 칩. 생김새는 전부 `radar-toolbar-parts`가 쥐고
있어서 두 화면이 어긋날 자리가 없다(그 파일 머리글에 왜 그렇게 했는지 적혀 있다).

다른 점은 **오른쪽에 시계가 없다**는 것뿐이다. 점수는 하루 한 번 계산되므로 초 단위
시각을 찍을 값이 없고, 대신 「무엇을 잰 순위인가」를 아래 한 줄이 말한다 — 같은
날짜에 축·기간에 따라 여섯 가지 다른 순위가 나오기 때문에 그 문장이 필요하다.
*/
export const ETF_TAB = { id: "etf", label: "국내 ETF" } as const;

export function ScoreRankingToolbar({
  params,
  tradeDate,
  onChange,
  compact,
  onEtf,
}: ScoreRankingToolbarProps) {
  function patch(partial: Partial<ScoreRankingParams>) {
    onChange({ ...params, ...partial });
  }

  return (
    <RadarToolbarShell
      compact={compact}
      tabs={
        <RadarMarketTabs<string>
          options={onEtf ? [...SCORE_RANKING_MARKET_OPTIONS, ETF_TAB] : SCORE_RANKING_MARKET_OPTIONS}
          value={params.market}
          onChange={(market) => {
            if (market === ETF_TAB.id) onEtf?.();
            else patch({ market: market as ScoreRankingParams["market"] });
          }}
          compact={compact}
        />
      }
    >
      {/* 축 여섯 개. 좁은 레일에서는 세그먼트 안에서 가로로 스크롤한다. */}
      <RadarSegmented
        options={SCORE_RANKING_AXIS_OPTIONS}
        value={params.axis}
        onChange={(axis) => patch({ axis })}
        label="점수 축"
      />

      <RadarChips
        options={SCORE_RANKING_DURATION_OPTIONS}
        value={params.duration}
        onChange={(duration) => patch({ duration })}
        label="기간"
      />

      <p
        className="truncate text-micro text-[var(--text-muted)]"
        title="점수는 공개 재무·시세·뉴스로 계산한 참고 지표이며 특정 종목의 매수·매도 권유가 아닙니다. 투자 판단과 책임은 본인에게 있습니다."
      >
        {scoreRankingCaption(params, tradeDate)} · 투자 권유 아님
      </p>
    </RadarToolbarShell>
  );
}
