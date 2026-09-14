"use client";

import Link from "next/link";
import { Card } from "@/components/ui/card";
import { StockAvatar } from "@/components/stocks/stock-avatar";
import {
  ContrarianBadge,
  DirectionBadge,
  HorizonProgress,
  StatusBadge,
  daysUntilDue,
  toneClass,
} from "@/components/research/research-shared";
import { analysisSymbolHref } from "@/lib/analysis-tabs";
import { formatPercent, formatPrice } from "@/lib/format";
import {
  BENCHMARK_LABELS,
  STATUS_LABELS,
  statusTone,
  type ResearchCall,
} from "@/lib/research-types";
import { PriceDisplay } from "@/components/ui/price-display";
import { cn } from "@/lib/utils";

/**
 * 리포트 상세의 **붙박이 계기판** — 글 옆에 서서 스크롤을 따라온다
 * (2026-09-14 사용자 요청 3).
 *
 * # 왜 옆으로 갔나
 *
 * 09-14 오전에 이 값들을 본문 **뒤**로 내렸다. 제목과 첫 문장 사이에 알파·수익률·
 * 진행축·6칸 표가 통째로 서 있어서, 글을 읽으러 들어온 사람이 화면 하나를 지나야
 * 첫 문장에 닿았기 때문이다. 그 진단은 맞았지만 처방에 대가가 있었다 — **읽는
 * 도중에는 목표가를 볼 수 없다.** 「120달러까지 본다」는 문장을 읽으면서 지금
 * 얼마인지 확인하려면 글 끝까지 스크롤해야 한다.
 *
 * 세로로 쌓는 한 그 둘은 못 이긴다. 옆에 두면 둘 다 된다 — 본문은 제목 바로
 * 다음이고, 숫자는 **언제나** 시야에 있다. 사용자가 가리킨 화면도 그 배치다.
 *
 * # 좁은 화면에서는
 *
 * 컬럼이 하나가 되면서 **본문 뒤로 쌓인다** — 09-14 오전에 정한 순서 그대로다.
 * 붙박이는 컬럼으로 서 있을 때만 건다(`page-columns.tsx`의 `RailColumn`과 같은
 * 이유): 쌓인 상태에서 붙이면 본문 위를 덮는다.
 *
 * # 무엇이 여기 오고 무엇이 안 오나
 *
 * 여기 오는 것은 **한 줄로 답할 수 있는 값**이다 — 의견, 목표 수익률, 가격 셋,
 * 기간, 확신도, 진행. 「발행 시점 근거」(5축 막대·PER·52주·뉴스 목록)는 본문
 * 컬럼에 남는다. 읽다가 곁눈으로 확인하는 자료가 아니라 **읽고 나서 따져 보는**
 * 자료이고, 300px 폭에서 그 표들은 읽히지도 않는다.
 */
