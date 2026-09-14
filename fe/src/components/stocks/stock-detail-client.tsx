"use client";

import Link from "next/link";
import { analysisSymbolHref } from "@/lib/analysis-tabs";
import { useEffect, useRef, useState } from "react";
import { ChevronLeft } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { PriceDisplay } from "@/components/ui/price-display";
import { ChartZoomToolbar } from "@/components/quotes/chart-zoom-toolbar";
import { QuoteChartSlot } from "@/components/quotes/quote-chart-slot";
import type { CandlestickChartHandle } from "@/components/quotes/candlestick-chart";
import { QuoteChange, QuotePrice } from "@/components/quotes/quote-price";
import { PublishCallButton } from "@/components/research/publish-call-button";
import { StampButton } from "@/components/stamps/stamp-button";
import { HEADER_ACTION_CLASS } from "@/components/stocks/stock-symbol-action-bar";
import { Button } from "@/components/ui/button";
import { StockNewsPanel } from "@/components/stocks/stock-news-panel";
import { StockAvatar } from "@/components/stocks/stock-avatar";
import { useExchangeRate } from "@/hooks/use-exchange-rate";
import { useQuoteDetail } from "@/hooks/use-quote-detail";
import { useQuoteCandles } from "@/hooks/use-quote-candles";
import { useQuotesChartPrefs } from "@/hooks/use-quotes-chart-prefs";
import { addWatchlistClient, removeWatchlistClient, fetchWatchlistClient } from "@/lib/market-api";
import {
  formatChangeRate,
  formatMarketCap,
  formatTradingValue,
  formatVolume,
} from "@/lib/format";
import type { QuoteAssetType } from "@/lib/quotes-types";
import { quoteChangeBaseline, QUOTE_CHANGE_BASIS_LABEL } from "@/lib/quotes-types";
import { trendTextClass } from "@/lib/trend-colors";
import { cn } from "@/lib/utils";
import { useLoginGate } from "@/hooks/use-login-gate";

function stockNewsMarket(assetType: QuoteAssetType, currency: "KRW" | "USD"): "domestic" | "us" {
  if (assetType === "us" || currency === "USD") return "us";
  return "domestic";
}

