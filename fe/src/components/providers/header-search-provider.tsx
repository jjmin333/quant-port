"use client";

import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from "react";
import { usePathname, useRouter } from "next/navigation";
import {
  parseStockSearchSymbol,
  resolveHeaderSearchConfig,
  supportsStockSuggestions,
  type HeaderSearchConfig,
  type HeaderSearchMode,
} from "@/lib/header-search-config";
import { stockSuggestionsOpen } from "@/lib/header-search-panel";
import { analysisSymbolHref } from "@/lib/analysis-tabs";
import { searchStocksCatalogClient } from "@/lib/market-api";
import { rememberStock } from "@/lib/recent-stocks";
import type { StockInfo } from "@/lib/quotes-types";
import { fetchAssetSearch, type AssetSearchItem } from "@/lib/asset-search";

const STOCK_SUGGEST_LIMIT = 8;
const STOCK_SUGGEST_DEBOUNCE_MS = 200;

type SuggestResult = { key: string; items: StockInfo[] };

const EMPTY_ITEMS: StockInfo[] = [];
const EMPTY_ASSETS: AssetSearchItem[] = [];
const EMPTY_SUGGEST: SuggestResult = { key: "", items: EMPTY_ITEMS };

type HeaderSearchContextValue = {
  config: HeaderSearchConfig;
  mode: HeaderSearchMode;
  inputValue: string;
  setInputValue: (value: string) => void;
  /** Debounced query consumed by page components */
  activeQuery: string;
  isSearchActive: boolean;
  clearSearch: () => void;
  submitSearch: () => void;
  stockSuggestions: StockInfo[];
  /** 같은 검색어의 아파트·금융상품·금리·지역(2026-10-09). 종목 아래 한 무리로 뜬다. */
  assetSuggestions: AssetSearchItem[];
  suggestionsLoading: boolean;
  suggestionsOpen: boolean;
  highlightedIndex: number;
  setHighlightedIndex: (index: number) => void;
  selectStockSuggestion: (item: StockInfo) => void;
  registerStockPickHandler: (handler: ((symbol: string) => void) | null) => void;
};

const HeaderSearchContext = createContext<HeaderSearchContextValue | null>(null);

