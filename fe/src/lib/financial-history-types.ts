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
};
