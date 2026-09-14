"use client";

import { useMemo, useState } from "react";
import { PageColumns } from "@/components/layout/page-columns";
import { PageActions, PageScrollShell } from "@/components/layout/page-content-shell";
import { Card } from "@/components/ui/card";
import { MiniToggle } from "@/components/ui/mini-toggle";
import { SectionEmpty, SectionSkeleton } from "@/components/ui/section-card";
import { EarningsDayStrip } from "@/components/earnings/earnings-day-strip";
import { EarningsMonthGrid } from "@/components/earnings/earnings-month-grid";
import { EarningsWeekBoard } from "@/components/earnings/earnings-week-board";
import { EarningsSideRail } from "@/components/earnings/earnings-side-rail";
import { EarningsSurpriseView } from "@/components/earnings/earnings-surprise-view";
import { EarningsScoreboard } from "@/components/earnings/earnings-scoreboard";
import { EarningsDetailModal } from "@/components/earnings/earnings-detail-modal";
import { EarningsTable } from "@/components/earnings/earnings-table";
import {
  MONTH_ROW_LIMIT,
  useEarningsEvents,
  useEarningsQuotes,
  useEarningsStats,
  useEarningsSummary,
  useMyEarningsSymbols,
} from "@/hooks/use-earnings-calendar";
import {
  currentMonthStart,
  formatDayLabel,
  formatShortDate,
  monthEnd,
  monthStartOf,
  rangeFor,
  shiftDay,
  todayKST,
  weekEndOf,
  weekStartOf,
} from "@/lib/earnings-range";
import { isEarningsRangeEmpty } from "@/lib/earnings-empty";
import { setEarningsRangeMode, useEarningsRangeMode } from "@/lib/earnings-view";
import { useCalendarLayout } from "@/lib/layout-lab";
import { buttonVariants } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import type {
  EarningsEvent,
  EarningsDaySummary,
  EarningsMarket,
  EarningsRangeMode,
} from "@/lib/earnings-types";

/**
 * 실적 일정 — 국내·해외 **각각의** 화면.
 *
 * # 시장을 나눈 이유
 *
 * 두 시장은 같은 목록에 들어갈 수 없다. 국내는 실적발표 예정일을 주는 소스가 없어
 * 앞일이 전부 **추정**이고 지표는 **영업이익**(연결/개별 구분)이다. 해외는 예정일이
 * 정확하고 **발표 시각**(장전/장후)이 있으며 지표는 **EPS**다. 섞으면 한쪽은 늘
 * 칸이 비어 고장처럼 보인다.
 *
 * # 지켜야 하는 한 가지
 *
 * **확정·예정·추정을 절대 같은 모양으로 그리지 않는다.** 국내 미래 일정은 전부
 * 우리가 작년 발표일로 역산한 추정이고, 그것을 회사가 공시한 날짜처럼 보이게
 * 만드는 것이 이 기능에서 유일하게 하면 안 되는 일이다.
 */

const VIEWS: { id: "calendar" | "surprise"; label: string }[] = [
  { id: "calendar", label: "캘린더" },
  { id: "surprise", label: "서프라이즈" },
];

const MODES: { id: EarningsRangeMode; label: string }[] = [
  { id: "month", label: "월" },
  { id: "week", label: "주" },
  { id: "day", label: "일" },
];

/**
 * 데이터가 있는 날 중 **오늘에 가장 가까운 과거** 날짜.
 *
 * 과거를 먼저 보는 이유: 실적 화면에서 오늘이 비어 있다면 직전 발표가 가장 최근
 * 소식이다. 과거에 아무것도 없으면(달을 앞으로 넘긴 경우) 미래의 첫 날을 쓴다.
 */
function latestDayWithEvents(days: EarningsDaySummary[], today: string): string | null {
  const filled = days.filter((d) => d.confirmed + d.scheduled + d.estimated > 0);
  if (filled.length === 0) return null;
  const past = filled.filter((d) => d.date <= today);
  if (past.length > 0) return past[past.length - 1].date;
  return filled[0].date;
}


