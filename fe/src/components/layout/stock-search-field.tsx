"use client";

import { useCallback, useRef, useState } from "react";
import { Search, X } from "lucide-react";
import { useRouter } from "next/navigation";
import { HeaderStockSuggestions } from "@/components/layout/header-stock-suggestions";
import { useHeaderSearch } from "@/components/providers/header-search-provider";
import { supportsStockSuggestions } from "@/lib/header-search-config";
import { recentStocksPanelOpen, shouldBlurAfterPick } from "@/lib/header-search-panel";
import { recentStocks } from "@/lib/recent-stocks";
import type { StockInfo } from "@/lib/quotes-types";
import { cn } from "@/lib/utils";

type StockSearchFieldProps = {
  compact?: boolean;
  className?: string;
  suggestionsId?: string;
  /** 접혀 있던 검색을 펼쳤을 때 — 눌러서 연 입력에 커서까지 가 있어야 한 동작이 된다. */
  autoFocus?: boolean;
};

export function StockSearchField({
  compact = false,
  className,
  suggestionsId = "header-stock-suggestions",
  autoFocus = false,
}: StockSearchFieldProps) {
  const {
    config,
    inputValue,
    setInputValue,
    clearSearch,
    submitSearch,
    stockSuggestions,
    assetSuggestions,
    suggestionsLoading,
    suggestionsOpen,
    highlightedIndex,
    setHighlightedIndex,
    selectStockSuggestion,
  } = useHeaderSearch();
  const router = useRouter();
  const searchAnchorRef = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLInputElement>(null);

  /*
    빈 검색창에 **최근 본 종목**을 띄운다 (§A-4).

    예전에는 포커스만 하면 아무것도 없었다. 사람이 매일 보는 종목은 몇 개뿐인데 매번
    이름을 처음부터 다시 쳐야 했다. 빈 상태를 「지금 필요할 만한 것」으로 채우는 것이
    검색창이 할 수 있는 가장 값싼 일이다.

    포커스 시점에만 읽는다 — 렌더마다 localStorage를 건드릴 이유가 없고, 목록은
    포커스와 포커스 사이에만 바뀐다.
  */
  const [focused, setFocused] = useState(false);
  const [recents, setRecents] = useState<StockInfo[]>([]);

  const showClearButton = inputValue.trim().length > 0;
  const showStockSuggestions = suggestionsOpen;
  const showRecents = recentStocksPanelOpen({
    mode: config.mode,
    focused,
    inputValue,
    recentCount: recents.length,
  });

  /*
    종목을 고르면 **입력에서 포커스를 뗀다.**

    옵션 버튼이 `onMouseDown`에서 기본동작을 막고 있어(그래야 클릭 전에 패널이 닫히지
    않는다) 입력은 포커스를 쥔 채로 남는다. `selectStockSuggestion`이 검색어를 비우면
    자동완성은 닫히지만 「빈 입력 + 포커스」는 곧 **「최근 본 종목」의 조건**이라, 방금
    고른 종목이 맨 위에 얹힌 목록이 같은 자리에 즉시 다시 떴다. 화면이 이동해도 그
    패널은 그대로 떠 있었고, 모바일에서는 자판까지 올라온 채였다.

    포커스를 떼는 일은 여기서만 할 수 있다 — 입력 요소를 쥔 것이 이 컴포넌트다.
  */
  const pickStock = useCallback(
    (item: StockInfo) => {
      if (shouldBlurAfterPick(config.mode)) {
        inputRef.current?.blur();
        setFocused(false);
      }
      selectStockSuggestion(item);
    },
    [config.mode, selectStockSuggestion],
  );
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
        ref={inputRef}
        type="text"
        role={isCombobox ? "combobox" : "searchbox"}
        autoFocus={autoFocus}
        value={inputValue}
        onChange={(e) => setInputValue(e.target.value)}
        onFocus={() => {
          setRecents(
            recentStocks().map((r) => ({
              symbol: r.symbol,
              name: r.name,
              market: r.market,
              currency: r.currency,
              assetType: r.assetType,
            })) as StockInfo[],
          );
          setFocused(true);
        }}
        onBlur={() => setFocused(false)}
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
            // 엔터도 종목을 고르는 길이다 — 마우스 클릭과 같은 이유로 포커스를 뗀다.
            if (shouldBlurAfterPick(config.mode)) {
              inputRef.current?.blur();
              setFocused(false);
            }
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
          /*
            모바일 글자만 `text-base`(16px)로 둔다 — 16px 미만인 입력에 포커스하면 iOS
            사파리가 그 칸을 읽을 만한 크기로 **페이지 전체를 확대하고**, 되돌리는 길은
            사용자가 직접 핀치 아웃하는 것뿐이라 검색을 누를 때마다 화면이 어긋난 채
            남는다. viewport에 `maximum-scale=1`을 박는 우회는 확대 자체를 막아
            버리므로 쓰지 않는다. `ui/input.tsx`가 같은 이유로 같은 짝을 쓴다.

            높이는 38px — 32px에서 약 20% 키웠다. 좁은 화면에서 검색은 이 화면의
            부속이 아니라 들어오는 문인데, 헤더에서 가장 자주 눌리는 칸이 가장 작은
            손가락 표적이었다. 아이콘과 지우기 버튼은 `top-1/2`로 잡혀 있어 높이를
            바꿔도 따라 내려온다.
          */
          compact
            ? "h-[38px] pl-8 pr-8 text-base md:text-cell"
            : "h-9 min-w-[12rem] pl-9 pr-9 text-base md:text-sm",
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
          onSelect={pickStock}
          listboxId={suggestionsId}
          assets={assetSuggestions}
          onAssetSelect={(a) => {
            clearSearch();
            inputRef.current?.blur();
            router.push(a.href);
          }}
        />
      )}
      {/*
        검색어를 치기 전의 빈 패널. 자동완성과 동시에 뜨지 않는다 — 입력이 있으면
        위쪽 패널이, 없으면 이쪽이 뜬다.
      */}
      {isCombobox && !showStockSuggestions && showRecents && (
        <HeaderStockSuggestions
          anchorRef={searchAnchorRef}
          open
          items={recents}
          loading={false}
          query=""
          heading="최근 본 종목"
          highlightedIndex={-1}
          onHighlight={() => {}}
          onSelect={pickStock}
          listboxId={`${suggestionsId}-recent`}
        />
      )}
    </div>
  );
}
