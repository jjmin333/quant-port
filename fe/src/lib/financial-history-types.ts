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

  revenueGrowthQoq?: number;
  operatingIncomeGrowthQoq?: number;
  netIncomeGrowthQoq?: number;
};

export type FinancialHistoryData = {
  symbol: string;
  assetType: string;
  periods: FinancialHistoryPeriod[]; // oldest first
};
