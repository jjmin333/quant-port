export type StampStance = "bullish" | "bearish" | "neutral";
export type StampVerdict = "pending" | "hit" | "miss" | "unknown";

/**
 * 검증 기간의 종류. 실제 만기 시각은 서버가 생성 시점에 장 마감 캘린더로 확정해
 * `dueAt`에 박아두므로, 여기 값은 화면에 이름을 붙이는 용도다.
 */
export type StampHorizonKind =
  | "session_close"
  | "next_session_close"
  | "1w"
  | "1m"
  | "3m"
  | "custom";

export type ScoreBreakdown = {
  technical?: number;
  quality?: number;
  growth?: number;
  valuation?: number;
  sentiment?: number;
};

export type StampNews = {
  id?: string;
  headline: string;
  url?: string;
  publisherName?: string;
  publishedAt: string;
};

export type StampDisclosure = {
  date: string;
  symbol: string;
  event: string;
};

export type StampIndex = {
  id: string;
  name?: string;
  value: number;
  changeRate: number;
};

export type StampCandle = {
  time: string;
  open: number;
  high: number;
  low: number;
  close: number;
  volume: number;
};

export type StampSnapshot = {
  open?: number;
  high?: number;
  low?: number;
  week52High?: number;
  week52Low?: number;
  marketCap?: number;
  tradingValue?: number;
  per?: number | null;
  pbr?: number | null;
  scoreKind?: string;
  breakdown: ScoreBreakdown;
  coverage?: number;
  peerGroup?: string;
  peerPct?: number;
  watchlisted: boolean;
  held: boolean;
  holdingQuantity?: number;
  holdingAvgPrice?: number;
};

export type StampContext = {
  aiSummary?: string;
  aiKeywords?: string[];
  news: StampNews[];
  disclosures: StampDisclosure[];
  indices: StampIndex[];
  /** What was unavailable at capture time — rendered as an explicit note. */
  dataGaps?: string[];
};

export type StampReturn = {
  horizon: string;
  asOfDate?: string;
  price?: number;
  return?: number;
  benchmarkId?: string;
  benchmarkReturn?: number;
  alpha?: number;
};

export type StampCurrent = {
  price: number;
  changeRate?: number;
  score?: number;
  grade?: string;
  breakdown: ScoreBreakdown;
  tradeDate?: string;
  source?: "quote" | "close";
};

/** 검증 기간 중 장중에 목표가를 찍은 날. 그 순간 스탬프는 적중으로 종료된다. */
export type StampTargetHit = {
  date: string;
  /** 목표가에 닿은 고가(강세) 또는 저가(약세). */
  price: number;
};

export type Stamp = {
  id: number;
  symbol: string;
  symbolName?: string;
  assetType?: string;
  currency?: "KRW" | "USD";
  stampedAt: string;
  tradeDate: string;
  marketSession?: string;
  benchmarkId?: string;
  price: number;
  prevClose?: number;
  changeRate?: number;
  volume: number;
  score?: number;
  grade?: string;
  /** 예상 주가와 당시 주가에서 파생된다 — 직접 고르지 않는다. */
  stance: StampStance;
  targetPrice: number;
  conviction: number;
  horizonKind: StampHorizonKind;
  /** 서버가 장 마감 캘린더로 확정한 만기. */
  dueAt: string;
  note: string;
  tags?: string[];
  snapshot: StampSnapshot;
  context: StampContext;
  chart: { interval: string; bars: StampCandle[] };
  openedAt?: string;
  createdAt: string;
  updatedAt: string;

  current?: StampCurrent;
  returns: StampReturn[];
  horizon?: StampReturn;
  verdict: StampVerdict;
  targetHit?: StampTargetHit;
  /** |만기가 − 목표가| ÷ 목표가 (%). 판정을 좌우하지 않는 별도 정확도 축. */
  targetAccuracy?: number;
  /** 예상한 폭 중 몇 %가 실현됐나. 100이면 도달, 음수면 반대로 갔다. */
  targetProgress?: number;
  /** 목표가가 함의하는 등락률 (%). */
  expectedReturn?: number;
  /** 스탬프 이후 최고/최저 등락률 (%). */
  peakReturn?: number;
  troughReturn?: number;
  due: boolean;
  opened: boolean;
};

