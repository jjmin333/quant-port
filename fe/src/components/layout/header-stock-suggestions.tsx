"use client";

import { useEffect, useState, type RefObject } from "react";
import { createPortal } from "react-dom";
import { StockAvatar } from "@/components/stocks/stock-avatar";
import { fetchQuotesBatchClient } from "@/lib/market-api";
import { formatPercent } from "@/lib/format";
import { trendTextClass } from "@/lib/trend-colors";
import { cn } from "@/lib/utils";
import { PriceDisplay } from "@/components/ui/price-display";
import type { QuoteSummary, StockInfo } from "@/lib/quotes-types";

type HeaderStockSuggestionsProps = {
  anchorRef: RefObject<HTMLElement | null>;
  open: boolean;
  items: StockInfo[];
  loading: boolean;
  query: string;
  highlightedIndex: number;
  onHighlight: (index: number) => void;
  onSelect: (item: StockInfo) => void;
  listboxId?: string;
  /** 목록 위 한 줄 (예: 「최근 본 종목」). 없으면 그리지 않는다. */
  heading?: string;
  /** 결과 0건일 때의 문구. 「최근 본 종목」 패널에서는 안내가 달라야 한다. */
  emptyText?: string;
};

type PanelPosition = {
  top: number;
  left: number;
  width: number;
};

function useSuggestionPanelPosition(
  anchorRef: RefObject<HTMLElement | null>,
  open: boolean,
  query: string,
): PanelPosition | null {
  const [position, setPosition] = useState<PanelPosition | null>(null);
  // 검색어가 없어도 열릴 수 있다 — 「최근 본 종목」이 빈 입력에서 뜬다(§A-4).
  // 열지 말지는 호출부의 `open`이 이미 판단한다.
  const active = open;

  useEffect(() => {
    if (!active) return;

    const update = () => {
      const anchor = anchorRef.current;
      if (!anchor) return;
      const rect = anchor.getBoundingClientRect();
      setPosition({
        top: rect.bottom + 4,
        left: rect.left,
        width: rect.width,
      });
    };

    update();
    window.addEventListener("resize", update);
    window.addEventListener("scroll", update, true);
    return () => {
      window.removeEventListener("resize", update);
      window.removeEventListener("scroll", update, true);
    };
  }, [anchorRef, active]);

  // 닫혀 있을 때 좌표를 비우는 건 상태가 아니라 반환값으로 한다 — 이펙트에서 지우면
  // 렌더가 한 번 더 돈다. 좌표는 검색창(고정 헤더) 것이라 다시 열어도 그대로 맞는다.
  return active ? position : null;
}

/**
 * 목록에 뜬 종목의 시세를 심볼로 색인해 돌려준다.
 *
 * 자동완성이 그리는 종목 수는 8개 안팎이라 `/quotes/batch` 한 번이면 끝난다
 * (종목마다 부르지 않는다 — 목록 모드라 서버도 캐시·DB만 읽는다). 시세가 늦거나
 * 실패해도 목록은 그대로 뜬다: 이름·코드는 이미 있고, 가격 칸만 비워 둔다.
 */
function useSuggestionQuotes(items: StockInfo[], enabled: boolean): Record<string, QuoteSummary> {
  const [bySymbol, setBySymbol] = useState<Record<string, QuoteSummary>>({});
  // 심볼 목록이 실제로 바뀔 때만 다시 부르기 위해 문자열로 고정한다.
  const key = enabled ? items.map((i) => i.symbol).join(",") : "";

  useEffect(() => {
    if (!key) return;
    let alive = true;
    fetchQuotesBatchClient(key.split(","))
      .then((rows) => {
        if (!alive) return;
        setBySymbol((prev) => ({
          ...prev,
          ...Object.fromEntries(rows.map((q) => [q.symbol.toUpperCase(), q])),
        }));
      })
      .catch(() => {
        // 시세가 없어도 이름·코드는 그대로 보여준다.
      });
    return () => {
      alive = false;
    };
  }, [key]);

  return bySymbol;
}

/**
 * 행 오른쪽의 현재가·등락률.
 *
 * 시세가 아직 안 왔으면 **자리만 비워 둔다**(`min-w`). 「—」를 넣었다가 값으로
 * 바꾸면 목록 여덟 줄이 한 번에 흔들린다 — 폭을 미리 잡아 두면 값만 채워진다.
 */
function SuggestionPrice({ quote }: { quote?: QuoteSummary }) {
  const hasPrice = !!quote && quote.currentPrice > 0;
  return (
    <span className="flex min-w-[76px] shrink-0 flex-col items-end">
      <span className="text-note font-medium leading-tight tabular-nums text-[var(--text-primary)]">
        {hasPrice ? (
          <PriceDisplay value={quote.currentPrice} currency={quote.currency} size="inherit" />
        ) : null}
      </span>
      <span
        className={cn(
          "text-micro leading-tight tabular-nums",
          // 0%는 방향이 없다 — 색을 칠하면 「올랐다」로 읽힌다(지수 칩과 같은 규칙).
          hasPrice && quote.changeRate !== 0
            ? trendTextClass(quote.changeRate)
            : "text-[var(--text-muted)]",
        )}
      >
        {hasPrice ? formatPercent(quote.changeRate) : ""}
      </span>
    </span>
  );
}