const COPY: Record<EarningsMarket, { title: string; subtitle: string; source: React.ReactNode }> = {
  domestic: {
    title: "국내 실적 일정",
    subtitle: "코스피·코스닥 잠정실적 발표와 컨센서스 대비 결과를 한 화면에서 봅니다",
    source: (
      <>
        자료: <b>FnGuide</b> 잠정실적발표(컨센서스 추정치 · 발표치) · 주가 반응은 우리 일별 종가.
        국내는 실적발표 예정일을 제공하는 소스가 없어, 앞으로의 일정은{" "}
        <b className="text-[var(--sample)]">작년 같은 분기의 발표일로 추정</b>한 값입니다. 실제
        발표가 확인되면 추정은 사라집니다. 서프라이즈는 <b>영업이익</b> 기준이며, 연결·개별이
        모두 있으면 <b>연결</b>을 대표로 보여줍니다.
      </>
    ),
  },
  us: {
    title: "해외 실적 일정",
    subtitle: "미국 상장 종목의 발표 일정과 EPS 컨센서스 대비 결과를 봅니다",
    source: (
      <>
        자료: <b>Finnhub</b> 실적 캘린더(EPS·매출 컨센서스와 실적, 발표 시각) · 주가 반응은 우리
        일별 종가. 날짜는 <b>한국 시간 기준</b>입니다 — 장후(amc)·장중(dmh) 발표는 미국 거래일
        다음날 새벽(한국)에 해당합니다. 서프라이즈는 <b>EPS</b> 기준입니다 — 발표치는 회계
        기준(GAAP) EPS인데 컨센서스는 조정 EPS라, 수백 %대 차이는 대개 일회성 항목입니다.
        지난날은 실적이 확인된 행만 보여 줍니다. Finnhub가 숫자를 안 준 「예정」은 해당
        날짜가 지나면 목록에서 빼며, 나중에 실적이 오면 다시 나타납니다.
      </>
    ),
  },
};