export function StockDetailClient({ symbol }: { symbol: string }) {
  const { detail, loading } = useQuoteDetail(symbol);
  const { chartStyle } = useQuotesChartPrefs();
  const { candles, loadMore, hasMore, loadingMore, loading: candlesLoading } = useQuoteCandles(
    symbol,
    "1d",
    detail?.currentPrice ?? 0,
    // 마감 중에는 일봉이 자라지 않는다 — 2초 스냅샷 폴링을 걷는다.
    detail?.marketSession !== "closed",
  );
  const exchangeRate = useExchangeRate(true);
  const chartRef = useRef<CandlestickChartHandle>(null);
  const chartAreaRef = useRef<HTMLDivElement>(null);
  const [chartHeight, setChartHeight] = useState(360);
  const [watchlisted, setWatchlisted] = useState(false);
  const gate = useLoginGate("관심 추가");

  useEffect(() => {
    fetchWatchlistClient()
      .then((list) => setWatchlisted(list.some((i) => i.symbol === symbol)))
      .catch(() => setWatchlisted(false));
  }, [symbol]);

  useEffect(() => {
    const el = chartAreaRef.current;
    if (!el) return;
    const measure = () => setChartHeight(Math.max(260, el.clientHeight));
    measure();
    const observer = new ResizeObserver(measure);
    observer.observe(el);
    return () => observer.disconnect();
  }, [detail]);

  /*
    관심 등록도 로그인이 필요한 동작이다 (2026-09-04 사용자 요청 2 「다른 로그인이
    필요한 모든 버튼」).

    전에는 비로그인으로 누르면 401이 나고 `catch`가 그것을 통째로 삼켰다 — 하트가
    안 채워지고 화면에는 아무 말도 없어서, 사용자에게는 **버튼이 고장 난 것**이다.
    랭킹 목록의 같은 버튼은 이미 로그인으로 보내고 있었고(`market-radar-list`),
    상세 화면만 빠져 있었다.
  */
  async function toggleWatchlist() {
    if (!gate.run(() => {})) return;
    try {
      if (watchlisted) await removeWatchlistClient(symbol);
      else await addWatchlistClient(symbol);
      setWatchlisted(!watchlisted);
    } catch {
      // 등록 실패는 되돌리지 않는다 — 다음 조회가 서버 상태로 맞춘다.
    }
  }

  if (loading && !detail) {
    return (
      <div className="flex flex-1 items-center justify-center text-sm text-[var(--text-muted)]">
        로딩 중…
      </div>
    );
  }

  if (!detail) {
    return (
      <div className="flex flex-1 items-center justify-center text-sm text-[var(--text-muted)]">
        종목을 찾을 수 없습니다.
      </div>
    );
  }

  const newsMarket = stockNewsMarket(detail.assetType, detail.currency);
  const updatedLabel = detail.updatedAt
    ? new Date(detail.updatedAt).toLocaleTimeString("ko-KR", {
        timeZone: "Asia/Seoul",
        hour: "2-digit",
        minute: "2-digit",
        second: "2-digit",
      })
    : "—";

  const range52 =
    detail.week52High > detail.week52Low
      ? ((detail.currentPrice - detail.week52Low) / (detail.week52High - detail.week52Low)) * 100
      : 50;

  const currency = detail.currency;

  function StatPrice({ value }: { value: number }) {
    if (value <= 0) return <>—</>;
    return (
      <PriceDisplay value={value} currency={currency} size="sm" className="text-cell font-medium" />
    );
  }

  return (
    <div className="flex min-h-0 flex-1 flex-col overflow-hidden bg-[var(--background)]">
      <header className="shrink-0 border-b border-[var(--border)] bg-[var(--surface)] px-4 py-2.5 sm:px-5">
        <div className="flex flex-wrap items-start justify-between gap-2">
          <div className="min-w-0 flex-1">
            <div className="flex flex-wrap items-center gap-1.5">
              <Link
                href="/market"
                className="mr-0.5 flex h-7 w-7 shrink-0 items-center justify-center rounded-md border border-[var(--border)] text-[var(--text-muted)] hover:bg-[var(--surface-elevated)] hover:text-[var(--text-primary)]"
                aria-label="실시간 차트로 돌아가기"
              >
                <ChevronLeft className="h-4 w-4" />
              </Link>
              <StockAvatar
                symbol={detail.symbol}
                name={detail.name}
                currency={detail.currency}
                assetType={detail.assetType}
                size="md"
              />
              <h1 className="truncate text-base font-semibold text-[var(--text-primary)] sm:text-lg">
                {detail.name}
              </h1>
              <span className="font-num text-micro text-[var(--text-muted)]">{detail.symbol}</span>
              <Badge variant="default" className="text-micro">
                {detail.market}
              </Badge>
              {detail.isHeld && (
                <Badge variant="info" className="text-micro">
                  보유
                </Badge>
              )}
              <Badge variant="profit" className="animate-pulse text-micro">
                실시간
              </Badge>
            </div>
            <p className="mt-0.5 pl-8 text-micro text-[var(--text-muted)]">{updatedLabel} 기준</p>
          </div>

          <div className="flex shrink-0 flex-wrap items-center gap-1.5">
            {/* 크기는 전부 공용 `<Button size="sm">` — 예전엔 이 줄만 px-2.5 py-1.5로 따로 컸다. */}
            <Button variant="outline" size="sm" className={HEADER_ACTION_CLASS} asChild>
              <Link href={analysisSymbolHref(symbol, "company")}>종목 분석</Link>
            </Button>
            <Button
              variant="outline"
              size="sm"
              onClick={toggleWatchlist}
              aria-pressed={watchlisted}
              title={
                gate.authed ? undefined : "로그인하면 이 종목을 관심 목록에 담을 수 있습니다"
              }
              className={HEADER_ACTION_CLASS}
            >
              {/* 비로그인이면 눌렀을 때 로그인으로 나간다 — 버튼이 그렇게 말한다. */}
              {watchlisted ? "관심 해제" : gate.authed ? "관심 추가" : "로그인하고 관심 추가"}
            </Button>
            <StampButton
              symbol={symbol}
              price={detail.currentPrice}
              currency={detail.currency}
              week52High={detail.week52High}
              week52Low={detail.week52Low}
            />
            <PublishCallButton symbol={symbol} />
          </div>
        </div>

        <div className="mt-2 flex flex-wrap items-end gap-x-5 gap-y-1 pl-8">
          <QuotePrice
            value={detail.currentPrice}
            flashKey={detail.symbol}
            currency={detail.currency}
            exchangeRate={exchangeRate}
            size="hero"
            className="!text-2xl sm:!text-3xl lg:!text-4xl"
          />
          <QuoteChange
            change={detail.change}
            changeRate={detail.changeRate}
            currency={detail.currency}
            exchangeRate={exchangeRate}
            size="md"
            hasBaseline={quoteChangeBaseline(detail) > 0}
          />
        </div>

        <dl className="mt-2.5 grid grid-cols-3 gap-x-3 gap-y-1.5 pl-8 sm:grid-cols-4 lg:grid-cols-8">
          {[
            { label: "시가", price: detail.open },
            { label: "고가", price: detail.high },
            { label: "저가", price: detail.low },
            { label: "전일", price: detail.prevClose },
            { label: "거래량", text: formatVolume(detail.volume) },
            {
              label: "거래대금",
              text: formatTradingValue(detail.tradingValue, detail.currency, exchangeRate),
            },
            {
              label: "시가총액",
              text: formatMarketCap(detail.marketCap, detail.currency, exchangeRate),
            },
            ...(detail.regularClose != null && detail.regularClose > 0
              ? [{ label: "정규장 종가", price: detail.regularClose }]
              : []),
            {
              label: QUOTE_CHANGE_BASIS_LABEL,
              text: formatChangeRate(detail.changeRate, quoteChangeBaseline(detail) > 0),
            },
          ].map((stat) => (
            <div key={stat.label}>
              <dt className="text-micro text-[var(--text-muted)]">{stat.label}</dt>
              <dd
                className={cn(
                  "mt-0.5 text-cell font-medium",
                  stat.label === "등락률"
                    ? cn("font-sans tabular-nums", trendTextClass(detail.changeRate))
                    : "text-[var(--text-primary)]",
                )}
              >
                {"price" in stat && stat.price != null ? (
                  <StatPrice value={stat.price} />
                ) : (
                  ("text" in stat ? stat.text : "—")
                )}
              </dd>
            </div>
          ))}
        </dl>

        {detail.week52High > 0 && detail.week52Low > 0 && (
          <div className="mt-2 flex items-center gap-2 pl-8 text-micro text-[var(--text-muted)]">
            <span className="shrink-0">52주</span>
            <StatPrice value={detail.week52Low} />
            <div className="relative h-1 min-w-[80px] flex-1 max-w-xs rounded-full bg-[var(--surface-elevated)]">
              <div
                className="absolute top-1/2 h-2 w-2 -translate-x-1/2 -translate-y-1/2 rounded-full bg-[var(--text-primary)]"
                style={{ left: `${Math.min(100, Math.max(0, range52))}%` }}
              />
            </div>
            <StatPrice value={detail.week52High} />
          </div>
        )}
      </header>

      <div className="flex min-h-0 flex-1 flex-col overflow-hidden xl:flex-row">
        <section className="flex min-h-[280px] min-w-0 flex-1 flex-col overflow-hidden">
          <div className="flex shrink-0 items-center justify-between gap-2 border-b border-[var(--border)] px-3 py-1.5">
            <h2 className="text-cell font-medium text-[var(--text-secondary)]">일봉 차트</h2>
            <ChartZoomToolbar
              onZoomIn={() => chartRef.current?.zoomIn()}
              onZoomOut={() => chartRef.current?.zoomOut()}
              onReset={() => chartRef.current?.resetZoom()}
            />
          </div>
          <div ref={chartAreaRef} className="min-h-0 flex-1 p-1">
            <QuoteChartSlot
              chartRef={chartRef}
              candles={candles}
              loading={candlesLoading}
              currency={detail.currency}
              interval="1d"
              chartStyle={chartStyle}
              height={chartHeight}
              hasMore={hasMore}
              loadingMore={loadingMore}
              onLoadMore={loadMore}
              /* 현재가 표는 위 헤더가 적는 값 그대로 — 사용자 지적 4 */
              livePrice={detail.currentPrice}
            />
          </div>
        </section>

        <aside className="flex max-h-[40vh] min-h-0 shrink-0 flex-col gap-3 overflow-y-auto border-t border-[var(--border)] bg-[var(--surface)] p-3 xl:max-h-none xl:w-72 xl:border-l xl:border-t-0">
          <StockNewsPanel
            symbol={symbol}
            name={detail.name}
            market={newsMarket}
            variant="compact"
            className="min-h-0 flex-1"
          />
        </aside>
      </div>
    </div>
  );
}
