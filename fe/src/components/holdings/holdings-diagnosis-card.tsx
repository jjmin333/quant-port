"use client";

import Link from "next/link";
import { useState } from "react";
import { Loader2, Stethoscope } from "lucide-react";

import { Card } from "@/components/ui/card";
import { MiniToggle } from "@/components/ui/mini-toggle";
import {
  formatReturnPct,
  formatScoreDelta,
  signalMark,
  signalReasonLabel,
  trimDate,
  VERDICT_BADGE,
  VERDICT_LABEL,
  type HoldingDiagnosis,
  type HoldingsDiagnosisData,
} from "@/lib/holdings-diagnosis-api";
import { analysisSymbolHref } from "@/lib/analysis-tabs";
import { cn } from "@/lib/utils";

/**
 * 보유 이유 점검 — 보유 요약 바로 아래 (2026-09-18 사용자 결정 A).
 *
 * # 다른 앱이 답하지 않는 질문
 *
 * 증권사 앱은 「얼마 벌었나」를 답한다. 이 카드는 그 다음 질문인 **「왜 아직 들고
 * 있나」**를 답한다 — 살 때의 이유가 지금도 유효한지를 AI 점수 추세 · 실적 서프라이즈 ·
 * 내부자 거래 셋으로 재고, **못 쓴 근거까지 함께 적는다.**
 *
 * # 수익률은 판정에 쓰지 않는다
 *
 * 「올랐으니 이유가 강화됐다」는 결과를 근거로 되돌리는 말이다. 그래서 수익률은 줄
 * **끝**에 대조용으로만 둔다. 실측이 그 차이를 보여 준다 — 삼성전자 +37.6%에 「약화」,
 * 엔비디아 +21.8%에 「유지」.
 *
 * # 간단 / 상세 (2026-09-17 사용자 결정)
 *
 * 「AI 핵심 요약」과 **같은 손잡이**다(`MiniToggle`, 기본 「간단」).
 *
 *   · **간단** — 종목당 한 줄: 이름 · 판정 · 점수 변화 · 근거 수 · 수익률
 *   · **상세** — 종목당 신호 세 줄 전부 + 보유 기간
 *
 * # 모바일이 기준이다
 *
 * 375px에서 다섯 칸은 어느 것도 못 읽는다. 좁은 폭에서는 **근거 수와 수익률을 접고**
 * 이름 · 판정 · 점수 변화만 남긴다 — 접히는 둘은 판정의 근거가 아니라 곁가지다.
 * 상세에서도 신호는 세 줄 고정이라 종목당 높이가 일정하다.
 */

type DetailLevel = "brief" | "full";

const DETAIL_OPTIONS: { id: DetailLevel; label: string }[] = [
  { id: "brief", label: "간단" },
  { id: "full", label: "상세" },
];

export function HoldingsDiagnosisCard({
  data,
  loading,
  error,
}: {
  data: HoldingsDiagnosisData | null;
  loading: boolean;
  error?: string | null;
}) {
  /*
    **판정에서 근거로 돌아갈 길** (2026-09-20, 요구 9).

    「카카오 약화 −46」을 보고도 그 근거로 갈 수 없었다 — 보유 **목록**은 종목을
    열어 주는데 이 카드만 빠져 있었다. 추적(5)에서 근거(2)로 되돌아가는 유일한
    자리라, 그게 없으면 흐름이 거기서 끊긴다.
  */
  const [detail, setDetail] = useState<DetailLevel>("brief");
  const items = data?.items ?? [];

  /*
    보유가 없으면 카드를 세우지 않는다. 0건 화면은 이미 안내 카드가 자리를 잡고 있고,
    거기에 「점검할 것이 없습니다」를 하나 더 얹으면 빈 화면에 빈 카드가 둘이 된다.
  */
  if (!loading && !error && items.length === 0) return null;

  const summary = data?.summary;
  const asOf = items.find((it) => it.lastCloseDate)?.lastCloseDate;

  return (
    <Card className="overflow-hidden p-0">
      <div className="flex flex-wrap items-center gap-x-2 gap-y-1 px-3 pt-2.5 pb-1.5">
        <Stethoscope className="h-3.5 w-3.5 shrink-0 text-[var(--brand)]" aria-hidden />
        <span className="text-cell font-semibold text-[var(--text-primary)]">보유 이유 점검</span>
        {summary ? (
          <span className="text-cell text-[var(--text-muted)]">
            약화 {summary.weaker} · 유지 {summary.steady} · 강화 {summary.stronger}
            {summary.unknown > 0 ? ` · 보류 ${summary.unknown}` : ""}
            {/* 종가 기준임을 적는다 — 실시간 시세가 아니다 (요구 8). */}
            {asOf ? <span className="hidden sm:inline"> · {trimDate(asOf)} 종가</span> : null}
          </span>
        ) : null}
        {/* 토글은 언제나 오른쪽 끝이다 — 목록이 바뀌어도 누르는 자리가 안 움직인다. */}
        <MiniToggle className="ml-auto" options={DETAIL_OPTIONS} value={detail} onChange={setDetail} />
      </div>

      {loading ? (
        <p className="flex items-center gap-1.5 px-3 pb-2.5 text-cell text-[var(--text-muted)]">
          <Loader2 className="h-3 w-3 animate-spin" />
          보유 이유를 점검하는 중입니다…
        </p>
      ) : error ? (
        /* 「불러오지 못했습니다」와 「아직 없습니다」는 다른 상태다 (요구 8). */
        <p className="px-3 pb-2.5 text-cell text-[var(--text-muted)]">{error}</p>
      ) : (
        <>
          <div className="px-3">
            {items.map((item) => (
              <Link
                key={item.symbol}
                href={analysisSymbolHref(item.symbol)}
                className="block rounded-sm hover:bg-[var(--surface-elevated)]"
              >
                {detail === "brief" ? <BriefRow item={item} /> : <FullBlock item={item} />}
              </Link>
            ))}
          </div>
          {data?.note ? (
            <p className="mt-1 border-t border-[var(--border)] px-3 py-2 text-micro text-[var(--text-muted)]">
              {data.note}
            </p>
          ) : null}
        </>
      )}
    </Card>
  );
}

