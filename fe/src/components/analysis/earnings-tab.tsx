"use client";

import { useEffect, useState } from "react";
import { CalendarClock, Loader2 } from "lucide-react";
import { Card } from "@/components/ui/card";
import { fetchEarningsClient } from "@/lib/earnings-api";
import {
  DateStatusBadge,
  SessionBadge,
  SurpriseValue,
  earningsEventKey,
} from "@/components/earnings/earnings-shared";
import type {
  EarningsCalendarEvent,
  EarningsConsensus,
  EarningsEvent,
  EarningsQuarter,
  EarningsTabData,
} from "@/lib/earnings-types";
import type { FundamentalsData } from "@/lib/fundamentals-types";
import { formatFinancialAmount, formatPercent } from "@/lib/format";
import type { QuoteCurrency } from "@/lib/quotes-types";
import { toUserMessage } from "@/lib/error-message";

type EarningsTabProps = {
  symbol: string;
  currency: QuoteCurrency;
  fundamentals: FundamentalsData | null;
};

export function EarningsTab({ symbol, currency, fundamentals }: EarningsTabProps) {
  /*
   * 어느 종목의 결과인지 **함께** 들고 있는다.
   *
   * 예전에는 이펙트 첫 줄에서 `setLoading(true)`·`setError(null)`을 동기로 불렀는데,
   * 그건 종목을 바꿀 때마다 캐스케이드 렌더를 만든다(react-hooks/set-state-in-effect).
   * 심볼 키가 안 맞으면 "아직 안 옴"으로 읽으면 로딩 플래그 자체가 필요 없다 —
   * 이 앱이 다른 화면에서 이미 쓰는 방식이다(use-news-symbols.ts).
   */
  const [loaded, setLoaded] = useState<{
    symbol: string;
    data: EarningsTabData | null;
    error: string | null;
  } | null>(null);

  useEffect(() => {
    let cancelled = false;
    fetchEarningsClient(symbol)
      .then((res) => {
        if (!cancelled) setLoaded({ symbol, data: res, error: null });
      })
      .catch((e) => {
        if (!cancelled) {
          setLoaded({
            symbol,
            data: null,
            error: toUserMessage(e, "실적 정보 조회 실패"),
          });
        }
      });
    return () => {
      cancelled = true;
    };
  }, [symbol]);

  const current = loaded?.symbol === symbol ? loaded : null;
  const data = current?.data ?? null;
  const error = current?.error ?? null;
  const loading = current === null;

  // 가장 최근 발표. 서버가 최신순으로 준다.
  const latest = data?.announcements?.[0] ?? null;

  return (
    <div className="space-y-2.5">
      <SummarySection fundamentals={fundamentals} currency={currency} latest={latest} />

      {loading ? (
        <Card className="flex min-h-[160px] items-center justify-center p-6 text-cell text-[var(--text-muted)]">
          <Loader2 className="mr-2 h-4 w-4 animate-spin" />
          실적 정보 불러오는 중…
        </Card>
      ) : error ? (
        <Card className="p-4 text-cell text-[var(--loss)]">{error}</Card>
      ) : (
        <>
          <CalendarSection calendar={data?.calendar ?? []} assetType={data?.assetType} />
          <AnnouncementsSection
            announcements={data?.announcements ?? []}
            consensus={data?.consensus}
            currency={currency}
          />
          <QuarterlySection quarterly={data?.quarterly ?? []} currency={currency} />
        </>
      )}
    </div>
  );
}

/**
 * 발표 잠정치 + 컨센서스 대비.
 *
 * 아래 「분기별 실적 상세」와 **한 표로 합치지 않는다.** DART 정기보고서는 1분기를
 * 뺀 나머지가 누적(반기보고서 = 상반기 합계)이고, 이 표의 잠정치는 분기 단독이다.
 * 합치면 「2Q 컨센서스(분기)」 옆에 「2Q 실적(반기 누적)」이 놓여 서프라이즈가
 * 두 배 넘게 부풀고, 화면은 그게 왜 틀렸는지 말할 방법이 없다.
 */
