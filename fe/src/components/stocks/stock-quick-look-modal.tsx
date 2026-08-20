"use client";

import { useEffect, useState, type ReactNode } from "react";
import Link from "next/link";
import { ArrowRight } from "lucide-react";
import { PriceHistoryChart } from "@/components/charts/price-history-chart";
import { useNewsArticleModal } from "@/components/news/news-article-modal-host";
import { StockAvatar } from "@/components/stocks/stock-avatar";
import { StockScoreChip } from "@/components/stocks/stock-score-chip";
import { WatchlistRangeBar } from "@/components/watchlist/watchlist-range-bar";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { useSparklineCandles } from "@/hooks/use-sparkline-candles";
import { analysisSymbolHref } from "@/lib/analysis-tabs";
import { fetchEarningsClient } from "@/lib/earnings-api";
import { formatEarningsDateLabel } from "@/lib/earnings-date";
import type { EarningsCalendarEvent } from "@/lib/earnings-types";
import {
  formatPercent,
  formatPrice,
  formatPriceChange,
  formatRelativeTime,
} from "@/lib/format";
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

  const { points, times } = useSparklineCandles(symbol, open);

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
              <DialogHeader className="shrink-0 border-b px-4 py-3 pr-10 text-left">
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
                    <span className="block font-mono text-base font-bold tabular-nums">
                      {stock.hasQuote ? formatPrice(stock.currentPrice, stock.currency) : "—"}
                    </span>
                    {stock.hasQuote && (
                      <span
                        className={cn(
                          "block font-mono text-[11px] tabular-nums",
                          trendTextClass(stock.changeRate),
                        )}
                      >
                        {formatPriceChange(stock.change, stock.currency)} (
                        {formatPercent(stock.changeRate)})
                      </span>
                    )}
                  </span>
                </DialogTitle>
                <DialogDescription className="text-[11px]">
                  {stock.symbol} · {stock.marketLabel || (market === "us" ? "US" : "KRX")}
                  {stock.isHeld ? " · 보유 중" : ""}
                  {score?.score != null ? ` · 점수 ${score.score}${score.grade ?? ""}` : ""}
                </DialogDescription>
              </DialogHeader>

              <div className="min-h-0 flex-1 overflow-y-auto px-4 py-3">
                <PriceHistoryChart
                  points={points}
                  times={times}
                  currency={stock.currency}
                  markerPrice={markerPrice ?? undefined}
                  markerLabel={markerLabel}
                  className="mb-4"
                />

                {hasRange && (
                  <QuickLookSection title="52주 레인지">
                    <WatchlistRangeBar
                      pct={stock.rangePct ?? null}
                      low={stock.week52Low ?? undefined}
                      high={stock.week52High ?? undefined}
                      currency={stock.currency}
                    />
                    <p className="mt-1 text-center text-[11px] text-muted-foreground">
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
                    <p className="py-3 text-center text-[11px] text-muted-foreground">
                      산출된 점수가 없습니다
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
                            <span className="text-[11px] text-muted-foreground">
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
                            <span className="text-right font-mono text-[11px] tabular-nums">
                              {value == null ? "—" : Math.round(value)}
                            </span>
                          </div>
                        );
                      })}
                      <p className="pt-1 text-[10px] leading-relaxed text-muted-foreground">
                        {score.tradeDate} 기준 · 점수는 정보이지 매매 신호가 아닙니다.
                      </p>
                    </div>
                  )}
                </QuickLookSection>

                <QuickLookSection title="다가오는 일정">
                  {events === null ? (
                    <div className="h-10 animate-pulse rounded bg-muted" />
                  ) : events.length === 0 ? (
                    <p className="py-2 text-center text-[11px] text-muted-foreground">
                      예정된 실적·공시가 없습니다
                    </p>
                  ) : (
                    <ul>
                      {events.map((ev, i) => (
                        <li
                          key={`${ev.date}-${i}`}
                          className="flex items-start gap-2 py-1 text-[11px]"
                        >
                          {/*
                            `slice(5)`로 자르면 안 된다 — 미국 일정은 Finnhub가 `8/26`처럼
                            연도 없이 주므로 잘린 결과가 「6」이 된다(실측). 형식을 읽어서
                            거리로 적는다.
                          */}
                          <span className="w-[3.2rem] shrink-0 font-mono tabular-nums text-muted-foreground">
                            {formatEarningsDateLabel(ev.date)}
                          </span>
                          <span className="min-w-0 flex-1 truncate">{ev.label.trim()}</span>
                          <span
                            className={cn(
                              "shrink-0 rounded px-1 text-[10px]",
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
                    <p className="py-2 text-center text-[11px] text-muted-foreground">
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
                            <p className="line-clamp-2 text-[11px] leading-snug">
                              {a.headlineKo || a.headline}
                            </p>
                            <p className="mt-0.5 text-[10px] text-muted-foreground">
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
                  <Link href={analysisSymbolHref(stock.symbol, "ai")} onClick={onClose}>
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
                <DialogDescription className="text-xs">{loadingLabel}</DialogDescription>
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
        <h3 className="min-w-0 text-[11px] font-semibold text-muted-foreground">{title}</h3>
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
      <p className="text-[10px] text-muted-foreground">{label}</p>
      <p className={cn("truncate font-mono text-[12px] font-semibold tabular-nums", className)}>
        {value}
      </p>
    </div>
  );
}

/** 라벨 ↔ 값 한 줄. */
export function QuickLookKV({ label, value }: { label: string; value: ReactNode }) {
  return (
    <div className="flex min-w-0 items-center justify-between gap-2 py-0.5">
      <span className="shrink-0 text-[11px] text-muted-foreground">{label}</span>
      <span className="min-w-0 truncate text-right font-mono text-[11px] tabular-nums">
        {value}
      </span>
    </div>
  );
}
