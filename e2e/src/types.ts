/**
 * BE 모델 중 **테스트가 실제로 읽는 필드만** 옮겨 적는다.
 *
 * 전체를 미러링하지 않는 이유: 미러가 커지면 BE가 필드를 하나 바꿀 때마다 여기가
 * 깨지고, 그 깨짐은 버그가 아니라 잡음이다. 테스트가 단언하는 값만 두면 타입이
 * 곧 "이 테스트가 의존하는 계약"의 목록이 된다.
 */

export type UserPublic = {
  id: number;
  email: string;
  displayName: string;
  role: string;
  isAdmin: boolean;
};

export type AuthSession = {
  user: UserPublic;
  accessToken?: string;
  expiresIn: number;
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

export type UserHolding = {
  id: number;
  symbol: string;
  name?: string;
  quantity: number;
  avgPrice?: number;
  costCurrency?: string;
  source: string;
};

export type HoldingTrade = {
  id: number;
  symbol: string;
  side: "buy" | "sell";
  quantity: number;
  price: number;
  currency: string;
  tradeDate: string;
  fee: number;
  note?: string;
};

export type WatchlistItem = {
  id: number;
  symbol: string;
  createdAt: string;
  addedPrice?: number;
};

export type JournalNote = {
  id: number;
  title: string;
  body: string;
  symbol?: string;
  priceAtWrite?: number;
  priceCurrency?: string;
  tags?: string[];
  pinned: boolean;
};

export type PriceTarget = {
  id: number;
  symbol: string;
  targetPrice: number;
  basePrice: number;
  baseDate?: string;
  note?: string;
};

export type CashBalances = { krw: number; usd: number };

export type StampStance = "bullish" | "bearish" | "neutral";

export type Stamp = {
  id: number;
  symbol: string;
  price: number;
  targetPrice: number;
  stance: StampStance;
  conviction: number;
  horizonKind: string;
  dueAt: string;
  note: string;
  tags?: string[];
  stampedAt: string;
};

export type StampState = {
  symbol: string;
  count: number;
  lastStampedAt?: string;
  cooldownRemainingSeconds?: number;
};

export type ResearchDirection = "buy" | "sell" | "hold";
export type ResearchVisibility = "public" | "unlisted" | "private";

export type ResearchCall = {
  id: number;
  userId: number;
  symbol: string;
  title: string;
  direction: ResearchDirection;
  basePrice: number;
  targetPrice?: number;
  stopPrice?: number;
  horizonDays: number;
  conviction: number;
  scoreAtPublish?: number;
  gradeAtPublish?: string;
  bodyMd: string;
  bodyRev: number;
  visibility: ResearchVisibility;
  status: string;
  publishedAt: string;
  tradeDate: string;
  viewCount: number;
  retractedAt?: string | null;
  evidence: { marketCap?: number; prevClose?: number | null };
};

export type ImportJobSnapshot = {
  jobId: string;
  userId: number;
  status: "pending" | "processing" | "preview" | "failed" | "committed";
  progress: { done: number; total: number };
};

export type BatchJobStatus = {
  spec: { id: string; label: string; cost: string; category: string };
  lastRun?: { id: number; jobId: string; status: string; triggerKind: string } | null;
  running: boolean;
};

export type StockSummaryView = {
  symbol: string;
  generatedAt?: string;
};