export function HeaderStockSuggestions({
  anchorRef,
  open,
  items,
  loading,
  query,
  highlightedIndex,
  onHighlight,
  onSelect,
  listboxId = "header-stock-suggestions",
  heading,
  emptyText = "검색 결과가 없습니다.",
}: HeaderStockSuggestionsProps) {
  const trimmedQuery = query.trim();
  const position = useSuggestionPanelPosition(anchorRef, open, trimmedQuery);
  const quotes = useSuggestionQuotes(items, open);

  // `position`은 이펙트에서만 채워지므로 서버 렌더에서는 항상 null이다. 별도의
  // `mounted` 플래그 없이도 포털이 하이드레이션 전에 그려지지 않는다.
  if (!position) return null;

  return createPortal(
    <div
      id={listboxId}
      role="listbox"
      aria-label="종목 검색 결과"
      style={{
        top: position.top,
        left: position.left,
        width: position.width,
        /*
          `body`가 `pointer-events: none`이어도 이 패널만은 클릭을 받는다.

          모달(Radix Dialog)이 열리면 `react-remove-scroll`이 **`document.body`에
          `pointer-events: none`을 건다**. 이 패널은 `createPortal(document.body)`로
          나가므로 그 값을 물려받아, 목록이 멀쩡히 보이는데 클릭만 먹지 않았다.
          실측: 옵션 중앙 좌표의 `document.elementFromPoint()`가 옵션이 아니라
          다이얼로그 오버레이를 돌려줬고, 「스탬프 찍기」·「종목 일괄 추가」에서
          마우스로는 종목을 고를 수 없었다(키보드 ↓+Enter로만 가능).

          포털을 걷어내고 입력칸 아래에 절대배치하는 방법도 검토했지만, 두 모달 모두
          `overflow-y-auto` / `overflow-hidden`이라 목록이 잘린다 — 포털이 있는 이유가
          그것이다. 그래서 포털은 두고 이 한 줄로 상속만 끊는다.
        */
        pointerEvents: "auto",
      }}
      /*
        Radix의 DismissableLayer는 문서 레벨 `pointerdown`을 보고 「바깥 클릭」을
        판정한다. 이 패널은 DOM 상 다이얼로그 바깥이므로, 여기서 전파를 끊지 않으면
        종목을 고르는 순간 다이얼로그가 닫힌다.
      */
      onPointerDown={(e) => e.stopPropagation()}
      onMouseDown={(e) => e.stopPropagation()}
      className="fixed z-[200] overflow-hidden rounded-lg border border-[var(--border)] bg-[var(--surface)] shadow-lg"
    >
      {heading && items.length > 0 && (
        <p className="border-b border-[var(--border)] px-3 py-1.5 text-micro font-medium text-[var(--text-muted)]">
          {heading}
        </p>
      )}
      {loading && items.length === 0 ? (
        <p className="px-3 py-2.5 text-cell text-[var(--text-muted)]">검색 중…</p>
      ) : items.length === 0 ? (
        <p className="px-3 py-2.5 text-cell text-[var(--text-muted)]">{emptyText}</p>
      ) : (
        <ul className="py-1">
          {items.map((item, index) => (
            <li key={item.symbol} role="presentation">
              <button
                type="button"
                role="option"
                aria-selected={index === highlightedIndex}
                onMouseEnter={() => onHighlight(index)}
                onMouseDown={(e) => e.preventDefault()}
                onClick={() => onSelect(item)}
                className={cn(
                  "flex w-full items-center gap-2 px-3 py-1 text-left transition-colors",
                  index === highlightedIndex
                    ? "bg-[var(--brand-muted)] text-[var(--text-primary)]"
                    : "text-[var(--text-secondary)] hover:bg-[var(--surface-elevated)] hover:text-[var(--text-primary)]",
                )}
              >
                <StockAvatar
                  symbol={item.symbol}
                  name={item.name}
                  currency={item.currency}
                  assetType={item.assetType}
                  size="xs"
                />
                {/*
                  이름이 위, 종목코드가 그 아래 작게. 두 줄이지만 행 높이는 예전
                  한 줄짜리와 같다 — 세로 여백을 `py-2`에서 `py-1`로 줄이고 두 줄 모두
                  `leading-tight`로 눌렀다(실측 36 → 36.75px).
                */}
                <span className="flex min-w-0 flex-1 flex-col">
                  <span className="truncate text-note font-medium leading-tight">
                    {item.name}
                  </span>
                  <span className="truncate font-num text-micro leading-tight text-[var(--text-muted)]">
                    {item.symbol}
                  </span>
                </span>
                <SuggestionPrice quote={quotes[item.symbol.toUpperCase()]} />
              </button>
            </li>
          ))}
        </ul>
      )}
    </div>,
    document.body,
  );
}
