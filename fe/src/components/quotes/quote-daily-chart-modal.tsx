"use client";

import Link from "next/link";
import { ArrowRight } from "lucide-react";
import { QuoteDailyChart } from "@/components/quotes/quote-daily-chart";
import { QuoteChange, QuotePrice } from "@/components/quotes/quote-price";
import { StockMetricsGrid } from "@/components/stock-workspace/stock-metrics-column";
import { StockAvatar } from "@/components/stocks/stock-avatar";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { useExchangeRate } from "@/hooks/use-exchange-rate";
import { useQuoteDetail } from "@/hooks/use-quote-detail";
import type { QuoteDetail, StockInfo } from "@/lib/quotes-types";
import { quoteChangeBaseline, quoteChangeBasisLabel } from "@/lib/quotes-types";

type QuoteDailyChartModalProps = {
  open: boolean;
  onClose: () => void;
  stock: StockInfo | null;
  /** 목록이 이미 들고 있는 시세 — 모달이 뜨는 동안 헤더가 비지 않게. */
  initialQuote?: QuoteDetail | null;
  compact?: boolean;
  /** 분봉·호가가 필요할 때 나가는 경로. 기본 동작은 모달에서 끝낸다. */
  fullQuotesHref?: string;
};

/**
 * AI 추천 랭킹의 「실시간 시세」— 종목 퀵룩과 다른 모달.
 *
 * 퀵룩은 스파크라인으로 종목을 **훑는** 화면이다. 여기서 필요한 것은 실시간 시세
 * 개요 탭과 **같은 일봉 캔들**이다. 예전엔 `/market`으로 화면을 떠나 랭킹 맥락이
 * 끊겼다. 열 때만 시세·캔들을 구독한다.
 */
export function QuoteDailyChartModal({
  open,
  onClose,
  stock,
  initialQuote = null,
  compact = false,
  fullQuotesHref,
}: QuoteDailyChartModalProps) {
  return (
    <Dialog open={open} onOpenChange={(next) => !next && onClose()}>
      <DialogContent className="flex max-h-[88dvh] flex-col gap-0 overflow-hidden p-0 sm:max-w-3xl lg:max-w-4xl">
        {open && stock ? (
          <QuoteDailyChartModalBody
            key={stock.symbol}
            stock={stock}
            initialQuote={initialQuote}
            compact={compact}
            fullQuotesHref={fullQuotesHref}
            onClose={onClose}
          />
        ) : null}
      </DialogContent>
    </Dialog>
  );
}

function QuoteDailyChartModalBody({
  stock,
  initialQuote,
  compact,
  fullQuotesHref,
  onClose,
}: {
  stock: StockInfo;
  initialQuote: QuoteDetail | null;
  compact: boolean;
  fullQuotesHref?: string;
  onClose: () => void;
}) {
  const { detail, loading } = useQuoteDetail(stock.symbol);
  const quote = detail ?? initialQuote;
  const exchangeRate = useExchangeRate(true);

  const updatedLabel =
    quote?.updatedAt &&
    new Date(quote.updatedAt).toLocaleTimeString("ko-KR", {
      timeZone: "Asia/Seoul",
      hour: "2-digit",
      minute: "2-digit",
      second: "2-digit",
    });

  return (
    <>
      <DialogHeader className="shrink-0 border-b border-[var(--border)] px-4 py-3 pr-10 text-left">
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
          {quote && quote.currentPrice > 0 && (
            <span className="inline-flex items-center gap-1 text-[9px] font-normal text-[var(--profit)]">
              <span className="h-1.5 w-1.5 animate-pulse rounded-full bg-[var(--profit)]" />
              실시간
            </span>
          )}
        </DialogTitle>
        <DialogDescription className="flex flex-wrap items-center gap-1.5 text-[11px]">
          <span className="font-mono">{stock.symbol}</span>
          <Badge variant="default" className="px-1 py-0 text-[9px]">
            {stock.market}
          </Badge>
          {updatedLabel ? `${updatedLabel} 기준 · ${quote ? quoteChangeBasisLabel(quote) : ""}` : "일봉 차트"}
        </DialogDescription>

        {quote && quote.currentPrice > 0 ? (
          <div className="mt-2 flex flex-wrap items-end gap-x-3 gap-y-1">
            <QuotePrice
              value={quote.currentPrice}
              currency={quote.currency}
              exchangeRate={exchangeRate}
              size="xl"
              className="!text-xl sm:!text-2xl"
            />
            <QuoteChange
              change={quote.change}
              changeRate={quote.changeRate}
              currency={quote.currency}
              exchangeRate={exchangeRate}
              size="sm"
              hasBaseline={quoteChangeBaseline(quote) > 0}
            />
          </div>
        ) : (
          <p className="mt-2 text-[11px] text-[var(--text-muted)]">
            {loading ? "시세를 불러오는 중…" : "시세 데이터가 없습니다"}
          </p>
        )}
      </DialogHeader>

      <div className="min-h-0 flex-1 space-y-3 overflow-y-auto px-4 py-3">
        <QuoteDailyChart
          symbol={stock.symbol}
          currency={quote?.currency ?? stock.currency}
          livePrice={quote?.currentPrice ?? 0}
          compact={compact}
          height={compact ? 240 : 360}
          enableLoadMore
        />
        {quote && <StockMetricsGrid quote={quote} />}
      </div>

      {fullQuotesHref && (
        <div className="flex shrink-0 items-center justify-end border-t border-[var(--border)] px-4 py-3">
          <Button asChild variant="outline" size="sm">
            <Link href={fullQuotesHref} onClick={onClose}>
              실시간 차트에서 더 보기
              <ArrowRight className="size-3.5" />
            </Link>
          </Button>
        </div>
      )}
    </>
  );
}
