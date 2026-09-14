"use client";

import { Card } from "@/components/ui/card";
import {
  SCORE_AXIS_LABELS,
  SCORE_AXIS_ORDER,
  SCORE_TONE_BG as BAR_CLASS,
  SCORE_TONE_TEXT as TEXT_CLASS,
  scoreTone,
  type ScoreAxisId,
  type StockScoreData,
} from "@/lib/stock-score-api";
import { cn } from "@/lib/utils";

function AxisBar({ axis, value }: { axis: ScoreAxisId; value: number | undefined }) {
  const has = typeof value === "number";
  const tone = has ? scoreTone(value) : "mid";
  return (
    <div className="flex items-center gap-2">
      <span className="w-14 shrink-0 text-cell leading-none text-[var(--text-muted)]">
        {SCORE_AXIS_LABELS[axis]}
      </span>
      <div className="h-1.5 min-w-0 flex-1 overflow-hidden rounded-full bg-[var(--surface-elevated)]">
        {has && (
          <div
            className={cn("h-full rounded-full", BAR_CLASS[tone])}
            style={{ width: `${Math.max(2, Math.min(100, value))}%` }}
          />
        )}
      </div>
      <span
        className={cn(
          "w-7 shrink-0 text-right text-cell leading-none tabular-nums",
          has ? TEXT_CLASS[tone] : "text-[var(--text-muted)]",
        )}
      >
        {has ? Math.round(value) : "-"}
      </span>
    </div>
  );
}

/**
 * 종합 점수 카드 — **개요 탭 맨 위**에 선다 (2026-09-03 사용자 결정).
 *
 * # 왜 개요로 옮겼나
 *
 * 예전에는 AI 분석 탭 맨 위에 있었다. 그런데 그 탭의 본문(LLM 글)은 **전체
 * 17,916종목 중 744종목(4%)에만** 있고, 점수는 거의 모든 종목에 있다. 종목을 열고
 * 처음 보이는 화면이 개요인데, 대부분의 종목에서 개요는 「AI 요약 없음 + 차트」로
 * 시작하고 정작 그 종목을 요약하는 유일한 숫자는 한 탭 뒤에 숨어 있었다.
 *
 * # 높이를 어디서 줄였나
 *
 * 개요 탭 최상단은 모바일에서 **차트가 시작되기 전의 마지막 관문**이라 높이가 곧
 * 비용이다. 숫자를 머리줄로 끌어올려 한 층을 통째로 없앴다:
 *
 *   · 점수·등급이 본문 왼쪽 아래에서 **제목 옆**으로 (본문 한 줄 제거)
 *   · 동종 순위·커버리지가 본문 오른쪽 아래에서 **날짜 위**로 (그 줄과 합류)
 *   · 축 다섯 줄의 간격·라벨 폭·글자 높이를 한 단씩 죄었다
 *
 * 좁은 화면에서는 머리줄이 두 줄로 접힌다(`flex-wrap`) — 잘라 내지 않는다. 점수
 * 옆의 「/ 100」처럼 없어도 읽히는 것만 좁은 폭에서 감춘다.
 */