export function EarningsClient({ market }: { market: EarningsMarket }) {
  const [view, setView] = useState<"calendar" | "surprise">("calendar");
  /*
    달력을 띠로 눕힐지 — 실서비스 기본은 `grid`(지금 그대로)이고, 시안 화면만
    `strip`을 넘긴다(`lib/layout-lab`). `gridOpen`은 그 띠에서 「달력」을 눌러
    격자를 잠깐 편 상태다.
  */
  const calendarLayout = useCalendarLayout();
  const [gridOpen, setGridOpen] = useState(false);
  // 보기 단위는 사람마다 고정된 습관이라 기억한다. `useSyncExternalStore`를 쓰는
  // 이유는 §earnings-view.ts 주석 참고(캐스케이드 렌더·하이드레이션 어긋남 회피).
  const mode = useEarningsRangeMode();
  const [month, setMonth] = useState(currentMonthStart);
  const [week, setWeek] = useState(() => weekStartOf(todayKST()));
  /*
   * 선택 날짜의 기본은 **오늘**이다. 달 전체를 기본으로 두면 실적 시즌에 3,089행
   * (gzip 136KB)이 첫 화면에서 내려오고 DOM 3천 줄이 된다. 캘린더에 들어온 사람이
   * 먼저 묻는 것도 「오늘 누가 발표하나」다 — 달의 지형은 위 히트맵이 말한다.
   */
  const [selectedDate, setSelectedDate] = useState<string | null>(() => todayKST());
  /*
   * 사용자가 날짜를 직접 건드렸는지.
   *
   * 건드리기 전에는 **데이터가 있는 날로 자동 이동**한다. 국내 실적은 2·5·8·11월에
   * 몰려서(실측: 8월 국내 190건이 3~14일에 전부) 「오늘」이 1년 중 대부분 비어 있고,
   * 그러면 이 화면에 처음 들어온 사람이 빈 목록과 「—」로 채운 스코어보드를 본다.
   * 한 번 고른 뒤에는 절대 덮지 않는다 — 사용자가 고른 빈 날짜를 우리가 옮기면
   * 클릭이 안 먹는 것처럼 보인다.
   */
  const [dateTouched, setDateTouched] = useState(false);
  const [mineOnly, setMineOnly] = useState(false);
  const [openEvent, setOpenEvent] = useState<EarningsEvent | null>(null);

  const pickDate = (date: string | null) => {
    setDateTouched(true);
    setSelectedDate(date);
  };

  const changeMode = (next: EarningsRangeMode) => {
    setEarningsRangeMode(next);
    // 「일」로 바꿀 때 고른 날짜가 없으면 오늘로 — 빈 화면을 보여줄 이유가 없다.
    if (next === "day" && !selectedDate) pickDate(todayKST());
  };

  const { symbols: mySymbols, available: mineAvailable } = useMyEarningsSymbols();
  const filterSymbols = mineOnly && mineAvailable ? mySymbols : null;

  // 히트맵/주간 보드는 그 단위 전체를 본다. 상세는 고른 날짜(없으면 단위 전체).
  const summaryRange = useMemo(
    () =>
      mode === "week"
        ? { from: week, to: weekEndOf(week) }
        : { from: month, to: monthEnd(month) },
    [mode, week, month],
  );

  const { days } = useEarningsSummary(summaryRange.from, summaryRange.to, market);

  /*
   * 건드리기 전의 기본 날짜를 **데이터가 있는 날**로 옮긴다.
   *
   * 이펙트+setState로 하지 않는 이유는 그게 캐스케이드 렌더이고, 렌더 중에 파생
   * 값으로 구하면 같은 결과를 공짜로 얻는다. `days`가 오기 전에는 오늘을 그대로
   * 쓰므로 첫 페인트가 흔들리지 않는다.
   */
  const autoDate = useMemo(() => {
    if (dateTouched || !days) return null;
    return latestDayWithEvents(days, todayKST());
  }, [dateTouched, days]);

  // 「일」 보기는 선택 날짜 자체가 구간이라 별도의 날짜 선택이 없다.
  const resolvedSelected = dateTouched ? selectedDate : (autoDate ?? selectedDate);
  const effectiveSelected = mode === "day" ? (resolvedSelected ?? todayKST()) : resolvedSelected;

  const detailRange = useMemo(() => {
    if (mode === "day") {
      return rangeFor("day", { month, week, day: effectiveSelected ?? todayKST() });
    }
    if (effectiveSelected) return { from: effectiveSelected, to: effectiveSelected };
    return summaryRange;
  }, [mode, month, week, effectiveSelected, summaryRange]);

  const { events, loading } = useEarningsEvents(
    detailRange.from,
    detailRange.to,
    market,
    filterSymbols,
    MONTH_ROW_LIMIT,
  );

  // 주간 보드의 「대표 종목」은 그 주 전체를 봐야 나온다 — 고른 날짜와 별개다.
  const { events: weekEvents } = useEarningsEvents(
    week,
    weekEndOf(week),
    market,
    null,
    MONTH_ROW_LIMIT,
    mode === "week",
  );

  const { stats } = useEarningsStats(detailRange.from, detailRange.to, market);

  /*
   * 레일의 「내 종목 최근 발표」 — **지난 2주 ~ 앞으로 한 달**.
   *
   * 본문이 보고 있는 달과 무관하게 고정 창을 쓴다. 이 카드가 답하는 것은
   * 「내 종목이 최근 뭘 발표했고 다음은 언제인가」이고, 그건 사용자가 달력을
   * 어디로 넘겼든 같은 질문이다. 서버는 심볼 필터가 있으면 구간 상한을 넓게
   * 잡아 준다(31일 → 120일).
   */
  const myRange = useMemo(
    () => ({ from: shiftDay(todayKST(), -14), to: shiftDay(todayKST(), 30) }),
    [],
  );
  const { events: mineEvents } = useEarningsEvents(
    myRange.from,
    myRange.to,
    market,
    mySymbols,
    MONTH_ROW_LIMIT,
    mySymbols.length > 0,
  );

  // 표의 현재가 열 — 보이는 행의 심볼을 모아 배치로 **한 번** 부른다.
  const quotes = useEarningsQuotes(events);

  const rangeTotal = useMemo(
    () =>
      effectiveSelected || mineOnly || !days
        ? null
        : days.reduce((sum, d) => sum + d.confirmed + d.scheduled + d.estimated, 0),
    [days, effectiveSelected, mineOnly],
  );
  const truncated = rangeTotal != null && events != null && rangeTotal > events.length;

  const rangeLabel = effectiveSelected
    ? formatDayLabel(effectiveSelected)
    : mode === "week"
      ? `${formatShortDate(week)} – ${formatShortDate(weekEndOf(week))}`
      : `${month.slice(0, 4)}. ${month.slice(5, 7)} 전체`;

  /*
   * 이 달에 한 건도 없는가 — 스코어보드를 감출지와 달력에 문구를 낼지를 **같은 값**으로
   * 정한다(`lib/earnings-empty`). 「월」 보기에서만 본다: 주·일 보기는 구간이 짧아
   * 비어 있는 것이 정상이고, 그 경우는 아래 목록 카드가 이미 말한다.
   */
  const monthIsEmpty = mode === "month" && isEarningsRangeEmpty(days);

  const copy = COPY[market];

  return (
    <>
      <PageScrollShell>
        <PageActions>
          {view === "calendar" && (
            <MiniToggle options={MODES} value={mode} onChange={changeMode} />
          )}
          <MiniToggle options={VIEWS} value={view} onChange={setView} />
        </PageActions>
        <PageColumns
          rail={
            <EarningsSideRail
              market={market}
              mySymbols={mySymbols}
              myEvents={mineEvents}
              onPickEvent={setOpenEvent}
            />
          }
        >
          {view === "surprise" ? (
            <EarningsSurpriseView market={market} onPickEvent={setOpenEvent} />
          ) : (
            <div className="space-y-3">
              {/*
                # 빈 달에는 **스코어보드를 세우지 않는다** (2026-09-05 사용자 지적 8)

                셀 다섯이 「0건」과 「—」로 채워진 카드는 아무 말도 하지 않으면서 화면
                맨 위를 차지한다. 할 말(「9월에는 일정이 없습니다」)은 바로 아래 달력
                카드가 정중앙에서 한 번만 한다 — 같은 사실을 두 카드가 나눠 말하면
                어느 쪽이 답인지 흐려진다.
              */}
              {!monthIsEmpty && <EarningsScoreboard stats={stats} rangeLabel={rangeLabel} />}

              {mode === "month" &&
                (calendarLayout === "strip" && !gridOpen ? (
                  /*
                    # 달력을 눕힌 보기 (2026-09-06 사용자 선택 · 실험대)

                    스코어보드와 아래 상세 표는 그대로 두고 **격자만** 띠로 바꾼다.
                    375px 실측에서 상단이 573px(뷰포트의 70%)이라 첫 회사가 화면
                    밖이었고, 사용자가 유지하기로 한 둘 중 줄일 수 있는 것이 이것뿐이다.
                    띠는 격자와 같은 것을 말한다 — `earnings-day-strip` 머리말 참조.
                  */
                  <EarningsDayStrip
                    month={month}
                    days={days}
                    selected={resolvedSelected}
                    loading={days === null}
                    market={market}
                    onSelect={pickDate}
                    onMonthChange={(next) => {
                      setMonth(next);
                      pickDate(null);
                    }}
                    onExpand={() => setGridOpen(true)}
                  />
                ) : (
                  <EarningsMonthGrid
                    month={month}
                    days={days}
                    selected={resolvedSelected}
                    loading={days === null}
                    market={market}
                    onSelect={(date) => {
                      pickDate(date);
                      // 띠에서 펼친 격자는 날짜를 고르면 도로 접는다 — 고른 뒤에도
                      // 격자가 남아 있으면 띠로 바꾼 이유(세로 예산)가 사라진다.
                      if (calendarLayout === "strip") setGridOpen(false);
                    }}
                    onMonthChange={(next) => {
                      setMonth(next);
                      // 다른 달에서 특정 날짜를 고를 근거가 없다 — 히트맵을 보고 고르게 한다.
                      pickDate(null);
                    }}
                  />
                ))}
              {mode === "week" && (
                <EarningsWeekBoard
                  weekStart={week}
                  days={days}
                  events={weekEvents}
                  selected={resolvedSelected}
                  onSelect={pickDate}
                  onWeekChange={(next) => {
                    setWeek(next);
                    pickDate(null);
                  }}
                />
              )}
              {mode === "day" && (
                <DayNavigator
                  date={effectiveSelected ?? todayKST()}
                  onChange={(next) => {
                    pickDate(next);
                    setMonth(monthStartOf(next));
                    setWeek(weekStartOf(next));
                  }}
                />
              )}

              <Card className="p-3">
                <div className="mb-2 flex flex-wrap items-center justify-between gap-2">
                  <h2 className="text-section font-semibold text-foreground">
                    {rangeLabel}
                    <span className="ml-1.5 font-normal text-muted-foreground">
                      {events
                        ? truncated
                          ? `${rangeTotal}건 중 ${events.length}건`
                          : `${events.length}건`
                        : ""}
                    </span>
                  </h2>
                  <MineToggle on={mineOnly} available={mineAvailable} onChange={setMineOnly} />
                </div>

                {loading ? (
                  <SectionSkeleton rows={8} />
                ) : !events || events.length === 0 ? (
                  <SectionEmpty
                    text={
                      mineOnly
                        ? "이 기간에 내 종목 일정이 없습니다"
                        : "이 기간에 실적 발표가 없습니다"
                    }
                  />
                ) : (
                  <div className="space-y-3">
                    {/*
                      해외는 하루를 장전·장중·장후로 나눠 보는 게 훨씬 잘 읽힌다 —
                      국내에는 발표 시각 데이터가 아예 없어 이 표현이 불가능하다.
                    */}
                    <EarningsTable
                      events={events}
                      market={market}
                      quotes={quotes}
                      onPickEvent={setOpenEvent}
                    />
                  </div>
                )}

                {truncated && (
                  <p className="mt-2 rounded-md bg-muted/50 px-2 py-1.5 text-micro text-muted-foreground">
                    이 기간은 발표가 많아 <b>앞쪽 {events?.length}건</b>만 보여줍니다. 위에서
                    날짜를 고르면 그날 전체를 봅니다.
                  </p>
                )}

                <p className="mt-2 border-t pt-2 text-micro leading-relaxed text-muted-foreground">
                  {copy.source}
                </p>
              </Card>
            </div>
          )}
        </PageColumns>

        <EarningsDetailModal event={openEvent} onClose={() => setOpenEvent(null)} />
      </PageScrollShell>
    </>
  );
}

