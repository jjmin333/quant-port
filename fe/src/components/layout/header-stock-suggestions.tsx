"use client";

import { useEffect, useState, type RefObject } from "react";
import { createPortal } from "react-dom";
import { StockAvatar } from "@/components/stocks/stock-avatar";
import { cn } from "@/lib/utils";
import type { StockInfo } from "@/lib/quotes-types";

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
  const active = open && query.trim().length > 0;

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
}: HeaderStockSuggestionsProps) {
  const trimmedQuery = query.trim();
  const position = useSuggestionPanelPosition(anchorRef, open, trimmedQuery);

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
      }}
      className="fixed z-[200] overflow-hidden rounded-lg border border-[var(--border)] bg-[var(--surface)] shadow-lg"
    >
      {loading && items.length === 0 ? (
        <p className="px-3 py-2.5 text-xs text-[var(--text-muted)]">검색 중…</p>
      ) : items.length === 0 ? (
        <p className="px-3 py-2.5 text-xs text-[var(--text-muted)]">검색 결과가 없습니다.</p>
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
                  "flex w-full items-center gap-2 px-3 py-2 text-left text-sm transition-colors",
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
                <span className="shrink-0 font-mono text-xs font-semibold text-[var(--brand)]">
                  {item.symbol}
                </span>
                <span className="min-w-0 truncate">{item.name}</span>
                {item.market && (
                  <span className="ml-auto shrink-0 text-[10px] text-[var(--text-muted)]">
                    {item.market}
                  </span>
                )}
              </button>
            </li>
          ))}
        </ul>
      )}
    </div>,
    document.body,
  );
}