export function HeaderSearchProvider({ children }: { children: ReactNode }) {
  const pathname = usePathname();
  const router = useRouter();
  const config = useMemo(() => resolveHeaderSearchConfig(pathname), [pathname]);
  const [inputValue, setInputValue] = useState("");
  const [debouncedQuery, setDebouncedQuery] = useState("");
  const [suggestResult, setSuggestResult] = useState<SuggestResult>(EMPTY_SUGGEST);
  const [assetResult, setAssetResult] = useState<{ key: string; items: AssetSearchItem[] }>({ key: "", items: [] });
  const [highlightedIndex, setHighlightedIndex] = useState(0);

  /*
   * 화면 안 필터로 나가는 검색어. 입력이 비었거나 검색바가 없는 화면이면 곧바로 빈 값이고,
   * 그 외에는 디바운스가 끝난 마지막 값이다. 이펙트에서 지우면 한 프레임 동안 이전
   * 검색어가 새 화면에 남는다.
   */
  const activeQuery =
    config.mode === "hidden" || !inputValue.trim() ? "" : debouncedQuery;

  /*
   * 자동완성 결과는 **어떤 입력에 대한 답인지**(`key`)를 데이터와 함께 들고 다닌다.
   * 로딩 boolean을 이펙트에서 켜면 렌더가 한 번 더 돌면서 이전 검색어의 결과가
   * 새 검색어 밑에 잠깐 남는다. `key`가 어긋나면 그냥 로딩이다.
   */
  const suggestQuery = supportsStockSuggestions(config.mode) ? inputValue.trim() : "";
  const stockSuggestions = suggestResult.key === suggestQuery ? suggestResult.items : EMPTY_ITEMS;
  const assetSuggestions = assetResult.key === suggestQuery ? assetResult.items : EMPTY_ASSETS;
  const suggestionsLoading = suggestQuery.length > 0 && suggestResult.key !== suggestQuery;
  const stockSuggestionsRef = useRef<StockInfo[]>([]);
  const stockPickHandlerRef = useRef<((symbol: string) => void) | null>(null);

  useEffect(() => {
    stockSuggestionsRef.current = stockSuggestions;
  }, [stockSuggestions]);

  // 화면이 바뀌면 검색창을 비운다. 이펙트가 아니라 렌더 중 조정이라, 새 화면의 첫 프레임에
  // 이전 화면의 검색어가 남지 않는다.
  const routeKey = `${pathname}\u0000${config.mode}`;
  const [prevRouteKey, setPrevRouteKey] = useState(routeKey);
  if (prevRouteKey !== routeKey) {
    setPrevRouteKey(routeKey);
    if (inputValue) setInputValue("");
    if (highlightedIndex) setHighlightedIndex(0);
  }

  useEffect(() => {
    const trimmed = inputValue.trim();
    if (!trimmed) return;
    const timer = window.setTimeout(() => {
      setDebouncedQuery(trimmed);
    }, config.debounceMs);
    return () => window.clearTimeout(timer);
  }, [inputValue, config.debounceMs]);

  useEffect(() => {
    if (!suggestQuery) return;

    let cancelled = false;
    const timer = window.setTimeout(() => {
      // 자산 칸은 종목과 따로 온다 — 늦거나 실패해도 종목 목록을 붙잡지 않는다.
      void fetchAssetSearch(suggestQuery).then((items) => {
        if (!cancelled) setAssetResult({ key: suggestQuery, items });
      });
      searchStocksCatalogClient(suggestQuery, "", STOCK_SUGGEST_LIMIT)
        .then(({ items }) => {
          if (!cancelled) {
            setSuggestResult({ key: suggestQuery, items });
            setHighlightedIndex(0);
          }
        })
        .catch(() => {
          if (!cancelled) setSuggestResult({ key: suggestQuery, items: [] });
        });
    }, STOCK_SUGGEST_DEBOUNCE_MS);

    return () => {
      cancelled = true;
      window.clearTimeout(timer);
    };
  }, [suggestQuery]);

  const clearSearch = useCallback(() => {
    setInputValue("");
    setHighlightedIndex(0);
  }, []);

  const registerStockPickHandler = useCallback((handler: ((symbol: string) => void) | null) => {
    stockPickHandlerRef.current = handler;
  }, []);

  /**
   * 종목을 고르면 **AI 추천 랭킹**으로 보낸다.
   *
   * 예전에는 `/stocks/{symbol}` 워크스페이스로 갔다. 검색으로 종목을 찾는 사람이 처음
   * 알고 싶은 건 호가창이 아니라 "이 종목이 지금 어떤가"라서, 랭킹 화면의 AI 분석 탭이
   * 그 질문에 먼저 답한다. 랭킹·차트처럼 자기 화면 안에서 종목을 바꾸는 페이지
   * (`quotes` 모드)는 지금처럼 페이지가 등록한 핸들러가 처리한다.
   */
  const selectStockSuggestion = useCallback(
    (item: StockInfo) => {
      // 고른 종목을 최근 목록에 남긴다 (§A-4) — 빈 검색창이 이걸 보여준다.
      rememberStock(item);
      if (config.mode === "stock") {
        router.push(analysisSymbolHref(item.symbol));
        clearSearch();
        return;
      }

      if (config.mode === "quotes") {
        stockPickHandlerRef.current?.(item.symbol);
        clearSearch();
      }
    },
    [clearSearch, config.mode, router],
  );

  const submitSearch = useCallback(() => {
    const trimmed = inputValue.trim();
    if (!trimmed) return;

    if (supportsStockSuggestions(config.mode)) {
      const pick = stockSuggestionsRef.current[highlightedIndex] ?? stockSuggestionsRef.current[0];
      if (pick) {
        selectStockSuggestion(pick);
        return;
      }
    }

    if (config.mode === "quotes") {
      const symbol = parseStockSearchSymbol(trimmed);
      if (symbol && stockPickHandlerRef.current) {
        stockPickHandlerRef.current(symbol);
        clearSearch();
        return;
      }
      setDebouncedQuery(trimmed);
      return;
    }

    if (config.mode === "stock") {
      // 종목코드·티커로 읽히는 입력만 이동시킨다. 한글 종목명은 자동완성이 뜨기 전에
      // 엔터를 친 것이므로 화면 안 필터(`activeQuery`)로 두는 편이 낫다 —
      // `/analysis?symbol=삼성전자`는 존재하지 않는 심볼이다.
      const symbol = parseStockSearchSymbol(trimmed);
      if (symbol) {
        router.push(analysisSymbolHref(symbol));
        clearSearch();
        return;
      }
    }

    setDebouncedQuery(trimmed);
  }, [clearSearch, config.mode, highlightedIndex, inputValue, router, selectStockSuggestion]);

  /*
   * 검색어가 있으면 **결과가 0건이어도** 패널을 연다.
   *
   * 전에는 `stockSuggestions.length > 0`를 함께 걸어서, 0건이면 패널 자체가 뜨지
   * 않았다. 그래서 `HeaderStockSuggestions`의 「검색 결과가 없습니다」 분기가
   * 한 번도 그려지지 않는 죽은 코드였고, 사용자 입장에서는 **아직 찾는 중인지
   * 없는 것인지 구분할 방법이 없었다**(실측: `005930` 입력 → 아무 것도 안 뜸).
   */
  const suggestionsOpen = stockSuggestionsOpen(config.mode, inputValue);

  const value = useMemo(
    () => ({
      config,
      mode: config.mode,
      inputValue,
      setInputValue,
      activeQuery,
      isSearchActive: activeQuery.length > 0,
      clearSearch,
      submitSearch,
      stockSuggestions,
      assetSuggestions,
      suggestionsLoading,
      suggestionsOpen,
      highlightedIndex,
      setHighlightedIndex,
      selectStockSuggestion,
      registerStockPickHandler,
    }),
    [
      config,
      inputValue,
      activeQuery,
      clearSearch,
      submitSearch,
      stockSuggestions,
      assetSuggestions,
      suggestionsLoading,
      suggestionsOpen,
      highlightedIndex,
      selectStockSuggestion,
      registerStockPickHandler,
    ],
  );

  return (
    <HeaderSearchContext.Provider value={value}>{children}</HeaderSearchContext.Provider>
  );
}

export function useHeaderSearch() {
  const ctx = useContext(HeaderSearchContext);
  if (!ctx) {
    throw new Error("useHeaderSearch must be used within HeaderSearchProvider");
  }
  return ctx;
}

/** Returns null when provider is missing (e.g. auth layout). */
export function useHeaderSearchOptional() {
  return useContext(HeaderSearchContext);
}