export function StockScoreCard({
  score,
  loading,
  className,
}: {
  score: StockScoreData | null;
  loading?: boolean;
  className?: string;
}) {
  if (loading) {
    return (
      <Card className={cn("px-3 py-2 text-cell text-[var(--text-muted)]", className)}>
        점수 불러오는 중…
      </Card>
    );
  }
  if (!score || score.score == null) return null;

  const tone = scoreTone(score.score);
  const coveragePct = Math.round((score.coverage ?? 0) * 100);
  const thin = coveragePct < 60;

  return (
    <Card className={cn("p-0", className)}>
      <div className="flex flex-wrap items-center gap-x-2 gap-y-1 border-b border-[var(--border)] px-3 py-2">
        <h3 className="shrink-0 text-cell font-semibold text-[var(--text-primary)]">종합 점수</h3>

        {/* 제목과 숫자는 **한 덩어리로** 접힌다 — 등급 배지만 다음 줄에 남으면
            그것이 무엇의 등급인지 알 수 없다. */}
        <div className="flex shrink-0 items-baseline gap-1.5">
          <span className={cn("text-2xl font-bold leading-none tabular-nums", TEXT_CLASS[tone])}>
            {score.score}
          </span>
          {/*
            「/ 100」은 **좁은 화면에서도 남긴다** (2026-09-04 사용자 지적).

            0~100 척도를 아는 사람에게는 잉여라고 보고 `sm` 미만에서 감췄는데,
            그러면 모바일에서 「56」 하나만 서서 등급 배지 옆의 또 다른 숫자처럼
            읽힌다. 오른쪽에 있던 커버리지·기준일을 걷어낸 지금(아래) 머리줄에
            자리가 생겼고, 척도를 밝히는 편이 낫다.
          */}
          <span className="text-micro text-[var(--text-muted)]">/ 100</span>
          {score.grade && (
            <span
              className={cn(
                "rounded-md border border-current px-1.5 py-0.5 text-micro font-semibold leading-none",
                TEXT_CLASS[tone],
              )}
            >
              {score.grade}
            </span>
          )}
        </div>

        {/*
          오른쪽 위에는 **동종 상위 순위만** 남는다 (2026-09-04 사용자 결정).

          여기에는 세 가지가 두 줄로 있었다: 「동종 상위 17% · 커버리지 90%」와
          「서버 산출 · 2026-09-03」. 뒤의 둘은 사용자가 이 카드에서 하는 판단을
          바꾸지 않는다 — 「서버 산출」은 모든 종목·모든 방문에 똑같이 붙는 말이고,
          날짜와 커버리지 퍼센트는 점수를 읽는 데 쓰이지 않는다.

          **커버리지가 정말 낮을 때는 아래 경고가 말한다**(`thin`). 그건 숫자가
          아니라 「점수를 어디까지 믿을지」를 문장으로 말하는 자리라 남는다.
        */}
        {typeof score.peerPct === "number" && (
          <p className="ml-auto min-w-0 shrink truncate text-right text-micro leading-tight text-[var(--text-muted)]">
            동종 상위 {score.peerPct}%
          </p>
        )}
      </div>

      <div className="space-y-1 px-3 py-2">
        {SCORE_AXIS_ORDER.map((axis) => (
          <AxisBar key={axis} axis={axis} value={score.breakdown?.[axis]} />
        ))}

        {/*
          산출 방식 설명("각 지표를 동종 그룹 내 순위 백분위로 환산해 가중 합산합니다…")은
          뺐다(2026-09-03 사용자 결정 §7). 모든 종목·모든 방문에 똑같이 붙는 문장이라
          두 번째 종목부터는 읽히지 않으면서 카드 높이만 늘리고 있었다.

          아래 커버리지 경고는 **남긴다** — 이건 설명이 아니라 이 종목에만 해당하는
          사실이고, 점수를 어디까지 믿을지 바꾸는 정보다.
        */}
        {thin && (
          <p className="!mt-2 rounded-md border border-dashed border-[var(--border)] px-2 py-1 text-micro leading-relaxed text-[var(--text-muted)]">
            사용 가능한 데이터가 {coveragePct}%로 제한적이라 점수를 중립(50) 쪽으로 보정했습니다.
          </p>
        )}
      </div>
    </Card>
  );
}

/**
 * One-axis score for a specific tab (차트=추세, 밸류에이션=가치 …), so each tab
 * carries the part of the score it is actually about.
 */
export function AxisScoreBadge({
  score,
  axis,
  className,
}: {
  score: StockScoreData | null;
  axis: ScoreAxisId;
  className?: string;
}) {
  const value = score?.breakdown?.[axis];
  if (typeof value !== "number") return null;
  const tone = scoreTone(value);

  return (
    <div
      className={cn(
        "flex items-center gap-2 rounded-lg border border-[var(--border)] bg-[var(--surface-elevated)]/40 px-2.5 py-1.5",
        className,
      )}
    >
      <span className="text-cell text-[var(--text-muted)]">
        {SCORE_AXIS_LABELS[axis]} 점수
      </span>
      <div className="h-1.5 w-16 overflow-hidden rounded-full bg-[var(--surface)]">
        <div
          className={cn("h-full rounded-full", BAR_CLASS[tone])}
          style={{ width: `${Math.max(2, Math.min(100, value))}%` }}
        />
      </div>
      <span className={cn("text-cell font-semibold tabular-nums", TEXT_CLASS[tone])}>
        {Math.round(value)}
      </span>
      {typeof score?.score === "number" && (
        <span className="text-micro text-[var(--text-muted)]">(종합 {score.score})</span>
      )}
    </div>
  );
}
