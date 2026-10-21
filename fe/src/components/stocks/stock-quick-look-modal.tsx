"use client";

import { useEffect, useMemo, useState, type ReactNode } from "react";
import Link from "next/link";
import { ArrowRight, Sparkles } from "lucide-react";
import { DailyCloseMiniTable } from "@/components/charts/daily-close-mini-table";
import { DailyCloseRangeToggle } from "@/components/charts/daily-close-range-toggle";
import { PriceHistoryChart } from "@/components/charts/price-history-chart";
import { useNewsArticleModal } from "@/components/news/news-article-modal-host";
import { StockAvatar } from "@/components/stocks/stock-avatar";
import { StockQuickLookActions } from "@/components/stocks/stock-quick-look-actions";
import { StockScoreChip } from "@/components/stocks/stock-score-chip";
import { WatchlistRangeBar } from "@/components/watchlist/watchlist-range-bar";
import { Button } from "@/components/ui/button";
import { MiniToggle } from "@/components/ui/mini-toggle";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { usePinnedSymbol } from "@/hooks/use-pinned-symbol";
import { useSparklineCandles } from "@/hooks/use-sparkline-candles";
import { analysisSymbolHref } from "@/lib/analysis-tabs";
import { quickLookAISummary } from "@/lib/daily-close-ai-summary";
import { fetchStockDailyCloses } from "@/lib/daily-close-api";
import type { DailyCloseRow } from "@/lib/daily-close-types";
import {
  availableBarSteps,
  DAILY_CLOSE_MAX_BARS,
  resolveBarStep,
} from "@/lib/daily-close-window";
import { fetchEarningsClient } from "@/lib/earnings-api";
import { formatEarningsDateLabel } from "@/lib/earnings-date";
import type { EarningsCalendarEvent } from "@/lib/earnings-types";
import {
  formatPercent,
  formatPrice,
  formatPriceChange,
  formatRelativeTime,
} from "@/lib/format";
import { leaveModal } from "@/lib/modal-history";
import { fetchStockNews } from "@/lib/news-api";
import type { NewsArticle } from "@/lib/news-types";
import type { QuoteAssetType, QuoteCurrency } from "@/lib/quotes-types";
import {
  SCORE_AXIS_LABELS,
  SCORE_AXIS_ORDER,
  type StockScoreData,
} from "@/lib/stock-score-api";
import { trendTextClass } from "@/lib/trend-colors";
import { cn } from "@/lib/utils";

/**
 * 종목 퀵룩 — 앱에서 종목을 눌렀을 때 뜨는 **유일한** 모달 형태.
 *
 * 예전에는 화면마다 다른 모달이 떴다. 관심종목은 차트·52주·점수·일정·뉴스를 보여주고,
 * 보유종목은 손익과 매매 이력만, 점수 카드는 레이더만 보여줬다. 같은 종목을 어디서
 * 눌렀느냐에 따라 아는 것이 달라지는 셈이라, 종목을 훑는 사람은 매번 다른 화면을
 * 배워야 했다.
 *
 * 그래서 뼈대를 하나로 고정한다 — 헤더(이름·시세) / 차트 / 52주 / **맥락** / 점수 /
 * 일정 / 뉴스 / **부가** / 푸터(종목 분석 보기 + 화면별 동작). 화면마다 다른 것은
 * `contextSection`(관심=담은 뒤, 보유=내 보유)과 `extraSections`뿐이다.
 *
 * 차트·일정·뉴스는 **열 때만** 부른다. 목록에 스무 종목이 있어도 요청은 연 것 하나뿐이다.
 */

