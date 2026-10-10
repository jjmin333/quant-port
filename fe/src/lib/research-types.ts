/**
 * 투자 리포트 도메인 타입.
 *
 * 화면 이름은 "리포트 / 투자 리포트"지만 API·테이블·타입은 BE와 같은
 * `research_*` 이름을 유지한다 (`/api/v1/research/*`).
 *
 * 스탬프와는 독립된 기능이므로 스냅샷 형태를 직접 소유한다 — 다른 기능이
 * 바뀌었다는 이유로 바뀔 수 있는 스냅샷은 박제된 게 아니다.
 */
import type { ContentTopic } from "@/lib/topics";

export type ScoreBreakdown = {
  technical?: number;
  quality?: number;
  growth?: number;
  valuation?: number;
  sentiment?: number;
};

export type ResearchCandle = {
  time: string;
  open: number;
  high: number;
  low: number;
  close: number;
  volume: number;
};

/** 스파크라인 한 점 — 목록 응답이 캔들 대신 보내는 것. */
export type ResearchClosePoint = {
  time: string;
  close: number;
};

export type ResearchNews = {
  id?: string;
  headline: string;
  url?: string;
  publisherName?: string;
  publishedAt: string;
};

export type ResearchDisclosure = {
  date: string;
  symbol: string;
  event: string;
};

export type ResearchIndex = {
  id: string;
  name?: string;
  value: number;
  changeRate: number;
};

export type ResearchContext = {
  aiSummary?: string;
  aiKeywords?: string[];
  news: ResearchNews[];
  disclosures: ResearchDisclosure[];
  indices: ResearchIndex[];
  /** What was unavailable at publish time — rendered as an explicit note. */
  dataGaps?: string[];
};

export type ResearchCurrent = {
  price: number;
  changeRate?: number;
  tradeDate?: string;
  source?: "quote" | "close";
};

export type ResearchReturn = {
  horizon: string;
  asOfDate?: string;
  price?: number;
  return?: number;
  benchmarkId?: string;
  benchmarkReturn?: number;
  alpha?: number;
};

export type ResearchDirection = "buy" | "sell" | "hold";
export type ResearchVisibility = "public" | "unlisted" | "private";

export type ResearchStatus =
  | "open"
  | "target_hit"
  | "stop_hit"
  | "expired_hit"
  | "expired_miss"
  | "expired_unknown"
  | "retracted"
  /** 부동산·금융상품 「분석 리포트」 — 종목·목표가·판정이 없다(2026-10-09 요청 9). */
  | "note";

export type PriceLevel = {
  label: string;
  price: number;
  distancePct: number;
};

export type BriefingPoint = {
  label: string;
  text: string;
  tone: "positive" | "negative" | "neutral";
};

export type BriefingScenario = {
  summary: string;
  levels?: PriceLevel[];
};

export type MarketBriefing = {
  asOf?: string;
  price: number;
  currency?: string;
  keyPoints?: BriefingPoint[];
  upside?: BriefingScenario;
  downside?: BriefingScenario;
};

/** What the server measured at publish time. The author supplies none of it. */
export type ResearchEvidence = {
  prevClose?: number;
  changeRate?: number;
  volume?: number;
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
  marketSession?: string;
  briefing?: MarketBriefing;
};

export type ResearchExpectedValue = {
  upsidePct: number;
  downsidePct: number;
  riskReward?: number;
  /** EV using the author's stated conviction. */
  statedEV?: number;
  /** EV using the author's measured hit rate — the gap is the point. */
  historicalEV?: number;
  hitRateUsed?: number;
};

export type ResearchTargetOdds = {
  horizonDays: number;
  requiredPct: number;
  windows: number;
  reached: number;
  probability?: number;
  bestPct?: number;
  medianPct?: number;
  fromDate?: string;
  toDate?: string;
  /** "not enough history to say" — must never render as a low probability. */
  insufficient: boolean;
};

