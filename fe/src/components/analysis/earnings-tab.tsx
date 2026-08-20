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
  periodLabel,
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
            error: e instanceof Error ? e.message : "실적 정보 조회 실패",
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

  return (
    <div className="space-y-2.5">
      <SummarySection fundamentals={fundamentals} currency={currency} />

      {loading ? (
        <Card className="flex min-h-[160px] items-center justify-center p-6 text-xs text-[var(--text-muted)]">
          <Loader2 className="mr-2 h-4 w-4 animate-spin" />
          실적 정보 불러오는 중…
        </Card>
      ) : error ? (
        <Card className="p-4 text-xs text-[var(--loss)]">{error}</Card>
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
        <h3 className="mb-1 text-xs font-semibold text-[var(--text-primary)]">
          서프라이즈 · 컨센서스
        </h3>
        <p className="text-xs leading-relaxed text-[var(--text-muted)]">{consensus.message}</p>
      </Card>
    );
  }

  return (
    <Card className="p-3">
      <div className="mb-2 flex items-baseline justify-between gap-2">
        <h3 className="text-xs font-semibold text-[var(--text-primary)]">
          발표 실적 · 컨센서스 대비
        </h3>
        {consensus?.available && consensus.period && (
          <span className="text-[10px] text-[var(--text-muted)]">최근 {consensus.period}</span>
        )}
      </div>
      <div className="overflow-x-auto">
        <table className="w-full min-w-[460px] text-left text-xs">
          <thead>
            <tr className="border-b border-[var(--border)] text-[var(--text-muted)]">
              <th className="py-1.5 pr-2 font-medium">발표일</th>
              <th className="py-1.5 pr-2 font-medium">기간</th>
              <th className="py-1.5 pr-2 font-medium">기대</th>
              <th className="py-1.5 pr-2 font-medium">발표</th>
              <th className="py-1.5 font-medium">서프라이즈</th>
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
                      className="ml-1 text-[9px] text-[var(--text-muted)]"
                      title={`${ev.revisedAt}에 정정 공시됨 — 수치는 정정본입니다`}
                    >
                      정정
                    </span>
                  )}
                </td>
                <td className="py-1.5 pr-2 text-[var(--text-primary)]">{periodLabel(ev)}</td>
                <td className="py-1.5 pr-2 tabular-nums text-[var(--text-secondary)]">
                  {metricText(ev, "estimate", currency)}
                </td>
                <td className="py-1.5 pr-2 tabular-nums text-[var(--text-primary)]">
                  {metricText(ev, "actual", currency)}
                </td>
                <td className="py-1.5">
                  <SurpriseValue surprise={ev.surprise} className="text-[11px]" />
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      <p className="mt-2 text-[10px] leading-relaxed text-[var(--text-muted)]">
        발표 당일의 <b>잠정치</b>입니다(분기 단독 기준). 아래 「분기별 실적 상세」는 정기보고서
        확정치이며 <b>누적 기준</b>이라 두 표의 숫자는 서로 다릅니다.
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
    const v = event.eps[side];
    return v == null ? "—" : `${currency === "USD" ? "$" : ""}${v.toFixed(2)}`;
  }
  const v = event.opIncome[side];
  return v == null ? "—" : formatFinancialAmount(v, currency);
}