export type QuickLookStock = {
  symbol: string;
  name: string;
  currency: QuoteCurrency;
  assetType?: QuoteAssetType;
  /** 거래소 표기(KOSPI·NASDAQ…). 없으면 통화로 추정한다. */
  marketLabel?: string;
  currentPrice: number;
  change: number;
  changeRate: number;
  hasQuote: boolean;
  week52Low?: number | null;
  week52High?: number | null;
  changeRateWeek52Low?: number | null;
  changeRateWeek52High?: number | null;
  /** 52주 레인지 안 현재가 위치(0~100). */
  rangePct?: number | null;
  isHeld?: boolean;
};

export function StockQuickLookModal({
  stock,
  score,
  open,
  onClose,
  markerPrice,
  markerLabel,
  contextSection,
  extraSections,
  footerActions,
  loadingLabel = "시세를 불러오는 중…",
}: {
  stock: QuickLookStock | null;
  score?: StockScoreData | null;
  open: boolean;
  onClose: () => void;
  /** 차트에 겹칠 기준선(보유 화면의 평단). */
  markerPrice?: number | null;
  markerLabel?: string;
  /** 52주 아래에 들어가는 화면별 블록. */
  contextSection?: ReactNode;
  /** 뉴스 아래에 들어가는 화면별 블록(폼·이력 등). */
  extraSections?: ReactNode;
  /** 푸터 우측 버튼(관심 삭제·보유 삭제 등). */
  footerActions?: ReactNode;
  loadingLabel?: string;
}) {
  const symbol = stock?.symbol ?? "";
  const name = stock?.name ?? "";
  const market: "domestic" | "us" = stock?.currency === "USD" ? "us" : "domestic";

  const [eventsState, setEventsState] = useState<{
    key: string;
    rows: EarningsCalendarEvent[];
  } | null>(null);
  const [newsState, setNewsState] = useState<{ key: string; rows: NewsArticle[] } | null>(null);
  const [closes, setCloses] = useState<{ key: string; rows: DailyCloseRow[] } | null>(null);
  const [bars, setBars] = useState(60);
  // AI 핵심 요약의 간단/상세. 기본값 간단 — 이유는 그 자리의 주석에.
  const [aiDetail, setAiDetail] = useState<"brief" | "full">("brief");

  const { points, times, loading: candlesLoading } = useSparklineCandles(symbol, open);

  /*
    이 창이 떠 있는 동안은 **이 종목을 보고 있다** (2026-09-15 사용자 요청 4).

    탭이 숨어 유예가 지나면 가격 구독이 목록 전체에서 이 한 종목으로 줄어든다
    (`lib/pinned-symbols`). 창을 닫거나 다른 종목으로 옮기면 그 순간 핀도 옮겨 간다 —
    이펙트의 정리 함수가 그 일을 한다.
  */
  usePinnedSymbol(symbol, market, open);

  /*
    일별 종가를 **한 번에 최대치로** 받는다 (2026-09-05 사용자 요청 7-1·7-2).

    이 한 응답이 세 블록을 먹인다 — 차트(기간 토글은 이 배열을 자를 뿐 요청을 더 만들지
    않는다), 「AI 핵심 요약」, 「최근 일별」 표. 점수·해설·전일비가 전부 이 행들에 이미
    실려 온다(`daily_close.go`), 그래서 요청은 여전히 종목당 하나다.
  */
  useEffect(() => {
    if (!open || !symbol) return;
    let alive = true;
    fetchStockDailyCloses(symbol, DAILY_CLOSE_MAX_BARS)
      .then((series) => {
        if (alive) setCloses({ key: symbol, rows: series.rows ?? [] });
      })
      .catch(() => {
        if (alive) setCloses({ key: symbol, rows: [] });
      });
    return () => {
      alive = false;
    };
  }, [open, symbol]);

  useEffect(() => {
    if (!open || !symbol) return;
    let alive = true;
    fetchEarningsClient(symbol)
      .then((data) => {
        if (!alive) return;
        setEventsState({
          key: symbol,
          rows: (data.calendar ?? []).filter((ev) => ev.kind === "upcoming").slice(0, 4),
        });
      })
      .catch(() => {
        if (alive) setEventsState({ key: symbol, rows: [] });
      });
    fetchStockNews({ symbol, name: name || symbol, market, limit: 4 })
      .then((rows) => {
        if (alive) setNewsState({ key: symbol, rows });
      })
      .catch(() => {
        if (alive) setNewsState({ key: symbol, rows: [] });
      });
    return () => {
      alive = false;
    };
  }, [open, symbol, name, market]);

  const events = eventsState?.key === symbol ? eventsState.rows : null;
  const news = newsState?.key === symbol ? newsState.rows : null;
  const dailyRows = closes?.key === symbol ? closes.rows : null;

  /*
    일별 종가는 **최신순**으로 온다(BE가 전일비를 다음 행에서 계산한다). 차트는 왼쪽이
    과거여야 하므로 뒤집는다 — 정렬 없이 자르면 가장 오래된 구간을 거꾸로 그리게 된다.
    지수 모달이 같은 처리를 한다.
  */
  const chrono = useMemo(
    () => (dailyRows ? [...dailyRows].reverse().filter((r) => r.close > 0) : []),
    [dailyRows],
  );

  /*
    일별 종가가 **아직 안 쌓인 종목**이 있다(신규 상장·ETN 등). 그때는 예전처럼 일봉
    스냅샷 40포인트로 그리고, 제목을 「최근 흐름」으로 되돌린다 — 「일별 종가」라고
    적어 놓고 다른 재료를 그리면 그건 다른 이야기가 된다.
  */
  const usingCandleFallback = dailyRows != null && chrono.length < 2;

  const steps = useMemo(() => availableBarSteps(chrono.length), [chrono.length]);
  const activeStep = resolveBarStep(steps, bars);
  const windowed = chrono.slice(-activeStep);
  const chartPoints = usingCandleFallback ? points : windowed.map((r) => r.close);
  const chartTimes = usingCandleFallback ? times : windowed.map((r) => r.tradeDate);
  const chartLoading = dailyRows === null || (usingCandleFallback && candlesLoading);
  /*
    신규 상장 종목은 일별 종가도 일봉도 2개가 안 된다. 차트는 점이 2개 미만이면
    로딩 막대를 그리므로, 여기서 「다 받았는데 없다」를 문장으로 갈라 준다.
  */
  const chartEmpty = !chartLoading && chartPoints.filter((v) => v > 0).length < 2;
  // 거래 이력이 짧은 종목(신규 상장 등)은 점수가 없는 까닭을 말해 준다.
  const shortHistory = dailyRows != null && dailyRows.length < 20;

  /*
    「AI 핵심 요약」에 쓸 **가장 최신 LLM 추출값** (2026-09-05 사용자 요청 7-2).

    행마다 `analysisSource`가 `llm` / `rule` / 빈 값 중 하나다. 규칙이 만든 문장까지
    「AI」라고 부르면 그 이름이 아무것도 구분하지 못하므로 **`llm`인 행만** 고른다.
    오늘 배치가 아직 안 돌았어도 어제 것이 있으면 그것을 보여 준다 — 없는 날 자리를
    비워 두면 「이 종목은 요약이 없다」로 읽히는데, 사실은 오늘 것만 없다.
  */
  const llmDaily = useMemo(
    () =>
      (dailyRows ?? []).find(
        (r) => r.analysisSource === "llm" && r.aiSummary.trim() !== "",
      ) ?? null,
    [dailyRows],
  );

  /*
    「간단」이 보여 줄 **짧은 판** (2026-09-15 사용자 요청 1).

    이제 서버가 그날의 요약 한 문단을 일별 행에 같이 실어 준다(`aiOverview`) — 네
    갈래를 전부 근거로 **다시 쓴** 글이다. 예전에는 본문의 앞 두 문장을 잘라 썼는데,
    본문은 네 갈래를 순서대로 이어붙인 것이라 앞 두 문장이 언제나 밸류에이션
    이야기였다. 규칙 전체와 되돌아가는 조건은 `lib/daily-close-ai-summary`에 있다.
  */
  const ai = useMemo(() => quickLookAISummary(llmDaily), [llmDaily]);

  /*
   * 기사는 앱 전체와 **같은 방식**으로 연다 — 여기서 부르는 목록이 이 종목 기사라
   * 관련 종목 카드에 자기 자신이 시세·점수·일정과 함께 다시 서고, 해외 기사면
   * 번역·원문 셸로 열린다. 예전에는 셸 하나를 문맥 없이 띄웠다.
   *
   * 이 모달이 이미 받아 둔 기사 목록을 `pool`로 넘긴다.
   */
  const newsModal = useNewsArticleModal({ pool: news ?? undefined });

  /*
   * 부모가 모달을 닫으면 열려 있던 기사도 접는다. 이펙트가 아니라 **렌더 중 조정**인
   * 이유는 React가 권하는 방식이기도 하고(prop이 바뀔 때의 파생 상태), 이펙트로 하면
   * 닫힌 프레임이 한 번 더 그려지기 때문이다.
   */
  const [wasOpen, setWasOpen] = useState(open);
  if (wasOpen !== open) {
    setWasOpen(open);
    if (!open && newsModal.article) newsModal.close();
  }

  /*
   * 종목을 옮기면 AI 요약을 **다시 접는다** (2026-09-14 사용자 요청 2).
   *
   * 기본값이 「간단」인 이유가 「창으로 종목을 훑을 때 한눈에 보기」인데, 한 종목에서
   * 상세를 편 것이 다음 종목까지 따라오면 그 목적이 두 번째 종목부터 사라진다.
   * 위 `wasOpen`과 같은 **렌더 중 조정**이다 — 이펙트로 하면 펼쳐진 프레임이 한 번
   * 더 그려진다.
   */
  const [lastSymbol, setLastSymbol] = useState(symbol);
  if (lastSymbol !== symbol) {
    setLastSymbol(symbol);
    setAiDetail("brief");
  }

  const hasRange =
    stock?.week52Low != null && stock.week52Low > 0 && stock?.week52High != null && stock.week52High > 0;

  return (
    <>
      {/*
        기사를 열면 이 목록 모달을 **닫는다**(다시 열면 이 컴포넌트의 상태가 그대로라
        보던 종목·받아둔 뉴스가 유지된다). 겹쳐 띄우면 Radix의 포커스 트랩·바깥 클릭
        해제와 기사 모달의 백드롭이 서로를 먹어, 기사를 닫을 때 목록까지 함께 닫힌다.
      */}
      <Dialog open={open && !newsModal.article} onOpenChange={(next) => !next && onClose()}>
        <DialogContent className="flex max-h-[88dvh] flex-col gap-0 overflow-hidden p-0 sm:max-w-lg">
          {stock ? (
            <>
              <DialogHeader className="shrink-0 gap-1.5 border-b px-4 py-3 pr-10 text-left">
                {/*
                  # 동작 다섯은 **가격 위**에 선다 (2026-09-05 사용자 요청 7-4)

                  관심 · 메모장 · 보유 · 스탬프 · 목표가. 지금까지 이 다섯은 랭킹 목록의
                  행과 종목 상세 헤더에 나뉘어 있어서, 뉴스나 홈에서 종목을 눌러 창을 연
                  사람은 무엇도 할 수 없었다(`stock-quick-look-actions.tsx` 머리말).

                  `pr-10`이 오른쪽 위 닫기(×) 자리를 비워 두므로 그 아래 줄에 오른쪽
                  정렬로 붙인다 — 바로 아래 가격·등락률과 같은 축이다.
                */}
                <div className="flex flex-wrap items-center justify-end gap-1">
                  <StockQuickLookActions
                    stock={{
                      symbol: stock.symbol,
                      name: stock.name || stock.symbol,
                      currency: stock.currency,
                      assetType: stock.assetType,
                      marketLabel: stock.marketLabel,
                      price: stock.hasQuote ? stock.currentPrice : undefined,
                      week52High: stock.week52High,
                      week52Low: stock.week52Low,
                    }}
                  />
                </div>
                <DialogTitle className="flex min-w-0 items-center gap-2 text-sm">
                  <StockAvatar
                    symbol={stock.symbol}
                    name={stock.name}
                    currency={stock.currency}
                    assetType={stock.assetType}
                    size="sm"
                    className="shrink-0"
                  />
                  <span className="min-w-0 flex-1 truncate">{stock.name || stock.symbol}</span>
                  <span className="shrink-0 text-right">
                    <span className="block font-num text-base font-bold tabular-nums">
                      {stock.hasQuote ? formatPrice(stock.currentPrice, stock.currency) : "—"}
                    </span>
                    {stock.hasQuote && (
                      <span
                        className={cn(
                          "block font-num text-cell tabular-nums",
                          trendTextClass(stock.changeRate),
                        )}
                      >
                        {formatPriceChange(stock.change, stock.currency)} (
                        {formatPercent(stock.changeRate)})
                      </span>
                    )}
                  </span>
                </DialogTitle>
                <DialogDescription className="text-cell">
                  {stock.symbol} · {stock.marketLabel || (market === "us" ? "US" : "KRX")}
                  {stock.isHeld ? " · 보유 중" : ""}
                  {score?.score != null ? ` · 점수 ${score.score}${score.grade ?? ""}` : ""}
                </DialogDescription>
              </DialogHeader>

              <div className="min-h-0 flex-1 overflow-y-auto px-4 py-3">
                {/*
                  # 「최근 흐름」 → 「일별 종가」 (2026-09-05 사용자 요청 7-1)

                  예전에는 일봉 스냅샷 **40포인트 고정**이었다(`useSparklineCandles`).
                  이름이 「최근 흐름」이었던 것은 그 40개가 며칠치인지 화면도 모르기
                  때문이다 — 커버리지가 성근 종목은 40봉이 8개월에 걸쳐 있다.

                  지금은 지수 모달과 **같은 재료·같은 토글**을 쓴다: 일별 종가를 최대치로
                  한 번 받아 20일 / 60일 / 전체로 자른다. 기간을 바꿔도 요청은 나가지
                  않는다. 이력이 얕은 종목은 토글이 줄거나 사라지고, 아예 없으면 예전
                  스파크라인으로 되돌아가며 제목도 「최근 흐름」으로 돌아간다.
                */}
                <QuickLookSection
                  title={usingCandleFallback ? "최근 흐름" : "일별 종가"}
                  action={
                    <DailyCloseRangeToggle steps={steps} active={activeStep} onChange={setBars} />
                  }
                >
                  {chartLoading ? (
                    <div className="h-[96px] animate-pulse rounded bg-muted" />
                  ) : chartEmpty ? (
                    <p className="py-6 text-center text-cell text-muted-foreground">
                      거래일이 쌓이면 차트가 표시됩니다
                    </p>
                  ) : (
                    <PriceHistoryChart
                      points={chartPoints}
                      times={chartTimes}
                      currency={stock.currency}
                      markerPrice={markerPrice ?? undefined}
                      markerLabel={markerLabel}
                      title=""
                    />
                  )}
                </QuickLookSection>

                {hasRange && (
                  <QuickLookSection title="52주 레인지">
                    <WatchlistRangeBar
                      pct={stock.rangePct ?? null}
                      low={stock.week52Low ?? undefined}
                      high={stock.week52High ?? undefined}
                      currency={stock.currency}
                    />
                    <p className="mt-1 text-center text-cell text-muted-foreground">
                      저점 대비{" "}
                      <b className={cn(trendTextClass(stock.changeRateWeek52Low ?? 0))}>
                        {stock.changeRateWeek52Low != null
                          ? formatPercent(stock.changeRateWeek52Low)
                          : "—"}
                      </b>{" "}
                      · 고점 대비{" "}
                      <b className={cn(trendTextClass(stock.changeRateWeek52High ?? 0))}>
                        {stock.changeRateWeek52High != null
                          ? formatPercent(stock.changeRateWeek52High)
                          : "—"}
                      </b>
                    </p>
                  </QuickLookSection>
                )}

                {contextSection}

                <QuickLookSection
                  title={
                    <span className="flex items-center gap-1.5">
                      종목 점수
                      {/* 배지 모양·설명은 앱 전체가 하나를 쓴다 — 뉴스 모달의 카드와
                          같은 숫자를 여기서 다르게 그리면 읽는 법이 둘이 된다. */}
                      <StockScoreChip
                        symbol={stock.symbol}
                        score={score?.score}
                        grade={score?.grade}
                        tradeDate={score?.tradeDate}
                        onNavigate={onClose}
                      />
                    </span>
                  }
                >
                  {score?.score == null ? (
                    <p className="py-3 text-center text-cell text-muted-foreground">
                      {shortHistory
                        ? "거래 이력이 짧아 아직 점수가 없습니다"
                        : "산출된 점수가 없습니다"}
                    </p>
                  ) : (
                    <div className="space-y-1">
                      {SCORE_AXIS_ORDER.map((axis) => {
                        const value = score.breakdown?.[axis];
                        return (
                          <div
                            key={axis}
                            className="grid grid-cols-[3.5rem_1fr_1.75rem] items-center gap-2"
                          >
                            <span className="text-cell text-muted-foreground">
                              {SCORE_AXIS_LABELS[axis]}
                            </span>
                            <span className="h-1.5 overflow-hidden rounded-full bg-muted">
                              {/* 값이 없는 축은 막대를 그리지 않는다 — 0으로 그리면
                                  "그 항목이 바닥"이라는 없는 사실이 된다. */}
                              {value != null && (
                                <span
                                  className="block h-full rounded-full bg-[var(--brand)]"
                                  style={{ width: `${Math.min(100, Math.max(0, value))}%` }}
                                />
                              )}
                            </span>
                            <span className="text-right font-num text-cell tabular-nums">
                              {value == null ? "—" : Math.round(value)}
                            </span>
                          </div>
                        );
                      })}
                      {/*
                        기준일·면책 문구는 뺐다 (2026-09-05 사용자 요청 7-3). 기준일은
                        제목 옆 점수 배지가 이미 달고 있고(`StockScoreChip`), 면책은
                        모든 창에서 같은 말을 반복하며 읽히지 않는 줄이 됐다.
                      */}
                    </div>
                  )}
                </QuickLookSection>

                {/*
                  # AI 핵심 요약 (2026-09-05 사용자 요청 7-2)

                  일별 종가 응답에 **이미 실려 오는** LLM 추출값이다(추가 요청 0). 지금까지
                  이 문장은 종목 상세의 「AI 분석」 탭에서만 볼 수 있어서, 창으로 종목을
                  훑는 사람은 점수 다섯 줄만 보고 **왜 그 점수인지**는 알 수 없었다.

                  `llm`인 행이 하나도 없으면 이 블록은 아예 뜨지 않는다 — 「요약 없음」
                  이라는 빈 상자를 세우면 창이 길어지기만 한다.
                */}
                {llmDaily && (
                  <QuickLookSection
                    title={
                      <span className="flex items-center gap-1.5">
                        <Sparkles className="size-3.5 text-[var(--brand)]" aria-hidden />
                        AI 핵심 요약
                        <span className="font-num text-micro font-normal tabular-nums">
                          {llmDaily.tradeDate} 기준
                        </span>
                      </span>
                    }
                    /*
                      간단/상세 (2026-09-14 사용자 요청 2).

                      종목 상세의 AI 카드가 같은 토글을 달고 있다 — 같은 글을 두
                      화면에서 다른 방식으로 접으면 사용자가 「모달에서는 왜 다
                      나오지」를 매번 다시 배운다. **기본값은 간단**이고, 종목을
                      옮길 때마다 되돌아온다(상태를 저장하지 않는다 — 창으로 종목을
                      훑는 일의 목적이 한눈에 보기다).

                      「간단」은 그날의 **핵심 요약 한 문단**이다(`aiOverview`) —
                      2026-09-15부터 서버가 일별 행에 같이 실어 준다. 그 이전 날짜에는
                      값이 없어 예전처럼 본문의 앞 두 문장으로 돌아간다
                      (`lib/daily-close-ai-summary`).

                      간단과 본문이 같은 글이면 토글을 아예 안 세운다. 눌러도 아무것도
                      안 바뀌는 버튼은 「고장났나」로 읽힌다.
                    */
                    action={
                      ai.hasDetail ? (
                        <MiniToggle
                          options={AI_DETAIL_OPTIONS}
                          value={aiDetail}
                          onChange={setAiDetail}
                        />
                      ) : undefined
                    }
                  >
                    <p className="whitespace-pre-line rounded-lg border px-2.5 py-2 text-read">
                      {ai.hasDetail && aiDetail === "brief" ? ai.brief : ai.full}
                    </p>
                  </QuickLookSection>
                )}

                {dailyRows !== null && dailyRows.length > 0 && (
                  <QuickLookSection title="최근 일별">
                    {/*
                      지수 모달과 **같은 표**다(`DailyCloseMiniTable`). 오늘 점수 하나만
                      보여 주면 그것이 높은 건지 알 수 없다 — 어제·그제가 옆에 서야
                      58이라는 숫자가 방향을 갖는다.

                      종목은 지수와 달리 해설 칸이 하나뿐이라(`aiSummary`), 규칙이 쓴
                      날은 펴지지 않게 `llm`인 행만 눌리게 한다.
                    */}
                    <DailyCloseMiniTable
                      rows={dailyRows}
                      formatValue={(v) => formatPrice(v, stock.currency)}
                      noteOf={(r) =>
                        r.analysisSource === "llm" && r.aiSummary.trim()
                          ? { text: r.aiSummary }
                          : null
                      }
                    />
                  </QuickLookSection>
                )}

                <QuickLookSection title="다가오는 일정">
                  {events === null ? (
                    <div className="h-10 animate-pulse rounded bg-muted" />
                  ) : events.length === 0 ? (
                    <p className="py-2 text-center text-cell text-muted-foreground">
                      예정된 실적·공시가 없습니다
                    </p>
                  ) : (
                    <ul>
                      {events.map((ev, i) => (
                        <li
                          key={`${ev.date}-${i}`}
                          className="flex items-start gap-2 py-1 text-cell"
                        >
                          {/*
                            `slice(5)`로 자르면 안 된다 — 미국 일정은 Finnhub가 `8/26`처럼
                            연도 없이 주므로 잘린 결과가 「6」이 된다(실측). 형식을 읽어서
                            거리로 적는다.
                          */}
                          <span className="w-[3.2rem] shrink-0 font-num tabular-nums text-muted-foreground">
                            {formatEarningsDateLabel(ev.date)}
                          </span>
                          <span className="min-w-0 flex-1 truncate">{ev.label.trim()}</span>
                          <span
                            className={cn(
                              "shrink-0 rounded px-1 text-micro",
                              ev.source === "dart"
                                ? "bg-[var(--brand-muted)] text-[var(--brand)]"
                                : "bg-muted text-muted-foreground",
                            )}
                          >
                            {ev.source === "dart" ? "DART" : "실적"}
                          </span>
                        </li>
                      ))}
                    </ul>
                  )}
                </QuickLookSection>

                <QuickLookSection title="최근 뉴스">
                  {news === null ? (
                    <div className="space-y-1.5">
                      {Array.from({ length: 3 }).map((_, i) => (
                        <div key={i} className="h-8 animate-pulse rounded bg-muted" />
                      ))}
                    </div>
                  ) : news.length === 0 ? (
                    <p className="py-2 text-center text-cell text-muted-foreground">
                      관련 뉴스가 없습니다
                    </p>
                  ) : (
                    <ul>
                      {news.map((a) => (
                        <li key={a.id} className="border-b py-1 last:border-0">
                          <button
                            type="button"
                            onClick={() => newsModal.open(a)}
                            className="block w-full rounded-md px-1 py-0.5 text-left hover:bg-accent"
                          >
                            <p className="line-clamp-2 text-cell leading-snug">
                              {a.headlineKo || a.headline}
                            </p>
                            <p className="mt-0.5 text-micro text-muted-foreground">
                              {formatRelativeTime(a.publishedAt)}
                            </p>
                          </button>
                        </li>
                      ))}
                    </ul>
                  )}
                </QuickLookSection>

                {extraSections}
              </div>

              <div className="flex shrink-0 flex-wrap items-center gap-2 border-t px-4 py-3">
                <Button asChild size="sm" className="min-w-32 flex-1">
                  {/* `leaveModal` — 그냥 닫으면 모달이 걷어내는 히스토리 항목이 이 이동을
                      밀어내 아무 일도 일어나지 않는다. 까닭은 `lib/modal-history`. */}
                  <Link
                    href={analysisSymbolHref(stock.symbol)}
                    onClick={() => leaveModal(onClose)}
                  >
                    종목 분석 보기
                    <ArrowRight className="size-3.5" />
                  </Link>
                </Button>
                {footerActions}
              </div>
            </>
          ) : (
            <div className="p-6">
              <DialogHeader>
                <DialogTitle className="text-sm">종목</DialogTitle>
                <DialogDescription className="text-cell">{loadingLabel}</DialogDescription>
              </DialogHeader>
            </div>
          )}
        </DialogContent>
      </Dialog>

      {newsModal.modal}
    </>
  );
}

