export type FinancialHistoryPeriod = {
  fiscalYear: number;
  periodIndex: number; // 1..4
  periodEnd?: string;

  revenue?: number;
  operatingIncome?: number;
  netIncome?: number;
  epsBasic?: number;

  totalAssets?: number;
  totalLiabilities?: number;
  totalEquity?: number;
  currentAssets?: number;
  currentLiabilities?: number;

  operatingMargin?: number;
  netProfitMargin?: number;
  debtRatio?: number;
  currentRatio?: number;

  /** 분기말 종가 ÷ 최근 4개 분기 EPS 합. 만들 수 없으면 없다. */
  per?: number;
  /** 분기말 종가 ÷ 주당순자산. */
  pbr?: number;
  /** 위 둘의 분자 — 그 분기 마지막 거래일 종가. */
  closeAtPeriodEnd?: number;

  revenueGrowthQoq?: number;
  operatingIncomeGrowthQoq?: number;
  netIncomeGrowthQoq?: number;
};

export type FinancialHistoryData = {
  symbol: string;
  assetType: string;
  periods: FinancialHistoryPeriod[]; // oldest first
  /** 공시 통화가 상장 통화와 다를 때만(예: TSM → "TWD"). 금액은 이 통화다(2026-09-28). */
  currency?: string;
  /** 기간마다 통화가 섞였을 수 있다(달러 환산치 + 현지 통화). */
  currencyMixed?: boolean;
};
