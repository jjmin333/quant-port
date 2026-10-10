"use client";

import { useCallback, useEffect, useState, useSyncExternalStore } from "react";
import { Briefcase, Heart, NotepadText, Target } from "lucide-react";
import { AddHoldingDialog } from "@/components/holdings/add-holding-dialog";
import { RankingMemoModal, type RankingMemoTarget } from "@/components/market-radar/market-radar-memo-modal";
import { StampButton } from "@/components/stamps/stamp-button";
import { HEADER_ACTION_CLASS } from "@/components/stocks/stock-symbol-action-bar";
import { TargetPriceDialog } from "@/components/targets/target-price-dialog";
import { PREDICTION_FEATURES_HIDDEN } from "@/lib/hidden-features";
import { Button } from "@/components/ui/button";
import { useLoginGate } from "@/hooks/use-login-gate";
import type { QuoteAssetType, QuoteCurrency, StockInfo } from "@/lib/quotes-types";
import {
  ensureWatchlist,
  isWatchlisted,
  subscribeWatchlist,
  toggleWatchlistSymbol,
} from "@/lib/watchlist-store";
import { cn } from "@/lib/utils";

/**
 * 종목 퀵룩 모달의 **동작 줄** — 관심 · 메모장 · 보유 · 스탬프 · 목표가
 * (2026-09-05 사용자 요청 7-4).
 *
 * # 왜 창 안에 넣는가
 *
 * 이 다섯은 「이 종목에 무언가를 붙이는」 동작인데, 지금까지는 **화면마다 흩어져**
 * 있었다: 관심·메모는 랭킹 목록의 행에만, 스탬프·보유·목표가는 종목 상세 헤더에만.
 * 그래서 뉴스나 홈에서 종목을 눌러 창을 연 사람은 다섯 중 무엇도 할 수 없었고,
 * 관심 하나 담자고 창을 닫고 랭킹으로 가야 했다.
 *
 * 창은 이미 그 종목의 시세·점수·일정·뉴스를 다 들고 있다. 붙이는 동작도 여기서 끝난다.
 *
 * # 순서는 사용자가 정했다
 *
 * 관심 → 메모장 → 보유 → 스탬프 → 목표가. 왼쪽이 가볍고(한 번 누르면 끝) 오른쪽으로
 * 갈수록 창을 열어 값을 적는 동작이다.
 *
 * # 로그인 규칙은 앱과 같다 (`hooks/use-login-gate` 머리말)
 *
 * - **창을 여는 넷**(메모·보유·스탬프·목표가)은 비로그인에게도 **열린다**. 막는 자리는
 *   창 안의 저장 버튼이고, 로그인으로 가는 길은 그 아래 한 줄이 연다.
 * - **관심**은 창이 없다 — 누르면 그 자리에서 서버에 쓴다. 그래서 여기서 `run()`으로
 *   감싸 로그인으로 보낸다. 이건 예전부터 랭킹 행의 하트가 하던 것과 같은 규칙이다.
 */
export type QuickLookActionStock = {
  symbol: string;
  name: string;
  currency: QuoteCurrency;
  assetType?: QuoteAssetType;
  /** 거래소 표기(KOSPI·NASDAQ…). 보유 등록 창이 종목 정보로 함께 저장한다. */
  marketLabel?: string;
  price?: number;
  week52High?: number | null;
  week52Low?: number | null;
};

/**
 * 관심 여부를 **앱 공용 저장소에서** 읽는다.
 *
 * `useState` + `useEffect`로 복사해 두면, 같은 종목을 다른 화면에서 담았을 때 이
 * 창의 하트만 옛 상태로 남는다. 저장소는 낙관적 반영과 되돌리기를 이미 하고 있으므로
 * (`lib/watchlist-store`) 여기서는 구독만 한다.
 *
 * 서버 스냅샷을 `false`로 두는 이유: 이 값은 로그인한 사용자마다 다르므로 서버가
 * 그릴 수 있는 답이 아니다. 첫 클라이언트 렌더에서 실제 값으로 채워진다.
 */
function useIsWatchlisted(symbol: string): boolean {
  const subscribe = useCallback((onChange: () => void) => subscribeWatchlist(onChange), []);
  const get = useCallback(() => isWatchlisted(symbol), [symbol]);
  return useSyncExternalStore(subscribe, get, () => false);
}