/** 퀵룩 안의 한 블록. 화면별 섹션도 같은 제목 스타일을 쓰라고 내보낸다. */
const AI_DETAIL_OPTIONS: { id: "brief" | "full"; label: string }[] = [
  { id: "brief", label: "간단" },
  { id: "full", label: "상세" },
];

export function QuickLookSection({
  title,
  action,
  children,
}: {
  title: ReactNode;
  action?: ReactNode;
  children: ReactNode;
}) {
  return (
    <section className="mb-4 last:mb-0">
      <div className="mb-1.5 flex min-w-0 items-center justify-between gap-2">
        <h3 className="min-w-0 text-cell font-semibold text-muted-foreground">{title}</h3>
        {action}
      </div>
      {children}
    </section>
  );
}

/** 4칸 격자에 들어가는 작은 수치. */
export function QuickLookStat({
  label,
  value,
  className,
}: {
  label: string;
  value: ReactNode;
  className?: string;
}) {
  return (
    <div className="min-w-0">
      <p className="text-micro text-muted-foreground">{label}</p>
      <p className={cn("truncate font-num text-cell font-semibold tabular-nums", className)}>
        {value}
      </p>
    </div>
  );
}

/** 라벨 ↔ 값 한 줄. */
export function QuickLookKV({ label, value }: { label: string; value: ReactNode }) {
  return (
    <div className="flex min-w-0 items-center justify-between gap-2 py-0.5">
      <span className="shrink-0 text-cell text-muted-foreground">{label}</span>
      <span className="min-w-0 truncate text-right font-num text-cell tabular-nums">
        {value}
      </span>
    </div>
  );
}
