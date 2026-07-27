export const dashboardSummary = {
  totalAssets: 238_450_000,
  totalEvaluation: 226_050_000,
  totalCash: 12_400_000,
  totalProfit: 37_120_000,
  totalReturnRate: 18.4,
  todayProfit: 3_280_000,
  todayChangeRate: 1.43,
};

export const brokerAccounts = [
  { id: "toss-1", name: "토스증권1", balance: 53_000_000, changeRate: 1.2 },
  { id: "toss-2", name: "토스증권2", balance: 18_000_000, changeRate: -0.4 },
  { id: "kiwoom-1", name: "키움1", balance: 122_000_000, changeRate: 2.1 },
  { id: "kiwoom-2", name: "키움2", balance: 45_000_000, changeRate: 0.8 },
];

export const holdings: Array<{
  symbol: string;
  name: string;
  totalShares: number;
  avgPrice: number;
  currentPrice: number;
  returnRate: number;
  assetType: "domestic" | "us" | "etf" | "bond" | "cash";
  currency: "KRW" | "USD";
  accounts: Array<{ broker: string; shares: number }>;
}> = [
  {
    symbol: "005930",
    name: "삼성전자",
    totalShares: 1530,
    avgPrice: 58_300,
    currentPrice: 62_100,
    returnRate: 6.51,
    assetType: "domestic",
    currency: "KRW",
    accounts: [
      { broker: "토스1", shares: 430 },
      { broker: "키움1", shares: 1100 },
    ],
  },
  {
    symbol: "000660",
    name: "SK하이닉스",
    totalShares: 246,
    avgPrice: 178_500,
    currentPrice: 195_200,
    returnRate: 9.36,
    assetType: "domestic",
    currency: "KRW",
    accounts: [{ broker: "키움1", shares: 246 }],
  },
  {
    symbol: "SOXL",
    name: "Direxion Semiconductor Bull 3X",
    totalShares: 724,
    avgPrice: 32.4,
    currentPrice: 35.8,
    returnRate: 10.49,
    assetType: "etf",
    currency: "USD",
    accounts: [{ broker: "토스2", shares: 724 }],
  },
  {
    symbol: "TSLA",
    name: "Tesla Inc.",
    totalShares: 136,
    avgPrice: 248.2,
    currentPrice: 261.5,
    returnRate: 5.36,
    assetType: "us",
    currency: "USD",
    accounts: [{ broker: "키움2", shares: 136 }],
  },
];

export const watchlist = [
  { symbol: "005930", name: "삼성전자", price: 62_100, changeRate: 2.31 },
  { symbol: "000660", name: "SK하이닉스", price: 195_200, changeRate: 1.85 },
  { symbol: "SOXL", name: "SOXL", price: 35.8, changeRate: -0.92 },
  { symbol: "TSLA", name: "TSLA", price: 261.5, changeRate: 3.12 },
  { symbol: "NVDA", name: "NVDA", price: 128.4, changeRate: 1.44 },
];

export const morningBriefing = {
  summary: "보유종목 4개 상승, 1개 하락. 반도체 섹터 강세 지속.",
  earnings: [{ date: "7/25", symbol: "TSLA", event: "실적발표" }],
  disclosures: [{ date: "오늘", symbol: "SK하이닉스", event: "공시" }],
  volatility: "높음",
  highlights: [
    "반도체 가격 상승",
    "HBM 공급 확대",
    "외국인 순매수 증가",
  ],
};

export const assetHistory = [
  { period: "1월", value: 180_000_000 },
  { period: "4월", value: 195_000_000 },
  { period: "7월", value: 210_000_000 },
  { period: "10월", value: 220_000_000 },
  { period: "현재", value: 238_450_000 },
];

export const portfolioAnalysis = {
  sector: [
    { name: "반도체", value: 42 },
    { name: "AI", value: 18 },
    { name: "ETF", value: 25 },
    { name: "현금", value: 15 },
  ],
  country: [
    { name: "한국", value: 62 },
    { name: "미국", value: 38 },
  ],
};

export const riskAlerts = [
  { type: "warning", message: "삼성전자 비중 43% — 권장 20~30%" },
  { type: "info", message: "미국 비중 8% — 분산 권장" },
];
