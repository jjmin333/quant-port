import type { StockInfo } from "@/lib/quotes-types";

/**
 * 최근 본 종목 (§A-4).
 *
 * 왜 필요한가: 검색창은 포커스만 하면 **빈 패널**이었다. 사용자가 매일 보는 종목은
 * 몇 개뿐인데 매번 이름을 처음부터 다시 쳐야 했다. 빈 상태를 「지금 필요할 만한 것」으로
 * 채우는 것이 검색창이 할 수 있는 가장 값싼 일이다.
 *
 * 서버에 두지 않는 이유: 조회 기록은 계정 데이터라기보다 **이 브라우저의 흔적**이고,
 * 서버에 두면 「내가 뭘 봤는지」가 계정에 남는다. 이 기능이 주는 편의에 비해 과한 대가다.
 */

const STORAGE_KEY = "quant-port-recent-stocks";
const MAX = 8;

export type RecentStock = {
  symbol: string;
  name: string;
  market?: string;
  currency?: string;
  assetType?: string;
  at: number;
};

function read(): RecentStock[] {
  if (typeof window === "undefined") return [];
  try {
    const raw = window.localStorage.getItem(STORAGE_KEY);
    if (!raw) return [];
    const parsed = JSON.parse(raw) as RecentStock[];
    return Array.isArray(parsed) ? parsed.filter((r) => r && typeof r.symbol === "string") : [];
  } catch {
    return [];
  }
}

export function recentStocks(): RecentStock[] {
  return read();
}

export function rememberStock(stock: Partial<StockInfo> & { symbol: string }): void {
  if (typeof window === "undefined") return;
  const symbol = stock.symbol.trim().toUpperCase();
  if (!symbol) return;
  try {
    // 같은 종목은 하나만 남기고 맨 앞으로 올린다 — 목록이 「최근 순」이 아니면
    // 최근 본 종목이라는 이름이 거짓말이 된다.
    const next = [
      {
        symbol,
        name: stock.name || symbol,
        market: stock.market,
        currency: stock.currency,
        assetType: stock.assetType,
        at: Date.now(),
      },
      ...read().filter((r) => r.symbol.toUpperCase() !== symbol),
    ].slice(0, MAX);
    window.localStorage.setItem(STORAGE_KEY, JSON.stringify(next));
  } catch {
    // 용량 초과·프라이빗 모드. 편의 기능이므로 실패해도 조용히 넘어간다.
  }
}

export function clearRecentStocks(): void {
  if (typeof window === "undefined") return;
  try {
    window.localStorage.removeItem(STORAGE_KEY);
  } catch {
    /* 위와 같다 */
  }
}
