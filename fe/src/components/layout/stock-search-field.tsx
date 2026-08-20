"use client";

import { useRef } from "react";
import { Search, X } from "lucide-react";
import { HeaderStockSuggestions } from "@/components/layout/header-stock-suggestions";
import { useHeaderSearch } from "@/components/providers/header-search-provider";
import { supportsStockSuggestions } from "@/lib/header-search-config";
import { cn } from "@/lib/utils";

type StockSearchFieldProps = {
  compact?: boolean;
  className?: string;
  suggestionsId?: string;
};

export function StockSearchField({
  compact = false,
  className,
  suggestionsId = "header-stock-suggestions",
}: StockSearchFieldProps) {
  const {
    config,
    inputValue,
    setInputValue,
    clearSearch,
    submitSearch,
    stockSuggestions,
    suggestionsLoading,
    suggestionsOpen,
    highlightedIndex,
    setHighlightedIndex,
    selectStockSuggestion,
  } = useHeaderSearch();
  const searchAnchorRef = useRef<HTMLDivElement>(null);

  const showClearButton = inputValue.trim().length > 0;
  const showStockSuggestions = suggestionsOpen;
  // 자동완성이 없는 화면(뉴스·거래내역 필터)에서는 combobox가 아니라 그냥 검색창이다 —
  // `searchbox`는 `aria-expanded`를 지원하지 않으므로 역할과 속성을 함께 바꾼다.
  const isCombobox = supportsStockSuggestions(config.mode);

  return (
    <div ref={searchAnchorRef} className={cn("relative w-full", className)}>
      <Search
        className={cn(
          "pointer-events-none absolute left-2.5 top-1/2 -translate-y-1/2 text-[var(--text-muted)]",
          compact ? "h-3.5 w-3.5" : "h-4 w-4 left-3",
        )}
      />
      <input
        type="text"
        role={isCombobox ? "combobox" : "searchbox"}
        value={inputValue}
        onChange={(e) => setInputValue(e.target.value)}
        onKeyDown={(e) => {
          if (showStockSuggestions && stockSuggestions.length > 0) {
            if (e.key === "ArrowDown") {
              e.preventDefault();
              setHighlightedIndex((highlightedIndex + 1) % stockSuggestions.length);
              return;
            }
            if (e.key === "ArrowUp") {
              e.preventDefault();
              setHighlightedIndex(
                (highlightedIndex - 1 + stockSuggestions.length) % stockSuggestions.length,
              );
              return;
            }
          }
          if (e.key === "Enter") {
            e.preventDefault();
            submitSearch();
          }
          if (e.key === "Escape") {
            clearSearch();
          }
        }}
        placeholder={config.placeholder}
        aria-label={config.placeholder}
        aria-expanded={isCombobox ? showStockSuggestions : undefined}
        aria-autocomplete={isCombobox ? "list" : undefined}
        aria-controls={showStockSuggestions ? suggestionsId : undefined}
        className={cn(
          "w-full rounded-lg border border-[var(--border)] bg-[var(--surface)] text-[var(--text-primary)] placeholder:text-[var(--text-muted)] focus:border-[var(--brand)] focus:outline-none",
          compact
            ? "h-8 pl-8 pr-8 text-xs"
            : "h-9 min-w-[12rem] pl-9 pr-9 text-sm",
        )}
      />
      {showClearButton && (
        <button
          type="button"
          onClick={clearSearch}
          aria-label="검색어 지우기"
          className={cn(
            "absolute top-1/2 flex -translate-y-1/2 items-center justify-center rounded text-[var(--text-muted)] hover:bg-[var(--surface-elevated)] hover:text-[var(--text-primary)]",
            compact ? "right-1.5 h-5 w-5" : "right-2 h-6 w-6",
          )}
        >
          <X className={compact ? "h-3 w-3" : "h-3.5 w-3.5"} />
        </button>
      )}
      {showStockSuggestions && (
        <HeaderStockSuggestions
          anchorRef={searchAnchorRef}
          open={showStockSuggestions}
          items={stockSuggestions}
          loading={suggestionsLoading}
          query={inputValue}
          highlightedIndex={highlightedIndex}
          onHighlight={setHighlightedIndex}
          onSelect={selectStockSuggestion}
          listboxId={suggestionsId}
        />
      )}
    </div>
  );
}
