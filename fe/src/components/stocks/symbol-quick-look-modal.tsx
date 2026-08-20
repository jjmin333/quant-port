"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import {
  StockQuickLookModal,
  type QuickLookStock,
} from "@/components/stocks/stock-quick-look-modal";
import { useStockScores } from "@/hooks/use-stock-scores";
import { fetchQuotesBatchClient } from "@/lib/market-api";
import type { QuoteSummary } from "@/lib/quotes-types";

/**
 * **심볼만 알아도 열리는** 종목 퀵룩.
 *
 * 공용 `StockQuickLookModal`은 이름·통화·현재가가 채워진 객체를 요구한다. 관심종목·보유
 * 화면은 목록이 이미 그 값을 들고 있어 문제가 없었지만, 홈은 그렇지 않다 — 점수 랭킹은
 * 종가만, 뉴스 브리핑의 언급 종목은 **심볼과 이름뿐**이다. 그래서 화면마다 "시세를
 * 어디서 구해 오나"를 다시 짜게 되고, 그 결과 같은 종목이 화면마다 다른 정보로 열린다.
 *
 * 여기서 한 번만 해결한다 — 목록이 시세를 이미 들고 있으면 `seed`로 넘겨 **요청 없이**
 * 열고, 없으면 배치 시세를 **열 때 한 번** 부른다. 점수도 마찬가지로 열린 종목 하나만
 * 부른다. 목록에 스무 줄이 있어도 요청은 연 것 하나뿐이다.
 */
export function SymbolQuickLookModal({
  symbol,
  seed,
  onClose,
}: {
  /** null이면 닫힌 상태. */
  symbol: string | null;
  /** 목록이 이미 들고 있는 시세. 심볼이 일치할 때만 쓴다. */
  seed?: QuoteSummary | null;
  onClose: () => void;
}) {
  const hasSeed = seed != null && seed.symbol === symbol;
  const [fetched, setFetched] = useState<{ symbol: string; quote: QuoteSummary | null } | null>(
    null,
  );

  useEffect(() => {
    if (!symbol || hasSeed) return;
    let alive = true;
    fetchQuotesBatchClient([symbol])
      .then((rows) => {
        if (alive) setFetched({ symbol, quote: rows[0] ?? null });
      })
      .catch(() => {
        // 시세를 못 받으면 모달은 로딩 문구에 머문다 — 0원짜리 종목을 그리지 않는다.
        if (alive) setFetched({ symbol, quote: null });
      });
    return () => {
      alive = false;
    };
  }, [symbol, hasSeed]);

  const symbols = useMemo(() => (symbol ? [symbol] : []), [symbol]);
  const scores = useStockScores(symbols);

  const quote = hasSeed
    ? seed
    : fetched?.symbol === symbol
      ? fetched.quote
      : null;

  const stock: QuickLookStock | null =
    symbol && quote
      ? {
          symbol,
          name: quote.name || symbol,
          currency: quote.currency,
          assetType: quote.assetType,
          marketLabel: quote.market,
          currentPrice: quote.currentPrice,
          change: quote.change,
          changeRate: quote.changeRate,
          hasQuote: quote.currentPrice > 0,
          // 52주 값은 관심종목 응답에만 실린다. 없으면 레인지 블록 자체가 뜨지 않는다.
          week52Low: quote.watchlistMetrics?.week52Low ?? null,
          week52High: quote.watchlistMetrics?.week52High ?? null,
          changeRateWeek52Low: quote.watchlistMetrics?.changeRateWeek52Low ?? null,
          changeRateWeek52High: quote.watchlistMetrics?.changeRateWeek52High ?? null,
          isHeld: quote.isHeld,
        }
      : null;

  return (
    <StockQuickLookModal
      // 종목이 바뀌면 모달 내부의 차트·뉴스 상태를 초기화한다.
      key={symbol ?? "none"}
      stock={stock}
      score={symbol ? (scores.get(symbol.toUpperCase()) ?? null) : null}
      open={Boolean(symbol)}
      onClose={onClose}
    />
  );
}

/**
 * 종목을 여는 **한 가지** 방법 — 목록·레일 카드가 공유한다.
 *
 * 지금까지 레일 카드들은 종목 줄에 `/analysis` 링크를 걸었다. 옆 칸을 곁눈질하러 온
 * 사람을 화면 밖으로 내보내는 셈이라, 관심종목 셋을 견주려면 뒤로가기를 세 번 눌러야
 * 했다 — 홈이 08-16에 고친 것과 **같은 문제**가 레일에 남아 있었다.
 *
 * 「그 화면으로 가기」는 사라지지 않는다. 퀵룩 모달 바닥의 「종목 분석 보기」가 그 자리다.
 */
export function useSymbolQuickLook(): {
  symbol: string | null;
  /** `seed`를 주면 시세 요청 없이 연다. */
  open: (symbol: string, seed?: QuoteSummary | null) => void;
  close: () => void;
  modal: React.ReactNode;
} {
  const [picked, setPicked] = useState<{ symbol: string; seed: QuoteSummary | null } | null>(null);

  const open = useCallback((symbol: string, seed?: QuoteSummary | null) => {
    setPicked({ symbol, seed: seed ?? null });
  }, []);
  const close = useCallback(() => setPicked(null), []);

  return {
    symbol: picked?.symbol ?? null,
    open,
    close,
    modal: (
      <SymbolQuickLookModal symbol={picked?.symbol ?? null} seed={picked?.seed} onClose={close} />
    ),
  };
}