export function ResearchCallRail({ call }: { call: ResearchCall }) {
  const currency = call.currency ?? "KRW";
  const dueDays = daysUntilDue(call.dueAt, call.status);
  const alpha = call.verdictAlpha ?? call.horizon?.alpha;
  const ret = call.verdictReturn ?? call.horizon?.return ?? call.current?.changeRate;

  /*
    목표 수익률 — 기준가에서 목표가까지 몇 %인가. 작성자가 적은 것은 가격이지만
    사람이 견주는 것은 비율이다(「$128」이 비싼지 싼지는 기준가를 알아야 말할 수
    있다). 관망 의견에는 목표가가 없으므로 이 줄도 없다.
  */
  const targetPct =
    call.targetPrice !== undefined && call.basePrice > 0
      ? ((call.targetPrice - call.basePrice) / call.basePrice) * 100
      : undefined;

  const tone = statusTone(call.status);

  return (
    <div className="flex flex-col gap-3">
      <Card className="overflow-hidden p-0">
        {/*
          머리띠가 **판정 결과**를 말한다. 카드 안의 숫자들은 전부 「얼마나」인데,
          가장 먼저 알고 싶은 것은 「그래서 맞았나」다.
        */}
        <div
          className={cn(
            "flex flex-wrap items-center gap-1.5 px-3 py-2",
            tone === "positive"
              ? "bg-[var(--profit)]/10"
              : tone === "negative"
                ? "bg-[var(--loss)]/10"
                : "bg-[var(--surface-elevated)]",
          )}
        >
          <StatusBadge status={call.status} />
          <DirectionBadge direction={call.direction} />
          {call.contrarian && <ContrarianBadge />}
        </div>

        <div className="space-y-3 p-3">
          <Link
            href={analysisSymbolHref(call.symbol)}
            className="flex items-center gap-2 rounded-lg px-1 py-1 hover:bg-[var(--surface-elevated)]"
          >
            <StockAvatar symbol={call.symbol} name={call.symbolName} size="sm" />
            <span className="min-w-0">
              <span className="block truncate text-sm font-semibold text-[var(--text-primary)]">
                {call.symbolName || call.symbol}
              </span>
              <span className="block font-num text-micro text-[var(--text-muted)]">
                {call.symbol}
              </span>
            </span>
          </Link>

          {/*
            큰 숫자는 **둘뿐**이다 — 주장(목표 수익률)과 결과(지금까지의 수익률).
            셋 이상이 같은 크기로 서면 무엇을 먼저 읽어야 할지 알 수 없다.
          */}
          <div className="grid grid-cols-2 gap-2">
            <BigFigure
              label="목표 수익률"
              value={targetPct === undefined ? "—" : formatPercent(targetPct)}
              tone={targetPct}
            />
            <BigFigure
              label={call.status === "open" ? "현재 수익률" : "달성 수익률"}
              value={ret === undefined ? "—" : formatPercent(ret)}
              tone={ret}
            />
          </div>

          <dl className="space-y-1 text-cell">
            <RailRow
              label="작성일 종가"
              value={<PriceDisplay value={call.basePrice} currency={currency} size="inherit" />}
            />
            {call.current && (
              <RailRow
                label="현재가"
                value={
                  <PriceDisplay value={call.current.price} currency={currency} size="inherit" />
                }
              />
            )}
            {call.targetPrice !== undefined && (
              <RailRow label="목표가" value={formatPrice(call.targetPrice, currency)} />
            )}
            {call.stopPrice !== undefined && (
              <RailRow label="손절가" value={formatPrice(call.stopPrice, currency)} />
            )}
            {call.expectedValue?.riskReward !== undefined && (
              <RailRow
                label="손익비"
                value={`${call.expectedValue.riskReward.toFixed(2)}:1`}
              />
            )}
            <RailRow label="투자 기간" value={`${call.horizonDays}일`} />
            <RailRow label="작성자의 투자 확신도" value={`${call.conviction}%`} />
            <RailRow
              label={`알파 (${BENCHMARK_LABELS[call.benchmarkId ?? ""] ?? call.benchmarkId ?? "—"} 대비)`}
              value={
                <span className={toneClass(alpha)}>
                  {alpha === undefined ? "—" : `${formatPercent(alpha)}p`}
                </span>
              }
            />
          </dl>

          <div className="space-y-1.5 border-t border-[var(--border)] pt-2.5">
            {dueDays !== null && (
              <p className="text-micro text-[var(--text-muted)]">
                판정까지{" "}
                <span className="font-semibold tabular-nums text-[var(--brand)]">D-{dueDays}</span>
              </p>
            )}
            <HorizonProgress
              progress={call.progress}
              publishedAt={call.publishedAt}
              dueAt={call.dueAt}
              status={call.status}
            />
          </div>

          <p className="text-micro leading-relaxed text-[var(--text-muted)]">
            {call.status === "open"
              ? "판정은 종가로만 합니다 — 장중 고가 터치는 목표 달성으로 보지 않습니다."
              : `${STATUS_LABELS[call.status]}${call.verdictDate ? ` · ${call.verdictDate} 종가 기준` : ""}`}
          </p>
        </div>
      </Card>
    </div>
  );
}

function BigFigure({
  label,
  value,
  tone,
}: {
  label: string;
  value: string;
  tone: number | undefined;
}) {
  return (
    <div className="rounded-lg bg-[var(--surface-elevated)] px-2.5 py-2">
      <p className="text-micro text-[var(--text-muted)]">{label}</p>
      <p className={cn("text-lg font-semibold tabular-nums", toneClass(tone))}>{value}</p>
    </div>
  );
}

function RailRow({ label, value }: { label: string; value: React.ReactNode }) {
  return (
    <div className="flex items-baseline justify-between gap-2">
      <dt className="shrink-0 text-[var(--text-muted)]">{label}</dt>
      <dd className="min-w-0 text-right tabular-nums text-[var(--text-secondary)]">{value}</dd>
    </div>
  );
}
