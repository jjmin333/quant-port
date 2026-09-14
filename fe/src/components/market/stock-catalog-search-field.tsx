"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { Search } from "lucide-react";
import { HeaderStockSuggestions } from "@/components/layout/header-stock-suggestions";
import { searchStocksCatalogClient } from "@/lib/market-api";
import type { StockInfo } from "@/lib/quotes-types";
import { cn } from "@/lib/utils";

const SUGGEST_LIMIT = 8;
const DEBOUNCE_MS = 200;

type StockCatalogSearchFieldProps = {
  placeholder?: string;
  onSelect: (stock: StockInfo) => void;
  disabled?: boolean;
  className?: string;
  inputClassName?: string;
  suggestionsId?: string;
  autoFocus?: boolean;
};

export function StockCatalogSearchField({
  placeholder = "종목명·티커 검색 (예: 삼성전자, NVDA)",
  onSelect,
  disabled = false,
  className,
  inputClassName,
  suggestionsId = "stock-catalog-suggestions",
  autoFocus = false,
}: StockCatalogSearchFieldProps) {
  const anchorRef = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLInputElement>(null);
  const [query, setQuery] = useState("");
  const [items, setItems] = useState<StockInfo[]>([]);
  const [loading, setLoading] = useState(false);
  const [open, setOpen] = useState(false);
  const [highlightedIndex, setHighlightedIndex] = useState(0);

  useEffect(() => {
    if (autoFocus && !disabled) {
      inputRef.current?.focus();
    }
  }, [autoFocus, disabled]);

  useEffect(() => {
    const trimmed = query.trim();
    if (trimmed.length < 1) {
      setItems([]);
      setOpen(false);
      return;
    }

    const timer = window.setTimeout(() => {
      setLoading(true);
      searchStocksCatalogClient(trimmed, "", SUGGEST_LIMIT)
        .then(({ items: found }) => {
          setItems(found);
          setOpen(true);
          setHighlightedIndex(0);
        })
        .catch(() => {
          setItems([]);
          setOpen(false);
        })
        .finally(() => setLoading(false));
    }, DEBOUNCE_MS);

    return () => window.clearTimeout(timer);
  }, [query]);

  const pick = useCallback(
    (stock: StockInfo) => {
      onSelect(stock);
      setQuery("");
      setItems([]);
      setOpen(false);
    },
    [onSelect],
  );

  return (
    <div ref={anchorRef} className={cn("relative min-w-0 flex-1", className)}>
      <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-[var(--text-muted)]" />
      <input
        ref={inputRef}
        type="text"
        value={query}
        disabled={disabled}
        placeholder={placeholder}
        aria-label={placeholder}
        aria-expanded={open}
        aria-autocomplete="list"
        aria-controls={open ? suggestionsId : undefined}
        onChange={(e) => setQuery(e.target.value)}
        onFocus={() => {
          if (query.trim() && items.length > 0) setOpen(true);
        }}
        onKeyDown={(e) => {
          if (open && items.length > 0) {
            if (e.key === "ArrowDown") {
              e.preventDefault();
              setHighlightedIndex((i) => (i + 1) % items.length);
              return;
            }
            if (e.key === "ArrowUp") {
              e.preventDefault();
              setHighlightedIndex((i) => (i - 1 + items.length) % items.length);
              return;
            }
            if (e.key === "Enter") {
              e.preventDefault();
              const item = items[highlightedIndex];
              if (item) pick(item);
              return;
            }
          }
          if (e.key === "Escape") {
            setOpen(false);
          }
        }}
        className={cn(
          // 모바일 글자만 16px — 그 아래로 내려가면 iOS가 포커스 시 화면을 확대한다
          // (`stock-search-field.tsx`와 같은 이유).
          "w-full rounded-lg border border-[var(--border)] bg-[var(--surface)] py-2 pl-9 pr-3 text-base text-[var(--text-primary)] placeholder:text-[var(--text-muted)] focus:border-[var(--brand)] focus:outline-none disabled:opacity-50 md:text-sm",
          inputClassName,
        )}
      />
      <HeaderStockSuggestions
        anchorRef={anchorRef}
        open={open}
        items={items}
        loading={loading}
        query={query}
        highlightedIndex={highlightedIndex}
        onHighlight={setHighlightedIndex}
        onSelect={pick}
        listboxId={suggestionsId}
      />
    </div>
  );
}
