/** Mirrors be/internal/model/holding_trade.go. */

export type TradeSideKind = "buy" | "sell";

export type HoldingTrade = {
  id: number;
  symbol: string;
  name?: string;
  side: TradeSideKind;
  quantity: number;
  price: number;
  currency: string;
  fxRate?: number;
  /** YYYY-MM-DD */
  tradeDate: string;
  fee: number;
  note?: string;
  createdAt: string;
};

export type HoldingPosition = {
  symbol: string;
  quantity: number;
  avgPrice: number;
  currency: string;
  realizedPnl: number;
  tradeCount: number;
  firstDate?: string;
};

export type ValuationPoint = {
  date: string;
  valueKrw: number;
  costKrw: number;
};

export type ValuationSeries = {
  points: ValuationPoint[];
  /** 거래는 있는데 일별 종가가 없어 값을 못 매긴 종목 — 화면이 이걸 말해야 한다. */
  missingSymbols?: string[];
  /** 보유는 하는데 거래 이력이 아예 없는 종목. 곡선에 포함될 수 없다. */
  untrackedSymbols?: string[];
  exchangeRate: number;
};

export type NewTradeInput = {
  symbol: string;
  side: TradeSideKind;
  quantity: number;
  price: number;
  currency: string;
  tradeDate: string;
  fee?: number;
  fxRate?: number;
  note?: string;
};