export type StampSummary = {
  total: number;
  symbols: number;
  dueCount: number;
  judged: number;
  hits: number;
  hitRate?: number;
  avgReturn?: number;
  avgAlpha?: number;
  lastStampedAt?: string;
  /** 목표가 평균 오차 (%) — 적중률 옆에 서는 두 번째 축. */
  avgTargetError?: number;
  /** 오차가 ±3% 이내였던 건수 (폭까지 맞춘 스탬프). */
  withinBand: number;
  /** 장중 목표가 도달로 조기 종료된 건수. */
  earlyHits: number;
};

export type StampState = {
  symbol: string;
  count: number;
  lastStampedAt?: string;
  cooldownRemainingSeconds: number;
};

export type StampCreateInput = {
  symbol: string;
  /** 필수 — 이 값이 없으면 채점할 수 없다. 서버가 당시 주가 기준으로 검증한다. */
  targetPrice: number;
  horizonKind: StampHorizonKind;
  /** `horizonKind === "custom"`일 때만 읽힌다 (YYYY-MM-DD). */
  customDueDate?: string;
  conviction?: number;
  note?: string;
  tags?: string[];
  interval?: string;
};

/** 언제든 고칠 수 있는 것들 — 미래에 대한 주장이 아니라서. */
export type StampUpdateInput = {
  conviction: number;
  note: string;
  tags?: string[];
};

/** 예상 주가·검증 기간 정정. 스탬프 직후 5분 안에만 서버가 받아준다. */
export type StampReviseInput = {
  targetPrice: number;
  horizonKind: StampHorizonKind;
  customDueDate?: string;
};

export const STANCE_LABELS: Record<StampStance, string> = {
  bullish: "강세",
  bearish: "약세",
  neutral: "관망",
};

export const VERDICT_LABELS: Record<StampVerdict, string> = {
  pending: "검증 중",
  hit: "적중",
  miss: "빗나감",
  unknown: "판정 불가",
};

export const HORIZON_OPTIONS: { id: StampHorizonKind; label: string; hint: string }[] = [
  { id: "session_close", label: "이번 장 종료", hint: "오늘 장 마감 (마감 후에는 다음 장)" },
  { id: "next_session_close", label: "다음 장 종료", hint: "다음 거래일 마감" },
  { id: "1w", label: "1주", hint: "일주일 뒤 마감" },
  { id: "1m", label: "1개월", hint: "한 달 뒤 마감" },
  { id: "3m", label: "3개월", hint: "석 달 뒤 마감" },
  { id: "custom", label: "직접", hint: "날짜를 골라 그날 마감" },
];

export const HORIZON_LABELS: Record<StampHorizonKind, string> = {
  session_close: "이번 장 종료",
  next_session_close: "다음 장 종료",
  "1w": "1주",
  "1m": "1개월",
  "3m": "3개월",
  custom: "직접 지정",
};

/**
 * 목표가가 당시 주가와 이만큼(%) 이내로 붙어 있으면 관망으로 읽는다. BE의
 * `model.StampNeutralBandPct`와 같은 값이어야 화면과 판정이 어긋나지 않는다.
 */
export const STAMP_NEUTRAL_BAND_PCT = 3;

/** 예상 주가에서 관점을 파생한다 — 폼에서 따로 고르지 않는다. */
export function stanceForTarget(price: number, target: number): StampStance {
  if (!(price > 0) || !(target > 0)) return "neutral";
  const diff = ((target - price) / price) * 100;
  if (diff > STAMP_NEUTRAL_BAND_PCT) return "bullish";
  if (diff < -STAMP_NEUTRAL_BAND_PCT) return "bearish";
  return "neutral";
}

export const BENCHMARK_LABELS: Record<string, string> = {
  kospi: "코스피",
  kosdaq: "코스닥",
  sp500: "S&P 500",
  nasdaq: "나스닥",
  nasdaq100: "나스닥 100",
};

const DATA_GAP_LABELS: Record<string, string> = {
  quote: "실시간 시세",
  score: "종합 점수",
  news: "관련 뉴스",
  disclosures: "공시",
  chart: "차트",
  aiSummary: "AI 요약",
};

export function dataGapLabel(gap: string): string {
  return DATA_GAP_LABELS[gap] ?? gap;
}