function AnnouncementsSection({
  announcements,
  consensus,
  currency,
}: {
  announcements: EarningsEvent[];
  consensus?: EarningsConsensus;
  currency: QuoteCurrency;
}) {
  if (announcements.length === 0) {
    if (!consensus?.message) return null;
    return (
      <Card className="border-dashed p-3">
        <h3 className="mb-1 text-cell font-semibold text-[var(--text-primary)]">
          시장 기대치 대비 실적
        </h3>
        <p className="text-cell leading-relaxed text-[var(--text-muted)]">{consensus.message}</p>
      </Card>
    );
  }

  return (
    <Card className="p-3">
      <div className="mb-2 flex items-baseline justify-between gap-2">
        <h3 className="text-cell font-semibold text-[var(--text-primary)]">
          발표 실적 · 시장 기대치 대비
        </h3>
        {consensus?.available && consensus.period && (
          <span className="text-micro text-[var(--text-muted)]">
            최근 {consensusPeriodLabel(consensus.period)} 기준
          </span>
        )}
      </div>
      <div className="overflow-x-auto">
        <table className="w-full min-w-[500px] text-left text-cell">
          <thead>
            <tr className="border-b border-[var(--border)] text-[var(--text-muted)]">
              <th className="py-1.5 pr-2 font-medium">발표일</th>
              <th className="py-1.5 pr-2 font-medium">기간</th>
              {/* 「기대 / 발표 / 서프라이즈」는 업계 말이다. 초보자에게는 무엇과
                  무엇을 비교한 값인지가 열 이름에서 바로 읽혀야 한다. */}
              <th className="py-1.5 pr-2 font-medium">
                시장 기대치
                <span className="ml-1 font-normal text-[var(--text-muted)]">(증권사 전망)</span>
              </th>
              <th className="py-1.5 pr-2 font-medium">실제 발표</th>
              <th className="py-1.5 font-medium">
                기대 대비
                <span className="ml-1 font-normal text-[var(--text-muted)]">(서프라이즈)</span>
              </th>
            </tr>
          </thead>
          <tbody>
            {announcements.slice(0, 8).map((ev) => (
              <tr
                key={earningsEventKey(ev)}
                className="border-b border-[var(--border)]/60 last:border-0"
              >
                <td className="py-1.5 pr-2 tabular-nums text-[var(--text-secondary)]">
                  {ev.date.slice(2).replace(/-/g, ".")}
                  {/* 날짜는 원 발표일, 수치는 정정본이라는 사실을 밝힌다. */}
                  {ev.revisedAt && (
                    <span
                      className="ml-1 text-micro text-[var(--text-muted)]"
                      title={`${ev.revisedAt}에 정정 공시됨 — 수치는 정정본입니다`}
                    >
                      정정
                    </span>
                  )}
                </td>
                <td className="py-1.5 pr-2 text-[var(--text-primary)]">
                  {announcementPeriodLabel(ev)}
                  {ev.basis && (
                    <span className="ml-1 text-micro text-[var(--text-muted)]">{ev.basis}</span>
                  )}
                </td>
                <td className="py-1.5 pr-2 tabular-nums text-[var(--text-secondary)]">
                  {metricText(ev, "estimate", currency)}
                </td>
                <td className="py-1.5 pr-2 tabular-nums text-[var(--text-primary)]">
                  {metricText(ev, "actual", currency)}
                </td>
                <td className="py-1.5">
                  <SurpriseValue surprise={ev.surprise} className="text-cell" />
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      <p className="mt-2 text-micro leading-relaxed text-[var(--text-muted)]">
        <b>시장 기대치</b>는 증권사들이 발표 전에 내놓은 전망의 평균이고, <b>기대 대비</b>는
        실제 발표가 그보다 얼마나 높았는지(+) 낮았는지(−)입니다. 여기 숫자는 발표 당일의{" "}
        <b>잠정치</b>이며 <b>그 분기만</b>의 값입니다. 아래 「분기별 실적 상세」는 정기보고서
        확정치이고 <b>연초부터 누적</b>이라 두 표의 숫자는 서로 다릅니다.
      </p>
    </Card>
  );
}

/**
 * 서프라이즈 기준 지표의 값. 국내는 영업이익, 해외는 EPS이므로 어느 쪽으로
 * 비교했는지에 맞춰 같은 지표를 보여준다 — 기대는 EPS인데 발표는 영업이익을
 * 적으면 두 칸이 서로 다른 것을 말한다.
 */
function metricText(
  event: EarningsEvent,
  side: "estimate" | "actual",
  currency: QuoteCurrency,
): string {
  const metric = event.surprise?.metric ?? (event.market === "us" ? "eps" : "opIncome");
  if (metric === "eps") {
    const v = event.eps?.[side];
    return v == null ? "—" : `${currency === "USD" ? "$" : ""}${v.toFixed(2)}`;
  }
  const v = event.opIncome?.[side];
  return v == null ? "—" : formatFinancialAmount(v, currency);
}

/**
 * 이 종목이 어느 기간의 숫자인지 한국어로. "FY2025"는 초보자에게 올해인지
 * 작년인지를 말해 주지 않는다 — 실제로는 **2025년 1~12월을 사업보고서로 확정한
 * 값**이고, 그 보고서는 2026년 3월에 나온다. 그래서 연도와 함께 「연간(확정)」과
 * 결산 월을 같이 적는다.
 */
function fundamentalsPeriodLabel(f: FundamentalsData): string | null {
  if (!f.fiscalYear) return null;
  const period = (f.fiscalPeriod ?? "").trim().toUpperCase();
  const q = /^([1-4])Q$/.exec(period);
  if (q) return `${f.fiscalYear}년 ${q[1]}분기`;
  if (period === "반기") return `${f.fiscalYear}년 상반기 누적`;
  return `${f.fiscalYear}년 연간`;
}

/** "2025년 1월~12월" — 결산일이 있으면 그 12개월 구간을 그대로 적는다. */
function fundamentalsRangeLabel(f: FundamentalsData): string | null {
  if (!f.periodEnd) return null;
  const m = /^(\d{4})-(\d{2})-\d{2}$/.exec(f.periodEnd);
  if (!m) return null;
  const endYear = Number(m[1]);
  const endMonth = Number(m[2]);
  const period = (f.fiscalPeriod ?? "").trim().toUpperCase();
  if (/^[1-4]Q$/.test(period)) return `${endYear}년 ${endMonth}월 결산`;
  // 12개월 구간의 시작. 12월 결산이면 같은 해 1월, 아니면 전년도의 다음 달이다
  // (미국 회사는 결산월이 제각각이라 "1월~12월"로 못 박으면 틀린다).
  const startMonth = (endMonth % 12) + 1;
  const startYear = endMonth === 12 ? endYear : endYear - 1;
  return `${startYear}년 ${startMonth}월 ~ ${endYear}년 ${endMonth}월`;
}

/** "2026년 2분기" — 발표 한 건의 기간. `periodLabel`의 초보자용 표기. */
function announcementPeriodLabel(ev: EarningsEvent): string {
  const p = (ev.fiscalPeriod ?? "").trim().toUpperCase();
  const q = /^([1-4])Q$/.exec(p);
  if (q) return `${ev.fiscalYear}년 ${q[1]}분기`;
  if (p === "FY") return `${ev.fiscalYear}년 연간`;
  return `${ev.fiscalYear}년 ${ev.fiscalPeriod}`;
}

/** "2026 2Q" → "2026년 2분기". 서버가 주는 컨센서스 기간 문자열을 그대로 옮긴다. */
function consensusPeriodLabel(period: string): string {
  const m = /^(\d{4})\s*([1-4])Q$/.exec(period.trim());
  return m ? `${m[1]}년 ${m[2]}분기` : period;
}

/** "7월 7일" — 발표일. 연도는 기간 라벨이 이미 말한다. */
function announceDateLabel(date: string): string {
  const m = /^\d{4}-(\d{2})-(\d{2})$/.exec(date);
  return m ? `${Number(m[1])}월 ${Number(m[2])}일 발표` : `${date} 발표`;
}

/*
 * 증감률과 「무엇 대비인지」를 **두 줄로 나눈다.** 한 줄에 두면 모바일의 3열
 * 타일에서 "직전 분 / 기 대비"처럼 단어 가운데가 끊긴다.
 */
function ChangeNote({ value, caption }: { value?: number; caption: string }) {
  if (value == null) return null;
  return (
    <p className="mt-1 leading-tight">
      <span
        className={
          "text-micro font-medium tabular-nums " +
          (value >= 0 ? "text-[var(--profit)]" : "text-[var(--loss)]")
        }
      >
        {formatPercent(value)}
      </span>
      <span className="block whitespace-nowrap text-micro text-[var(--text-muted)]">{caption}</span>
    </p>
  );
}

function MetricTile({
  label,
  value,
  change,
  changeCaption,
}: {
  label: string;
  value: string;
  change?: number;
  changeCaption: string;
}) {
  return (
    <div className="rounded-lg border border-[var(--border)] p-2">
      <p className="text-micro text-[var(--text-muted)]">{label}</p>
      <p className="mt-0.5 text-cell font-semibold tabular-nums text-[var(--text-primary)]">
        {value}
      </p>
      <ChangeNote value={change} caption={changeCaption} />
    </div>
  );
}

/**
 * 실적 요약 — **두 층으로 나눈다.**
 *
 * 예전에는 「FY2025」 한 줄만 달고 연간 확정치를 보여 줬는데, 바로 아래 표에는
 * 2026년 2분기 발표가 있어 「요약이 최신인가?」가 화면만 봐서는 답이 안 나왔다.
 * 둘은 성격이 다르다: 위는 **가장 최근 발표(잠정치·분기 단독)**, 아래는 **가장
 * 최근 확정(사업보고서·1년 합산)**이고, 확정은 늘 몇 달 늦는다. 그래서 층을
 * 갈라 각각 언제 것인지 적는다.
 */
function SummarySection({
  fundamentals,
  currency,
  latest,
}: {
  fundamentals: FundamentalsData | null;
  currency: QuoteCurrency;
  latest: EarningsEvent | null;
}) {
  const hasConfirmed = fundamentals != null && fundamentals.coverageTier !== "unavailable";
  if (!hasConfirmed && !latest) return null;

  const amount = (v?: number) => (v == null ? "—" : formatFinancialAmount(v, currency));

  return (
    <Card className="p-3">
      <h3 className="mb-2 text-cell font-semibold text-[var(--text-primary)]">실적 요약</h3>

      {latest && (
        <section className="mb-3">
          <div className="mb-1.5 flex flex-wrap items-baseline gap-x-2 gap-y-0.5">
            <span className="rounded-full bg-[var(--brand-muted)] px-2 py-0.5 text-micro font-medium text-[var(--brand)]">
              가장 최근 발표
            </span>
            <span className="text-cell font-medium text-[var(--text-primary)]">
              {announcementPeriodLabel(latest)}
            </span>
            <span className="text-micro text-[var(--text-muted)]">
              {announceDateLabel(latest.date)}
            </span>
          </div>
          <div className="grid grid-cols-3 gap-2">
            <MetricTile
              label="매출"
              value={amount(latest.revenue?.actual)}
              change={latest.revenue?.qoq}
              changeCaption="직전 분기 대비"
            />
            <MetricTile
              label="영업이익"
              value={amount(latest.opIncome?.actual)}
              change={latest.opIncome?.qoq}
              changeCaption="직전 분기 대비"
            />
            <MetricTile
              label="순이익"
              value={amount(latest.netIncome?.actual)}
              change={latest.netIncome?.qoq}
              changeCaption="직전 분기 대비"
            />
          </div>
        </section>
      )}

      {hasConfirmed && fundamentals && (
        <section>
          <div className="mb-1.5 flex flex-wrap items-baseline gap-x-2 gap-y-0.5">
            <span className="rounded-full bg-[var(--surface-elevated)] px-2 py-0.5 text-micro font-medium text-[var(--text-secondary)]">
              확정 실적
            </span>
            <span className="text-cell font-medium text-[var(--text-primary)]">
              {fundamentalsPeriodLabel(fundamentals) ?? "최근 사업보고서"}
            </span>
            {fundamentalsRangeLabel(fundamentals) && (
              <span className="text-micro text-[var(--text-muted)]">
                {fundamentalsRangeLabel(fundamentals)}
              </span>
            )}
          </div>
          <div className="grid grid-cols-3 gap-2">
            <MetricTile
              label="매출"
              value={amount(fundamentals.revenue)}
              change={fundamentals.revenueGrowthYoy}
              changeCaption="작년 대비"
            />
            <MetricTile
              label="순이익"
              value={amount(fundamentals.netIncome)}
              changeCaption="작년 대비"
            />
            <MetricTile
              label="주당순이익 (EPS)"
              value={
                fundamentals.epsBasic != null
                  ? `${fundamentals.epsBasic.toLocaleString("ko-KR")}${currency === "USD" ? "" : "원"}`
                  : "—"
              }
              change={fundamentals.epsGrowthYoy}
              changeCaption="작년 대비"
            />
          </div>
        </section>
      )}

      {latest && hasConfirmed && (
        <p className="mt-2 text-micro leading-relaxed text-[var(--text-muted)]">
          <b>확정 실적</b>은 사업보고서로 확정된 숫자라 <b>가장 최근 발표</b>보다 몇 달 늦게
          나옵니다. 두 층의 기간이 다르므로 숫자를 그대로 비교하지 마세요. 「작년 대비」는 1년 전
          같은 기간과 견준 값(YoY), 「직전 분기 대비」는 바로 앞 분기와 견준 값(QoQ)입니다.
        </p>
      )}
    </Card>
  );
}

/**
 * "2026 3Q 실적발표" → "2026년 3분기 실적발표".
 *
 * 서버 라벨은 실적 일정 화면들과 공유하므로 손대지 않고, 이 탭에서만 옮긴다 —
 * 「3Q」는 이 화면의 대상(주식 입문자)에게 설명 없이 통하는 말이 아니다.
 */
function koreanEventLabel(label: string): string {
  return (
    label
      .trim()
      // 연도가 붙은 형태를 먼저 처리한다 — 뒤 규칙이 "2026 3Q"의 "3Q"만 먼저
      // 집으면 "2026 3분기"라는 어중간한 표기가 남는다.
      .replace(/(\d{4})\s*([1-4])Q\b/g, "$1년 $2분기")
      .replace(/(\d{4})\s*FY\b/g, "$1년 연간")
      .replace(/\b([1-4])Q\b/g, "$1분기")
  );
}

export function CalendarSection({
  calendar,
  assetType,
}: {
  calendar: EarningsCalendarEvent[];
  assetType?: string;
}) {
  // Domestic (DART) only ever has "filed" history; US (Finnhub) only ever has
  // an "upcoming" forward window — branch on assetType, not on array contents,
  // so an empty US calendar doesn't get mislabeled with DART's 공시 copy.
  const isDomestic = assetType === "domestic";
  return (
    <Card className="p-3">
      <h3 className="mb-2 text-cell font-semibold text-[var(--text-primary)]">
        {isDomestic ? "최근 실적 공시" : "실적 발표 일정"}
      </h3>
      {calendar.length === 0 ? (
        <p className="text-cell text-[var(--text-muted)]">
          {isDomestic ? "최근 1년 내 실적 관련 공시가 없습니다." : "예정된 발표 일정이 없습니다."}
        </p>
      ) : (
        <ul className="space-y-1.5">
          {calendar.map((ev, i) => (
            <li key={`${ev.date}-${i}`} className="flex min-w-0 items-start gap-2 text-cell">
              <CalendarClock className="mt-0.5 h-3.5 w-3.5 shrink-0 text-[var(--text-muted)]" />
              <span className="shrink-0 tabular-nums text-[var(--text-secondary)]">{ev.date}</span>
              <span className="min-w-0 flex-1">
                <span className="text-[var(--text-primary)]">{koreanEventLabel(ev.label)}</span>
                {/* 추정 행은 근거를 같은 줄에 적는다 — 「9월 2일 3Q 발표」만 적으면
                    회사가 그날을 공시한 것처럼 보이는데, 국내에는 그런 공시가 없다. */}
                {ev.estimateBasis && (
                  <span className="ml-1 text-micro text-[var(--sample)]">
                    ({koreanEventLabel(ev.estimateBasis)})
                  </span>
                )}
              </span>
              <SessionBadge session={ev.session} />
              {/* 확정·예정·추정 셋을 한 배지로 합치지 않는다. 예전에는 `kind`만 보고
                  「발표 예정」/「공시 제출」 둘로 갈랐는데, 그러면 우리가 역산한
                  추정일이 소스가 준 예정일과 똑같이 보인다. */}
              {ev.dateStatus ? (
                <DateStatusBadge status={ev.dateStatus} />
              ) : (
                <span className="shrink-0 rounded-full bg-[var(--surface-elevated)] px-1.5 py-0.5 text-micro text-[var(--text-muted)]">
                  {ev.kind === "upcoming" ? "발표 예정" : "공시 제출"}
                </span>
              )}
            </li>
          ))}
        </ul>
      )}
      {isDomestic && calendar.length > 0 && (
        <p className="mt-2 text-micro leading-relaxed text-[var(--text-muted)]">
          국내는 향후 발표 일정을 제공하는 소스가 없습니다. 앞으로의 일정은{" "}
          <b className="text-[var(--sample)]">작년 같은 분기의 발표일로 추정</b>한 값이고, 나머지는
          이미 제출된 공시 이력입니다.
        </p>
      )}
    </Card>
  );
}

/**
 * "2026년 상반기 (1~6월 누적)" — DART 정기보고서의 기간 표기.
 * `period`는 "1Q"·"반기"·"3Q"·"연간"으로 오는데, 그중 「반기」와 「3Q」가 누적임을
 * 화면이 말하지 않으면 바로 위 분기 단독 표와 같은 것으로 읽힌다.
 */
function quarterPeriodLabel(q: EarningsQuarter): string {
  const p = (q.period ?? "").trim();
  switch (p.toUpperCase()) {
    case "1Q":
      return `${q.fiscalYear}년 1분기 (1~3월)`;
    case "반기":
    case "2Q":
      return `${q.fiscalYear}년 상반기 (1~6월 누적)`;
    case "3Q":
      return `${q.fiscalYear}년 3분기까지 (1~9월 누적)`;
    case "FY":
    case "연간":
      return `${q.fiscalYear}년 연간 (1~12월)`;
    default:
      return `${q.fiscalYear}년 ${p}`;
  }
}

function QuarterlySection({
  quarterly,
  currency,
}: {
  quarterly: EarningsQuarter[];
  currency: QuoteCurrency;
}) {
  return (
    <Card className="p-3">
      <div className="mb-2">
        <h3 className="text-cell font-semibold text-[var(--text-primary)]">분기별 실적 상세</h3>
        <p className="text-micro text-[var(--text-muted)]">
          정기보고서 확정치 · 그 해 <b>1월부터 누적</b>한 값입니다
        </p>
      </div>
      {quarterly.length === 0 ? (
        <p className="text-cell text-[var(--text-muted)]">분기별 실적 데이터가 아직 없습니다.</p>
      ) : (
        <div className="overflow-x-auto">
          <table className="w-full min-w-[440px] text-left text-cell">
            <thead>
              <tr className="border-b border-[var(--border)] text-[var(--text-muted)]">
                <th className="py-1.5 pr-2 font-medium">기간</th>
                <th className="py-1.5 pr-2 font-medium">매출</th>
                <th className="py-1.5 pr-2 font-medium">영업이익</th>
                <th className="py-1.5 pr-2 font-medium">순이익</th>
                <th className="py-1.5 font-medium">
                  EPS
                  <span className="ml-1 font-normal text-[var(--text-muted)]">(주당순이익)</span>
                </th>
              </tr>
            </thead>
            <tbody>
              {quarterly.map((q, i) => (
                <tr key={`${q.fiscalYear}-${q.period}-${i}`} className="border-b border-[var(--border)]/60 last:border-0">
                  <td className="py-1.5 pr-2 text-[var(--text-primary)]">
                    {quarterPeriodLabel(q)}
                  </td>
                  <td className="py-1.5 pr-2 tabular-nums text-[var(--text-secondary)]">
                    {formatFinancialAmount(q.revenue, currency)}
                  </td>
                  <td className="py-1.5 pr-2 tabular-nums text-[var(--text-secondary)]">
                    {formatFinancialAmount(q.operatingIncome, currency)}
                  </td>
                  <td className="py-1.5 pr-2 tabular-nums text-[var(--text-secondary)]">
                    {formatFinancialAmount(q.netIncome, currency)}
                  </td>
                  <td className="py-1.5 tabular-nums text-[var(--text-secondary)]">
                    {q.epsBasic != null
                      ? `${q.epsBasic.toLocaleString("ko-KR")}${currency === "USD" ? "" : "원"}`
                      : "—"}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </Card>
  );
}

// ConsensusSection은 삭제했다. 「무료 데이터 소스에서는 컨센서스·서프라이즈를
// 제공하지 않습니다」라는 안내만 싣던 카드인데, 그 문장이 사실이 아니었다 —
// 국내 FnGuide가 추정치와 발표치를 함께 주고, Finnhub 응답에도 EPS 추정·실적이
// 처음부터 실려 오는데 파서가 버리고 있었다. 이제 AnnouncementsSection이 실제
// 비교를 보여주고, 정말로 컨센서스가 없는 종목에서만 그 안내를 낸다.
