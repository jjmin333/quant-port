export type CoverageTier = "full" | "basic" | "unavailable" | "not_applicable";

export type FundamentalsCalcStatus = "ok" | "partial" | "failed" | "pending" | "";

export type FundamentalsData = {
  symbol: string;
  assetType?: string;
  productLabel?: string;
  message?: string;
  dataSource?: string;
  fiscalYear?: number;
  /** "FY" | "1Q".."4Q" | "반기" — 이 숫자가 어느 기간의 것인지. */
  fiscalPeriod?: string;
  /** 그 기간의 결산일 (`YYYY-MM-DD`). */
  periodEnd?: string;
  coverageTier: CoverageTier;
  calcStatus?: FundamentalsCalcStatus;
  calcError?: string;
  fetchedAt?: string;

  price?: number;
  per?: number;
  pbr?: number;

  roe?: number;
  operatingMargin?: number;
  revenueGrowthYoy?: number;
  epsGrowthYoy?: number;
  fwdPer?: number;
  cyPer?: number;
  debtRatio?: number;
  roa?: number;
  netProfitMargin?: number;
  currentRatio?: number;

  revenue?: number;
  netIncome?: number;
  totalEquity?: number;
  sharesOutstanding?: number;
  epsBasic?: number;
  epsDiluted?: number;
};

export type MetricTone = "positive" | "negative" | "neutral" | "accent";

export type FundamentalMetric = {
  id: string;
  label: string;
  value: string;
  verdict: string;
  tone: MetricTone;
  gauge: number;
  available: boolean;
  /** 입문자용 한 줄 풀이 (`valuation-labels.ts`). 없는 지표도 있을 수 있다. */
  hint?: string;
};

export type StockFundamentalsPanel = {
  symbol: string;
  name: string;
  currency: "KRW" | "USD";
  coverageTier: CoverageTier;
  calcStatus: FundamentalsCalcStatus;
  dataSource: string;
  fiscalYear?: number;
  /**
   * 재무 지표를 한 줄로 이어 적은 문장.
   *
   * **밸류에이션 탭은 이제 쓰지 않는다**(2026-09-03 사용자 결정) — 바로 아래 지표
   * 카드들이 같은 숫자를 더 읽기 좋게 늘어놓기 때문이다. 남아 있는 쓰임은 재무제표
   * 화면(`/financials`)의 「핵심 요약」 밴드 하나뿐이고, 거기는 지표 카드가 한참
   * 아래에 있어 머리말 역할을 한다.
   */
  summary: string;
  metrics: FundamentalMetric[];
  price?: number;
  message?: string;
  productLabel?: string;
};

export const COVERAGE_TIER_LABEL: Record<CoverageTier, string> = {
  full: "전체 지표",
  basic: "기본 지표",
  unavailable: "데이터 없음",
  not_applicable: "재무제표 미제공",
};
