"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { Area, AreaChart, Tooltip, XAxis, YAxis } from "recharts";
import { Card, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { QuoteChange, QuotePrice } from "@/components/quotes/quote-price";
import { PriceDisplay } from "@/components/ui/price-display";
import { useExchangeRate } from "@/hooks/use-exchange-rate";
import { useQuoteDetail } from "@/hooks/use-quote-detail";
import { useQuoteCandles } from "@/hooks/use-quote-candles";
import { fetchPortfolioSummaryClient } from "@/lib/api-client";
import { addWatchlistClient, removeWatchlistClient, fetchWatchlistClient } from "@/lib/market-api";
import { ACCOUNT_FEATURES_VISIBLE } from "@/lib/account-features";
import type { UnifiedHolding } from "@/lib/portfolio-types";
import { trendTextClass } from "@/lib/trend-colors";
import { cn } from "@/lib/utils";

export function StockDetailClient({ symbol }: { symbol: string }) {
  const { detail, loading } = useQuoteDetail(symbol);
  const { candles } = useQuoteCandles(symbol, "1d");
  const exchangeRate = useExchangeRate(true);
  const [holding, setHolding] = useState<UnifiedHolding | null>(null);
  const [watchlisted, setWatchlisted] = useState(false);

  useEffect(() => {
    if (ACCOUNT_FEATURES_VISIBLE) {
      fetchPortfolioSummaryClient()
        .then((s) => setHolding(s.holdings.find((h) => h.symbol === symbol) ?? null))
        .catch(() => setHolding(null));
    } else {
      setHolding(null);
    }
    fetchWatchlistClient()
      .then((list) => setWatchlisted(list.some((i) => i.symbol === symbol)))
      .catch(() => setWatchlisted(false));
  }, [symbol]);

  async function toggleWatchlist() {
    try {
      if (watchlisted) await removeWatchlistClient(symbol);
      else await addWatchlistClient(symbol);
      setWatchlisted(!watchlisted);
    } catch {
      // ignore
    }
  }

  if (loading && !detail) {
    return <div className="p-6 text-sm text-[var(--text-muted)]">로딩 중…</div>;
  }

  if (!detail) {
    return <div className="p-6 text-sm text-[var(--text-muted)]">종목을 찾을 수 없습니다.</div>;
  }

  return (
    <div className="mx-auto max-w-6xl space-y-6">
      <section className="rounded-2xl border border-[var(--border)] bg-[var(--surface)] p-6">
        <div className="flex flex-wrap items-end justify-between gap-4">
          <div>
            <p className="text-sm text-[var(--text-muted)]">{detail.symbol}</p>
            <QuotePrice value={detail.currentPrice} currency={detail.currency} exchangeRate={exchangeRate} size="hero" />
            <QuoteChange
              change={detail.change}
              changeRate={detail.changeRate}
              currency={detail.currency}
              exchangeRate={exchangeRate}
              size="lg"
            />
          </div>
          <div className="flex gap-2">
            <button
              type="button"
              onClick={toggleWatchlist}
              className="rounded-lg border border-[var(--border)] px-4 py-2 text-sm hover:bg-[var(--surface-elevated)]"
            >
              {watchlisted ? "관심 해제" : "관심 추가"}
            </button>
          </div>
        </div>
      </section>

      <div className={cn("grid gap-6", ACCOUNT_FEATURES_VISIBLE ? "lg:grid-cols-3" : "")}>
        {ACCOUNT_FEATURES_VISIBLE && (
        <Card className="lg:col-span-1">
          <CardHeader>
            <CardTitle>{holding ? "보유 현황" : "종목 정보"}</CardTitle>
          </CardHeader>
          {holding ? (
            <dl className="space-y-3 px-4 pb-4 text-sm">
              <div className="flex justify-between">
                <dt className="text-[var(--text-muted)]">총 보유</dt>
                <dd className="font-mono font-semibold">{holding.totalShares}주</dd>
              </div>
              <div className="flex justify-between">
                <dt className="text-[var(--text-muted)]">평균단가</dt>
                <dd>
                  <QuotePrice
                    value={holding.avgPrice}
                    currency={holding.currency}
                    exchangeRate={holding.purchaseFxRate || exchangeRate}
                    size="sm"
                  />
                </dd>
              </div>
              <div className="flex justify-between">
                <dt className="text-[var(--text-muted)]">평가손익</dt>
                <dd className={cn(trendTextClass(holding.returnRateKRW))}>
                  {holding.currency === "USD" ? (
                    <QuotePrice
                      value={holding.marketValueNative - holding.purchaseAmountNative}
                      currency="USD"
                      exchangeRate={holding.purchaseFxRate || exchangeRate}
                      size="sm"
                      signed
                    />
                  ) : (
                    <PriceDisplay
                      value={holding.profitLossKRW}
                      currency="KRW"
                      size="sm"
                      signed
                    />
                  )}
                </dd>
              </div>
            </dl>
          ) : (
            <p className="px-4 pb-4 text-sm text-[var(--text-muted)]">보유하지 않은 종목입니다.</p>
          )}
        </Card>
        )}

        <Card className={ACCOUNT_FEATURES_VISIBLE ? "lg:col-span-2" : ""}>
          <CardHeader>
            <CardTitle>일봉 차트</CardTitle>
            <Badge variant="info">실시간</Badge>
          </CardHeader>
          <div className="h-64 px-2 pb-4">
            {candles.length > 0 ? (
              <AreaChart width={600} height={240} data={candles}>
                <XAxis dataKey="time" tick={{ fontSize: 10, fill: "#64748B" }} />
                <YAxis tick={{ fontSize: 10, fill: "#64748B" }} width={50} />
                <Tooltip />
                <Area type="monotone" dataKey="close" stroke="#3B82F6" fill="#3B82F640" />
              </AreaChart>
            ) : (
              <div className="flex h-full items-center justify-center text-sm text-[var(--text-muted)]">
                차트 데이터 없음
              </div>
            )}
          </div>
        </Card>
      </div>

      <div className="grid gap-4 md:grid-cols-2">
        <Link href={`/analysis?symbol=${encodeURIComponent(symbol)}`}>
          <Card className="transition-colors hover:border-[var(--accent)]">
            <CardTitle>종목 분석</CardTitle>
            <p className="mt-2 text-xs text-[var(--text-muted)]">
              펀더멘털 · 실적 · 공시 · AI 분석
            </p>
          </Card>
        </Link>
        <Link href={`/analysis?symbol=${encodeURIComponent(symbol)}#fundamentals`}>
          <Card className="transition-colors hover:border-[var(--accent)]">
            <CardTitle>재무제표</CardTitle>
            <p className="mt-2 text-xs text-[var(--text-muted)]">펀더멘털 지표 바로가기</p>
          </Card>
        </Link>
      </div>
    </div>
  );
}