/** 「일」 보기의 하루 이동. 달력을 접어 두고 하루씩 훑는 흐름. */
function DayNavigator({
  date,
  onChange,
}: {
  date: string;
  onChange: (date: string) => void;
}) {
  const shift = (delta: number) => {
    const [y, m, d] = date.split("-").map(Number);
    const next = new Date(Date.UTC(y, m - 1, d + delta));
    onChange(
      `${next.getUTCFullYear()}-${String(next.getUTCMonth() + 1).padStart(2, "0")}-${String(
        next.getUTCDate(),
      ).padStart(2, "0")}`,
    );
  };
  return (
    <div className="flex items-center justify-between rounded-xl border bg-card px-3 py-2">
      <button
        type="button"
        onClick={() => shift(-1)}
        className="rounded-md px-2 py-1 text-cell text-muted-foreground hover:bg-muted hover:text-foreground"
      >
        ← 전날
      </button>
      <div className="flex items-center gap-2">
        <span className="text-sm font-semibold">{formatDayLabel(date)}</span>
        {date !== todayKST() && (
          <button
            type="button"
            onClick={() => onChange(todayKST())}
            className="rounded-full bg-muted px-2 py-0.5 text-micro text-muted-foreground hover:text-foreground"
          >
            오늘로
          </button>
        )}
      </div>
      <button
        type="button"
        onClick={() => shift(1)}
        className="rounded-md px-2 py-1 text-cell text-muted-foreground hover:bg-muted hover:text-foreground"
      >
        다음날 →
      </button>
    </div>
  );
}

/**
 * 「내 종목」 필터. 비로그인이거나 담은 종목이 없으면 **끈 상태로 잠근다** —
 * 눌러도 아무 일이 없는 토글은 고장으로 보인다.
 */
function MineToggle({
  on,
  available,
  onChange,
}: {
  on: boolean;
  available: boolean;
  onChange: (v: boolean) => void;
}) {
  return (
    <button
      type="button"
      disabled={!available}
      onClick={() => onChange(!on)}
      title={
        available
          ? "관심 종목과 보유 종목만 봅니다"
          : "관심 종목이나 보유 종목을 담으면 사용할 수 있습니다"
      }
      className={cn(
        buttonVariants({ variant: "outline", size: "xs" }),
        "rounded-full",
        on
          ? "border-foreground bg-foreground text-background"
          : "border-border text-muted-foreground hover:text-foreground",
        !available && "cursor-not-allowed opacity-50",
      )}
    >
      내 종목만
    </button>
  );
}