export type ResearchCall = {
  id: number;
  userId: number;
  authorName?: string;
  /** 주식이 아니면 분석 리포트다(status "note", symbol 빈 값). 옛 응답에는 없다 — 없으면 주식. */
  topic?: ContentTopic;
  symbol: string;
  symbolName?: string;
  assetType?: string;
  currency?: "KRW" | "USD";
  title: string;

  publishedAt: string;
  tradeDate: string;
  benchmarkId?: string;

  direction: ResearchDirection;
  basePrice: number;
  targetPrice?: number;
  stopPrice?: number;
  horizonDays: number;
  conviction: number;

  scoreAtPublish?: number;
  gradeAtPublish?: string;
  contrarian: boolean;

  evidence: ResearchEvidence;
  context: ResearchContext;
  /**
   * 발행 시점 차트. **상세는 `bars`(OHLCV 전량), 목록은 `closes`(종가만, 솎아낸 것)**
   * 하나만 채워져 온다 — 둘 다 오지는 않는다.
   *
   * 목록 카드가 그리는 건 스파크라인이고 거기서 읽는 값은 시각과 종가뿐인데,
   * OHLCV 200봉은 리포트 한 건에 19KB라 목록 응답의 대부분을 차지했다.
   * 캔들스틱을 그려야 하는 화면은 상세 응답을 쓴다.
   */
  chart: {
    interval: string;
    bars?: ResearchCandle[];
    closes?: ResearchClosePoint[];
  };

  bodyMd: string;
  bodyRev: number;
  /** 본문의 데이터 태그(상세에서만, 2026-09-28). */
  mentions?: import("@/lib/mentions").MentionSet;
  /**
   * 목록 카드의 표지 이미지. 비어 있으면 화면이 종목 로고로 대신한다 — 목적은
   * 모든 카드에 무언가가 있는 것이지, 모든 작성자가 그림을 찾는 것이 아니다.
   */
  thumbnailUrl?: string;
  tags?: string[];
  visibility: ResearchVisibility;

  status: ResearchStatus;
  verdictAt?: string;
  verdictDate?: string;
  verdictReturn?: number;
  verdictAlpha?: number;

  viewCount: number;
  retractedAt?: string;
  hiddenAt?: string;
  createdAt: string;
  updatedAt: string;

  current?: ResearchCurrent;
  returns: ResearchReturn[];
  horizon?: ResearchReturn;
  dueAt: string;
  due: boolean;
  /** Elapsed fraction of the horizon, 0..1. */
  progress: number;
  toTargetPct?: number;
  toStopPct?: number;
  expectedValue?: ResearchExpectedValue;
  canEdit: boolean;
  revisions: number;

  likeCount: number;
  commentCount: number;
  bookmarkCount: number;
  liked: boolean;
  bookmarked: boolean;
  rating?: ResearchRatingSummary;
  myRating?: ResearchRating;
  /** False once the verdict is in — a settled call can only be outcome-scored. */
  canRate: boolean;
};

export type ResearchComment = {
  id: number;
  callId: number;
  parentId?: number;
  userId: number;
  authorName?: string;
  body: string;
  deleted: boolean;
  canEdit: boolean;
  createdAt: string;
  updatedAt: string;
  replies?: ResearchComment[];

  /** 이 댓글의 추천 수와 내가 눌렀는지 (2026-09-14 요청 3). */
  likeCount: number;
  liked: boolean;
  /**
   * 글쓴이의 누적 활동 수 — 발행한 리포트 + 남긴 댓글.
   *
   * 화면은 이 값을 **구간으로 접어** 보여준다(`activityBadge`). 정확한 수는 읽는
   * 사람에게 아무 결정도 바꾸지 않으면서 그 사람의 활동량만 필요 이상으로 드러낸다.
   */
  authorActivity?: number;
};

/** Reader score of the *argument*, split so raters must say what was good. */
export type ResearchRating = {
  callId: number;
  userId: number;
  clarity: number;
  evidence: number;
  risk: number;
  createdAt: string;
  updatedAt: string;
};

export type ResearchRatingSummary = {
  count: number;
  clarity: number;
  evidence: number;
  risk: number;
  overall: number;
};

/**
 * One author's scorecard. There is no ranking type beside it on purpose —
 * ranking authors makes a report a contest entry, and a contest needs prizes
 * (docs/RESEARCH_CALL_DESIGN.md §15-1).
 */
export type ResearchAuthorProfile = {
  userId: number;
  authorName: string;
  summary: ResearchSummary;
  calls: ResearchCall[];
};