/* ------------------------------------------------------------------------- */

function VerdictBadge({ item }: { item: HoldingDiagnosis }) {
  return (
    <span
      className={cn(
        "shrink-0 rounded-full px-2 py-0.5 text-micro font-bold",
        VERDICT_BADGE[item.verdict],
      )}
    >
      {VERDICT_LABEL[item.verdict]}
    </span>
  );
}

/**
 * 점수 변화 색 — 판정 뱃지와 **같은 초록/주황**이다.
 *
 * 여기에 손익의 빨강·파랑을 쓰면 같은 줄에서 색 하나가 두 가지를 말하게 된다.
 */
function scoreDeltaTone(delta: number | undefined): string {
  if (typeof delta !== "number" || delta === 0) return "text-[var(--text-muted)]";
  return delta > 0 ? "text-[var(--score-high)]" : "text-[var(--score-low)]";
}

function BriefRow({ item }: { item: HoldingDiagnosis }) {
  const delta = item.score?.delta;
  return (
    <div className="flex items-center gap-2 border-b border-[var(--border)] py-2 last:border-b-0">
      <span className="min-w-0 flex-1 truncate text-cell font-semibold text-[var(--text-primary)]">
        {item.name || item.symbol}
        <span className="ml-1.5 font-normal text-[var(--text-muted)]">{item.symbol}</span>
      </span>
      <VerdictBadge item={item} />
      <span
        className={cn(
          "w-11 shrink-0 text-right text-cell font-bold tabular-nums",
          scoreDeltaTone(delta),
        )}
      >
        {formatScoreDelta(delta)}
      </span>
      {/* 좁은 폭에서 접히는 둘 — 판정의 근거가 아니라 곁가지다. */}
      <span className="hidden w-11 shrink-0 text-right text-micro text-[var(--text-muted)] sm:inline">
        근거 {item.signalCount}
      </span>
      <span className="hidden w-16 shrink-0 text-right text-micro tabular-nums text-[var(--text-muted)] sm:inline">
        {formatReturnPct(item.returnPct)}
      </span>
    </div>
  );
}

function FullBlock({ item }: { item: HoldingDiagnosis }) {
  return (
    <div className="border-b border-[var(--border)] py-2.5 last:border-b-0">
      <div className="flex flex-wrap items-center gap-x-2 gap-y-1">
        <span className="text-cell font-bold text-[var(--text-primary)]">
          {item.name || item.symbol}
        </span>
        <VerdictBadge item={item} />
        <span className="ml-auto text-micro tabular-nums text-[var(--text-muted)]">
          {item.firstBought ? `매수 ${trimDate(item.firstBought)}` : "매수 이력 없음"}
          {item.holdingDays ? ` · ${item.holdingDays}일` : ""}
          {" · "}
          {formatReturnPct(item.returnPct)}
        </span>
      </div>

      <div className="mt-1 space-y-0.5">
        {item.signals.map((signal) => {
          const { mark, tone } = signalMark(signal);
          return (
            <div key={signal.id} className="flex items-baseline gap-2 text-cell">
              <span className="w-14 shrink-0 text-micro text-[var(--text-muted)] sm:w-[4.5rem]">
                {signal.label}
              </span>
              <span className={cn("w-3 shrink-0 text-center font-bold", tone)}>{mark}</span>
              <span
                className={cn(
                  "min-w-0 flex-1 tabular-nums",
                  signal.available ? "text-[var(--text-primary)]" : "text-[var(--text-muted)]",
                )}
              >
                {signalReasonLabel(signal)}
                {signal.strong && signal.direction !== 0 ? (
                  <span className="ml-1 text-micro font-bold text-[var(--brand)]">단독 판정</span>
                ) : null}
              </span>
            </div>
          );
        })}
      </div>

      {/*
        매수 시점 점수는 **거의 언제나 없다** — 점수는 30거래일만 보관하고 대부분의
        매수는 그보다 오래됐다. 빈칸으로 두면 「안 변했다」로 읽히므로 사유를 적는다.
      */}
      {typeof item.scoreAtPurchase === "number" ? (
        <p className="mt-1 text-micro text-[var(--text-muted)]">
          매수 시점 점수 {item.scoreAtPurchase}점
        </p>
      ) : null}
    </div>
  );
}