export function StockQuickLookActions({
  stock,
  className,
}: {
  stock: QuickLookActionStock;
  className?: string;
}) {
  const gate = useLoginGate("등록");
  const watchlisted = useIsWatchlisted(stock.symbol);
  const [watchlistBusy, setWatchlistBusy] = useState(false);
  const [memoTarget, setMemoTarget] = useState<RankingMemoTarget | null>(null);
  const [holdingOpen, setHoldingOpen] = useState(false);
  const [targetOpen, setTargetOpen] = useState(false);
  /*
    보유 등록·목표가 창이 쓸 가격은 **누른 순간에 얼린다** — 시세가 5초마다 갱신되므로
    창을 띄워 둔 채로 두면 적기 시작한 숫자가 발밑에서 바뀐다(종목 헤더와 같은 처리).
  */
  const [frozenPrice, setFrozenPrice] = useState<number | undefined>(undefined);

  /* 로그인했는데 아직 집합을 안 받았으면 여기서 한 번 채운다(중복 요청은 저장소가 접는다). */
  useEffect(() => {
    if (gate.authed) void ensureWatchlist();
  }, [gate.authed]);

  const info: StockInfo = {
    symbol: stock.symbol,
    name: stock.name,
    market: stock.marketLabel || (stock.currency === "USD" ? "US" : "KRX"),
    currency: stock.currency,
    assetType: stock.assetType,
  };

  const onToggleWatchlist = () => {
    if (watchlistBusy) return;
    gate.run(async () => {
      setWatchlistBusy(true);
      try {
        await toggleWatchlistSymbol(stock.symbol);
      } catch {
        // 되돌리기는 저장소가 끝냈다 — 여기서 더 할 말은 없다.
      } finally {
        setWatchlistBusy(false);
      }
    });
  };

  return (
    <div className={cn("flex items-center gap-1", className)}>
      <Button
        variant="outline"
        size="sm"
        onClick={onToggleWatchlist}
        disabled={watchlistBusy}
        aria-pressed={watchlisted}
        aria-label={watchlisted ? `${stock.name} 관심 해제` : `${stock.name} 관심 등록`}
        title={
          gate.authed
            ? watchlisted
              ? "관심종목에서 빼기"
              : "관심종목에 담기"
            : "로그인하면 관심종목에 담을 수 있습니다"
        }
        className={cn(
          HEADER_ACTION_CLASS,
          "px-2",
          // 담긴 상태는 **채운 하트와 색**으로 말한다 — 테두리만 바꾸면 좁은 창에서
          // 담긴 것과 안 담긴 것이 구분되지 않는다.
          watchlisted && "border-[var(--brand)]/40 text-[var(--brand)] hover:text-[var(--brand)]",
        )}
      >
        <Heart className={cn(watchlisted && "fill-current")} />
      </Button>

      <Button
        variant="outline"
        size="sm"
        onClick={() =>
          setMemoTarget({
            symbol: stock.symbol,
            name: stock.name,
            currency: stock.currency,
            assetType: stock.assetType,
            price: stock.price ?? 0,
            // 순위가 없는 자리다 — 부제는 「005930 · 종목」에서 멈춘다.
            originLabel: "종목",
          })
        }
        aria-label={`${stock.name} 메모 작성`}
        title="이 종목에 메모 쓰기"
        className={cn(HEADER_ACTION_CLASS, "px-2")}
      >
        <NotepadText />
      </Button>

      <Button
        variant="outline"
        size="sm"
        onClick={() => {
          setFrozenPrice(stock.price);
          setHoldingOpen(true);
        }}
        aria-label="보유종목 등록"
        title={
          gate.authed
            ? "이 종목을 내 포트폴리오에 등록"
            : "로그인하면 이 종목을 포트폴리오에 등록할 수 있습니다"
        }
        className={cn(HEADER_ACTION_CLASS, "px-2")}
      >
        <Briefcase />
      </Button>

      {/* 목표가 예측(스탬프·목표가) 비노출 — lib/hidden-features (2026-10-10). */}
      {!PREDICTION_FEATURES_HIDDEN && (
        <>
          <StampButton
            symbol={stock.symbol}
            price={stock.price}
            currency={stock.currency}
            week52High={stock.week52High ?? undefined}
            week52Low={stock.week52Low ?? undefined}
            iconOnly
          />

          <Button
            variant="outline"
            size="sm"
            onClick={() => {
              setFrozenPrice(stock.price);
              setTargetOpen(true);
            }}
            aria-label="목표가 설정"
            title={
              gate.authed ? "이 종목의 목표가 설정" : "로그인하면 이 종목의 목표가를 설정할 수 있습니다"
            }
            className={cn(HEADER_ACTION_CLASS, "px-2")}
          >
            <Target />
          </Button>
        </>
      )}

      <RankingMemoModal item={memoTarget} onClose={() => setMemoTarget(null)} />
      <AddHoldingDialog
        open={holdingOpen}
        onOpenChange={setHoldingOpen}
        onAdded={() => setHoldingOpen(false)}
        stock={info}
        price={frozenPrice}
      />
      <TargetPriceDialog
        open={targetOpen}
        onOpenChange={setTargetOpen}
        stock={info}
        currentPrice={frozenPrice}
      />
    </div>
  );
}