export const RATING_AXES = [
  { id: "clarity", label: "논거 명확성" },
  { id: "evidence", label: "데이터 근거" },
  { id: "risk", label: "리스크 언급" },
] as const;

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
  briefing: "룰 기반 분석",
  news: "관련 뉴스",
  disclosures: "공시",
  chart: "차트",
  aiSummary: "AI 요약",
};

export function dataGapLabel(gap: string): string {
  return DATA_GAP_LABELS[gap] ?? gap;
}

export type ResearchSummary = {
  total: number;
  open: number;
  resolved: number;
  hits: number;
  retracted: number;
  symbols: number;
  hitRate?: number;
  avgReturn?: number;
  avgAlpha?: number;
  /** Shown next to the hit rate — one without the other is an advertisement. */
  retractRate?: number;
  calibration?: number;
  contrarianResolved: number;
  contrarianHits: number;
  contrarianHitRate?: number;
  minSample: number;
};

export type ResearchPreview = {
  symbol: string;
  symbolName?: string;
  assetType?: string;
  currency?: "KRW" | "USD";
  basePrice: number;
  benchmarkId?: string;
  score?: number;
  grade?: string;
  contrarian: boolean;
  evidence: ResearchEvidence;
  context: ResearchContext;
  /** Two-line prompt for the empty editor. Cleared on the author's first click. */
  draftMd: string;
  /** 발행 시점 데이터 section, inserted only when the author asks for it. */
  dataMd: string;
  levels: PriceLevel[];
  odds?: ResearchTargetOdds;
  expectedValue?: ResearchExpectedValue;
  eligible: boolean;
  ineligibleReason?: string;
  /** 하루 발행 한도와 오늘 남은 수. 발행 확인 화면이 대가를 말할 수 있게 한다. */
  dailyCap?: number;
  dailyRemaining?: number;
};

export type ResearchRevision = {
  id: number;
  callId: number;
  rev: number;
  title: string;
  bodyMd: string;
  createdAt: string;
};

export type ResearchSymbolSummary = {
  symbol: string;
  total: number;
  buy: number;
  sell: number;
  hold: number;
  open: number;
  resolved: number;
  hits: number;
};

export type ResearchPublishInput = {
  topic?: ContentTopic;
  symbol: string;
  title?: string;
  direction: ResearchDirection;
  targetPrice?: number | null;
  stopPrice?: number | null;
  horizonDays: number;
  conviction: number;
  bodyMd: string;
  thumbnailUrl?: string;
  tags?: string[];
  visibility: ResearchVisibility;
  interval?: string;
};

export type ResearchUpdateInput = {
  title: string;
  bodyMd: string;
  thumbnailUrl?: string;
  tags?: string[];
  visibility: ResearchVisibility;
};

export type ResearchPreviewInput = {
  symbol: string;
  direction: ResearchDirection;
  targetPrice?: number | null;
  stopPrice?: number | null;
  horizonDays: number;
  conviction: number;
  interval?: string;
};

export const DIRECTION_LABELS: Record<ResearchDirection, string> = {
  buy: "매수",
  sell: "매도",
  hold: "관망",
};

export const STATUS_LABELS: Record<ResearchStatus, string> = {
  open: "검증 중",
  target_hit: "목표 달성",
  stop_hit: "손절 도달",
  expired_hit: "만기 적중",
  expired_miss: "만기 빗나감",
  expired_unknown: "판정 불가",
  retracted: "철회",
  note: "분석 리포트",
};

export const VISIBILITY_LABELS: Record<ResearchVisibility, string> = {
  public: "공개",
  unlisted: "링크 공개",
  private: "비공개",
};

export const RESEARCH_HORIZON_OPTIONS = [
  { days: 30, label: "1개월" },
  { days: 90, label: "3개월" },
  { days: 180, label: "6개월" },
  { days: 365, label: "12개월" },
] as const;

export function isHitStatus(status: ResearchStatus): boolean {
  return status === "target_hit" || status === "expired_hit";
}

/** Tone for a status chip: won / lost / neither. */
export function statusTone(status: ResearchStatus): "positive" | "negative" | "neutral" {
  if (isHitStatus(status)) return "positive";
  if (status === "stop_hit" || status === "expired_miss") return "negative";
  return "neutral";
}
