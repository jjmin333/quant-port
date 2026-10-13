"use client";

import Link from "next/link";
import { ArrowLeftRight } from "lucide-react";
import { Card } from "@/components/ui/card";
import { ScoreRadar } from "@/components/analysis/score-radar";
import { peerGroupLabel, shortDriverLabel, type ScoreChangeSummary } from "@/lib/score-explain-api";
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
/** 국내 우선주 — 6자리 코드의 끝자리가 0이 아니다(서버 랭킹 우주 규칙과 같다). */
function isDomesticPreferred(symbol: string): boolean {
  return /^[0-9][0-9A-Z]{5}$/.test(symbol) && !symbol.endsWith("0");
}

export function StockScoreCard({
  score,
  loading,
  change = null,
  onOpenExplain,
  className,
}: {
  score: StockScoreData | null;
  loading?: boolean;
  /**
   * 전일 대비 변화와 그 최대 원인 (`/stocks/scores/changes`, 종목당 약 260B).
   *
   * 설명 전체(`/score/explain`, 15KB)를 받지 않는다 — 이 카드가 쓰는 것은 두 값뿐이고
   * 나머지는 AI 분석 탭의 패널이 자기가 열릴 때 받는다.
   */
  change?: ScoreChangeSummary | null;
  /** 있으면 머리줄 「전일」 칩이 버튼이 되어 AI 분석 탭(점수 근거)으로 보낸다. */
  onOpenExplain?: () => void;
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
  /*
    「N개 중 M위」 (2026-10-10, A안) — 배치가 inputs에 남긴다(그 전 회차에는 없어 「상위 N%」로 돌아간다).
    그룹 이름도 붙인다: 종합 점수의 비교 집단은 섹터가 아니라 시장 × 시가총액 3분위다.
  */
  const peerRank = typeof score.inputs?.peerRank === "number" ? score.inputs.peerRank : null;
  const peerSize = typeof score.inputs?.peerSize === "number" ? score.inputs.peerSize : null;
  const groupName = peerGroupLabel(score.peerGroup) ?? "동종";
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
          <ScoreDeltaChip change={change} onOpen={onOpenExplain} />
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
            {peerRank && peerSize ? (
              <>
                <b className="font-semibold text-[var(--text-primary)]">
                  {groupName} {peerSize.toLocaleString("ko-KR")}개 중 {peerRank.toLocaleString("ko-KR")}위
                </b>{" "}
                (상위 {score.peerPct}%)
              </>
            ) : (
              <>동종 상위 {score.peerPct}%</>
            )}
          </p>
        )}
      </div>

      <div className="flex items-center gap-3 px-3 py-2">
        <ScoreRadar breakdown={score.breakdown} className="h-40 w-48 shrink-0 sm:h-44 sm:w-52" />
        {/* 다섯 축 막대 — 10-10 A안의 「가장 강한·약한 축」 둘을 이전처럼 다섯으로 되돌렸다(2026-10-13 요청 1). */}
        <div className="min-w-0 flex-1 space-y-2">
          {SCORE_AXIS_ORDER.map((axis) => (
            <AxisBar key={axis} axis={axis} value={score.breakdown?.[axis]} />
          ))}
          <Link
            href={`/analysis?backtest=1&market=${/^[0-9]/.test(score.symbol) ? "domestic" : "overseas"}`}
            className="inline-block pt-0.5 text-micro text-[var(--brand)] hover:underline"
          >
            이 점수의 과거 성적 보기 →
          </Link>
        </div>
      </div>

      <div className="space-y-1 px-3 pb-2">

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
        {isDomesticPreferred(score.symbol) && (
          // 2026-09-24 사용자 결정 — 우선주는 랭킹 우주에서 빠진다. 점수는 보이되 이유를 적는다.
          <p className="!mt-2 text-micro leading-relaxed text-[var(--text-muted)]">
            우선주는 보통주의 재무로 평가돼 가격만 싸 보일 수 있어 AI 랭킹·스크리너 비교에서는 제외됩니다.
          </p>
        )}
      </div>

      {/* 스크리너 입구 (2026-09-28 요청 5) — 이 종목 값으로 조건을 채워 같거나 나은 종목을 찾는다. */}
      <Link
        href={`/screener?like=${encodeURIComponent(score.symbol)}`}
        className="flex items-center justify-center gap-1 border-t border-[var(--border)] px-3 py-1.5 text-cell font-medium text-[var(--brand)] hover:bg-[var(--brand)]/5"
      >
        <ArrowLeftRight className="h-3.5 w-3.5" />
        해당 종목과 유사한 다른 종목 찾기
      </Link>
    </Card>
  );
}

/**
 * 전일 대비 칩 — 머리줄 점수 옆 (2026-10-13 요청 1, 그전엔 카드 아래 「전일 +5 · 추세 변화」 한 줄).
 * 비교할 회차가 없으면 그리지 않는다 — 없는 값을 0으로 때우지 않는다. 원인은 title로, 누르면 점수 근거로.
 */
function ScoreDeltaChip({ change, onOpen }: { change: ScoreChangeSummary | null; onOpen?: () => void }) {
  if (!change?.available || change.delta == null) return null;
  const d = Math.round(change.delta);
  const cause = shortDriverLabel(change.topDriver);
  const cls = cn(
    "rounded-md px-1.5 py-0.5 text-micro font-semibold leading-none tabular-nums",
    d > 0
      ? "bg-[var(--profit)]/10 text-[var(--profit)]"
      : d < 0
        ? "bg-[var(--loss)]/10 text-[var(--loss)]"
        : "bg-[var(--surface-elevated)] text-[var(--text-muted)]",
  );
  const label = `전일 ${d > 0 ? "▲" : d < 0 ? "▼" : ""}${Math.abs(d)}`;
  const title = [change.fromScore != null ? `${change.fromDate} ${change.fromScore} → ${change.tradeDate} ${change.score}` : null, cause]
    .filter(Boolean)
    .join(" · ");
  if (!onOpen) return <span className={cls} title={title}>{label}</span>;
  return (
    <button type="button" onClick={onOpen} title={title} className={cn(cls, "hover:opacity-80")}>
      {label} ›
    </button>
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
