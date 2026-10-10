"use client";

import { useMemo } from "react";
import { earningsEventKey } from "@/components/earnings/earnings-shared";
import type { EarningsCalendarEvent, EarningsEvent } from "@/lib/earnings-types";
import { formatFinancialAmount } from "@/lib/format";

/**
 * 분기 추이 — **기대와 발표를 겹쳐** 그린다.
 *
 * # 왜 두 막대인가
 *
 * 한 분기의 「기대」와 「발표」를 나란히 두면 이 회사가 습관적으로 상회하는지
 * 하회하는지가 한눈에 보인다. 발표치만 그리면 그냥 실적 추이이고, 그건 종목 상세의
 * 재무 탭이 이미 하는 일이다.
 *
 * # 기준 지표
 *
 * 시장 관행을 따른다 — 국내는 **영업이익**, 해외는 **EPS**. 한 종목 안에서는
 * 한 지표로만 그린다(섞으면 막대 높이가 서로 다른 단위가 된다).
 */
export function EarningsQuarterBars({
  announcements,
  calendar,
  currency,
}: {
  /** 최신 순. 서버가 그렇게 준다. */
  announcements: EarningsEvent[];
  /** 다음 발표(예정·추정)를 오른쪽 점선 칸으로 그리는 데 쓴다. */
  calendar: EarningsCalendarEvent[];
  currency: string;
}) {
  const metric = announcements[0]?.market === "us" ? "eps" : "opIncome";

  const bars = useMemo(() => {
    // 오래된 것부터 왼쪽에. 최근 6분기면 한 화면에 읽히고, 그보다 많으면 막대가
    // 얇아져 기대/발표 두 개를 구별할 수 없다.
    const rows = announcements
      .filter((ev) => value(ev, metric, "actual") != null || value(ev, metric, "estimate") != null)
      .slice(0, 6)
      .reverse();
    return rows.map((ev) => ({
      event: ev,
      label: `${ev.fiscalPeriod === "FY" ? "연간" : ev.fiscalPeriod}${String(ev.fiscalYear).slice(2)}`,
      estimate: value(ev, metric, "estimate"),
      actual: value(ev, metric, "actual"),
    }));
  }, [announcements, metric]);

  const next = useMemo(() => calendar.find((c) => c.kind === "upcoming"), [calendar]);

  if (bars.length === 0) return null;

  // 음수 영업이익(적자)이 있으면 0을 기준선으로 삼아야 막대가 뒤집히지 않는다.
  const values = bars.flatMap((b) => [b.estimate, b.actual]).filter((v): v is number => v != null);
  const max = Math.max(...values, 0);
  const min = Math.min(...values, 0);
  const span = max - min || 1;

  const H = 96;
  const zeroY = (max / span) * H;

  return (
    <div>
      <div className="mb-1 flex items-baseline justify-between">
        <p className="text-cell font-semibold">
          분기 추이 — {metric === "eps" ? "EPS" : "영업이익"}
        </p>
        <div className="flex items-center gap-2 text-micro text-muted-foreground">
          <Legend className="bg-muted-foreground/35" label="기대" />
          <Legend className="bg-[var(--profit)]" label="발표" />
          {next && <Legend dashed label="다음 발표" />}
        </div>
      </div>

      <div className="flex items-end gap-1.5">
        {bars.map((bar, i) => {
          const beat =
            bar.estimate != null && bar.actual != null ? bar.actual >= bar.estimate : true;
          return (
            <div
              key={`${earningsEventKey(bar.event)}|${i}`}
              className="flex min-w-0 flex-1 flex-col items-center gap-1"
            >
              <div className="relative flex h-[96px] w-full items-end justify-center">
                {/* 0 기준선 — 적자 분기가 있을 때만 의미가 있다. */}
                {min < 0 && (
                  <div
                    className="absolute left-0 right-0 border-t border-dashed border-border"
                    style={{ bottom: `${H - zeroY}px` }}
                  />
                )}
                <div className="relative flex h-full w-full items-end justify-center">
                  <Bar
                    value={bar.estimate}
                    max={max}
                    min={min}
                    height={H}
                    className="w-full max-w-[26px] bg-muted-foreground/35"
                    title={`기대 ${fmt(bar.estimate, metric, currency)}`}
                  />
                  <Bar
                    value={bar.actual}
                    max={max}
                    min={min}
                    height={H}
                    className={
                      "absolute w-full max-w-[15px] " +
                      (beat ? "bg-[var(--profit)]" : "bg-[var(--loss)]")
                    }
                    title={`발표 ${fmt(bar.actual, metric, currency)}`}
                  />
                </div>
              </div>
              <p className="truncate font-num text-micro text-muted-foreground">{bar.label}</p>
            </div>
          );
        })}

        {next && (
          <div className="flex min-w-0 flex-1 flex-col items-center gap-1">
            <div className="flex h-[96px] w-full items-end justify-center">
              <div
                className="w-full max-w-[26px] rounded-sm border border-dashed border-[var(--sample)]"
                style={{ height: "56px" }}
                title={`${next.date} ${next.label}${next.estimateBasis ? ` · ${next.estimateBasis}` : ""}`}
              />
            </div>
            <p className="truncate font-num text-micro text-[var(--sample)]">다음</p>
          </div>
        )}
      </div>
    </div>
  );
}

function Bar({
  value,
  max,
  min,
  height,
  className,
  title,
}: {
  value: number | null | undefined;
  max: number;
  min: number;
  height: number;
  className: string;
  title: string;
}) {
  if (value == null) return null;
  const span = max - min || 1;
  // 값이 0에 가까워도 1px은 보이게 — 0px이면 "데이터 없음"과 구별되지 않는다.
  const px = Math.max(1, (Math.abs(value) / span) * height);
  return <div className={`${className} rounded-sm`} style={{ height: `${px}px` }} title={title} />;
}

function Legend({
  className,
  label,
  dashed,
}: {
  className?: string;
  label: string;
  dashed?: boolean;
}) {
  return (
    <span className="flex items-center gap-1">
      <span
        className={
          dashed
            ? "size-2 rounded-sm border border-dashed border-[var(--sample)]"
            : `size-2 rounded-sm ${className}`
        }
      />
      {label}
    </span>
  );
}

function value(
  event: EarningsEvent,
  metric: "eps" | "opIncome",
  side: "estimate" | "actual",
): number | undefined {
  return metric === "eps" ? event.eps?.[side] : event.opIncome?.[side];
}

function fmt(value: number | null | undefined, metric: "eps" | "opIncome", currency: string): string {
  if (value == null) return "—";
  if (metric === "eps") return `${currency === "USD" ? "$" : ""}${value.toFixed(2)}`;
  return formatFinancialAmount(value, currency === "USD" ? "USD" : "KRW");
}