function SummarySection({
  fundamentals,
  currency,
}: {
  fundamentals: FundamentalsData | null;
  currency: QuoteCurrency;
}) {
  if (!fundamentals || fundamentals.coverageTier === "unavailable") {
    return null;
  }
  const items = [
    {
      label: "매출",
      value:
        fundamentals.revenue != null ? formatFinancialAmount(fundamentals.revenue, currency) : "—",
      yoy: fundamentals.revenueGrowthYoy,
    },
    {
      label: "순이익",
      value:
        fundamentals.netIncome != null ? formatFinancialAmount(fundamentals.netIncome, currency) : "—",
      yoy: undefined,
    },
    {
      label: "EPS",
      value: fundamentals.epsBasic != null ? fundamentals.epsBasic.toLocaleString("ko-KR") : "—",
      yoy: fundamentals.epsGrowthYoy,
    },
  ];

  return (
    <Card className="p-3">
      <div className="mb-2 flex items-baseline justify-between">
        <h3 className="text-xs font-semibold text-[var(--text-primary)]">실적 요약</h3>
        {fundamentals.fiscalYear && (
          <span className="text-[10px] text-[var(--text-muted)]">FY{fundamentals.fiscalYear}</span>
        )}
      </div>
      <div className="grid grid-cols-3 gap-2">
        {items.map((item) => (
          <div key={item.label} className="rounded-lg border border-[var(--border)] p-2">
            <p className="text-[10px] text-[var(--text-muted)]">{item.label}</p>
            <p className="mt-0.5 text-xs font-semibold text-[var(--text-primary)]">{item.value}</p>
            {item.yoy != null && (
              <p
                className={
                  "mt-0.5 text-[10px] " +
                  (item.yoy >= 0 ? "text-[var(--profit)]" : "text-[var(--loss)]")
                }
              >
                YoY {formatPercent(item.yoy)}
              </p>
            )}
          </div>
        ))}
      </div>
    </Card>
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
      <h3 className="mb-2 text-xs font-semibold text-[var(--text-primary)]">
        {isDomestic ? "최근 실적 공시" : "실적 발표 일정"}
      </h3>
      {calendar.length === 0 ? (
        <p className="text-xs text-[var(--text-muted)]">
          {isDomestic ? "최근 1년 내 실적 관련 공시가 없습니다." : "예정된 발표 일정이 없습니다."}
        </p>
      ) : (
        <ul className="space-y-1.5">
          {calendar.map((ev, i) => (
            <li key={`${ev.date}-${i}`} className="flex min-w-0 items-start gap-2 text-xs">
              <CalendarClock className="mt-0.5 h-3.5 w-3.5 shrink-0 text-[var(--text-muted)]" />
              <span className="shrink-0 tabular-nums text-[var(--text-secondary)]">{ev.date}</span>
              <span className="min-w-0 flex-1">
                <span className="text-[var(--text-primary)]">{ev.label.trim()}</span>
                {/* 추정 행은 근거를 같은 줄에 적는다 — 「9월 2일 3Q 발표」만 적으면
                    회사가 그날을 공시한 것처럼 보이는데, 국내에는 그런 공시가 없다. */}
                {ev.estimateBasis && (
                  <span className="ml-1 text-[10px] text-[var(--sample)]">
                    ({ev.estimateBasis})
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
                <span className="shrink-0 rounded-full bg-[var(--surface-elevated)] px-1.5 py-0.5 text-[10px] text-[var(--text-muted)]">
                  {ev.kind === "upcoming" ? "발표 예정" : "공시 제출"}
                </span>
              )}
            </li>
          ))}
        </ul>
      )}
      {isDomestic && calendar.length > 0 && (
        <p className="mt-2 text-[10px] leading-relaxed text-[var(--text-muted)]">
          국내는 향후 발표 일정을 제공하는 소스가 없습니다. 앞으로의 일정은{" "}
          <b className="text-[var(--sample)]">작년 같은 분기의 발표일로 추정</b>한 값이고, 나머지는
          이미 제출된 공시 이력입니다.
        </p>
      )}
    </Card>
  );
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
      <h3 className="mb-2 text-xs font-semibold text-[var(--text-primary)]">분기별 실적 상세</h3>
      {quarterly.length === 0 ? (
        <p className="text-xs text-[var(--text-muted)]">분기별 실적 데이터가 아직 없습니다.</p>
      ) : (
        <div className="overflow-x-auto">
          <table className="w-full min-w-[420px] text-left text-xs">
            <thead>
              <tr className="border-b border-[var(--border)] text-[var(--text-muted)]">
                <th className="py-1.5 pr-2 font-medium">기간</th>
                <th className="py-1.5 pr-2 font-medium">매출</th>
                <th className="py-1.5 pr-2 font-medium">영업이익</th>
                <th className="py-1.5 pr-2 font-medium">순이익</th>
                <th className="py-1.5 font-medium">EPS</th>
              </tr>
            </thead>
            <tbody>
              {quarterly.map((q, i) => (
                <tr key={`${q.fiscalYear}-${q.period}-${i}`} className="border-b border-[var(--border)]/60 last:border-0">
                  <td className="py-1.5 pr-2 text-[var(--text-primary)]">
                    {q.fiscalYear} {q.period}
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
                    {q.epsBasic != null ? q.epsBasic.toLocaleString("ko-KR") : "—"}
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
