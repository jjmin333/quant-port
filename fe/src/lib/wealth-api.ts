import { apiRequest } from "@/lib/api-client";
import type { AssetMention, PreviewAssetKind } from "@/lib/mentions";
import type { ScreenField } from "@/lib/screener-api";

/**
 * 순자산(Wealth) — `be/internal/handler/wealth.go`의 거울 (2026-10-01).
 *
 * 주식·ETF·현금은 지금처럼 보유종목 API에서 오고(합산은 화면에서), 여기는 **시세 기호가 없는 자산**
 * (부동산·예금·적금·채권·코인·금·자동차·미술품·기타)과 부채, 그리고 공공데이터 조회다.
 * 금액은 전부 원. 평가 금액(value)은 전체 금액이고 `myValue`가 지분율을 곱한 내 몫이다.
 */

export type AssetType = "real_estate" | "deposit" | "saving" | "bond" | "crypto" | "gold" | "vehicle" | "art" | "other";
export type PropertyType = "apt" | "offi" | "rh" | "sh" | "land" | "nrg" | "indu" | "silv";
export type ValuationKind = "market" | "last_trade" | "estimate" | "user_input" | "accrual";
export type Confidence = "high" | "medium" | "low" | "";

export const ASSET_TYPE_LABEL: Record<AssetType, string> = {
  real_estate: "부동산",
  deposit: "예금",
  saving: "적금",
  bond: "채권",
  crypto: "코인",
  gold: "금",
  vehicle: "자동차",
  art: "미술품·수집품",
  other: "기타",
};

export const PROPERTY_TYPE_LABEL: Record<PropertyType, string> = {
  apt: "아파트",
  offi: "오피스텔",
  rh: "연립·다세대",
  sh: "단독·다가구",
  silv: "분양·입주권",
  land: "토지",
  nrg: "상업·업무용",
  indu: "공장·창고",
};

/** 숫자의 성격 — 화면은 이 이름을 숫자 옆에 늘 붙인다(시장가와 추정가를 섞어 보이지 않게). */
export const VALUATION_KIND_LABEL: Record<ValuationKind, string> = {
  market: "시장가",
  last_trade: "최근 실거래가",
  estimate: "추정가",
  user_input: "직접 입력",
  accrual: "원리금 계산",
};

export const CONFIDENCE_LABEL: Record<Exclude<Confidence, "">, string> = {
  high: "신뢰도 높음",
  medium: "신뢰도 보통",
  low: "신뢰도 낮음",
};

export type AssetValuation = {
  assetId: number;
  asOf: string;
  value: number;
  low?: number;
  high?: number;
  kind: ValuationKind;
  confidence?: Confidence;
  source: string;
  inputs?: unknown;
};

export type ReDetail = {
  propertyType: PropertyType;
  lawdCd: string;
  regionName?: string;
  complexKey?: string;
  complexName?: string;
  umdNm?: string;
  address?: string;
  area: number;
  floor?: number;
  leaseDeposit?: number;
  monthlyRent?: number;
  acquisitionCost?: number;
};

export type FixedDetail = {
  principal?: number;
  monthly?: number;
  months?: number;
  rate: number;
  startDate?: string;
  maturityDate?: string;
  compound?: boolean;
  taxRate?: number;
  institution?: string;
  couponFreq?: number;
  credit?: "gov" | "AA" | "BBB" | "";
};

export type UserAsset = {
  id: number;
  assetType: AssetType;
  name: string;
  currency: string;
  quantity?: number;
  acquiredAt?: string;
  acquiredPrice?: number;
  ownershipRatio: number;
  valuationMethod: string;
  userValue?: number;
  detail: Record<string, unknown>;
  memo: string;
  updatedAt: string;
};

export type AssetView = UserAsset & {
  valuation?: AssetValuation;
  valuationNote?: string;
  myValue?: number;
  myCost?: number;
  pnl?: number;
  returnPct?: number;
  annualizedPct?: number;
  live?: Record<string, unknown>;
};

export type LiabilityKind = "mortgage" | "jeonse_deposit" | "credit" | "other";
export const LIABILITY_KIND_LABEL: Record<LiabilityKind, string> = {
  mortgage: "주택담보대출",
  jeonse_deposit: "받은 전세보증금",
  credit: "신용대출",
  other: "기타 부채",
};
export const REPAY_TYPE_LABEL: Record<string, string> = {
  "": "모름",
  equal_payment: "원리금균등",
  equal_principal: "원금균등",
  bullet: "만기일시",
};

export type UserLiability = {
  id: number;
  assetId?: number;
  kind: LiabilityKind;
  name: string;
  principal: number;
  balance: number;
  rate?: number;
  rateType: string;
  repaymentType: string;
  startedAt?: string;
  maturityAt?: string;
  memo: string;
  scheduledBalance?: number;
  monthlyPayment?: number;
};

export type WealthData = { assets: AssetView[]; liabilities: UserLiability[] };

export function fetchWealth(): Promise<WealthData> {
  return apiRequest<WealthData>("/api/v1/wealth");
}

export type AssetInput = {
  assetType: AssetType;
  name: string;
  quantity?: number;
  acquiredAt?: string;
  acquiredPrice?: number;
  ownershipRatio?: number;
  valuationMethod?: string;
  userValue?: number;
  detail: Record<string, unknown>;
  memo?: string;
};

export function saveAsset(input: AssetInput, id?: number): Promise<AssetView> {
  return apiRequest<AssetView>(id ? `/api/v1/wealth/assets/${id}` : "/api/v1/wealth/assets", {
    method: id ? "PUT" : "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(input),
  });
}

export function deleteAsset(id: number): Promise<unknown> {
  return apiRequest(`/api/v1/wealth/assets/${id}`, { method: "DELETE" });
}

export type LiabilityInput = Omit<UserLiability, "id" | "scheduledBalance" | "monthlyPayment">;

export function saveLiability(input: LiabilityInput, id?: number): Promise<UserLiability> {
  return apiRequest<UserLiability>(id ? `/api/v1/wealth/liabilities/${id}` : "/api/v1/wealth/liabilities", {
    method: id ? "PUT" : "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(input),
  });
}

export function deleteLiability(id: number): Promise<unknown> {
  return apiRequest(`/api/v1/wealth/liabilities/${id}`, { method: "DELETE" });
}

// ── 부동산 ──────────────────────────────────────────────────────────────────

export type ReTrade = {
  kind: PropertyType;
  dealType: "sale" | "rent";
  dealDate: string;
  umdNm: string;
  jibun: string;
  complexKey: string;
  complexName: string;
  category?: string;
  area: number;
  landArea?: number;
  floor?: number;
  buildYear?: number;
  price?: number;
  deposit?: number;
  monthlyRent?: number;
  contractType?: string;
  direct: boolean;
  cancelled: boolean;
};

export type ReComp = { date: string; area: number; floor?: number; price: number; adjusted: number };

export type ReEstimate = {
  value: number;
  low: number;
  high: number;
  confidence: Exclude<Confidence, "">;
  method: "comparable" | "region_unit";
  n: number;
  windowMonths: number;
  indexAdjusted: boolean;
  lastTrade?: ReComp;
  lastTradeDays: number;
  recent3Avg?: number;
  recent3N: number;
  sameAreaMedian?: number;
  excludedDirect: number;
  excludedCancelled: number;
  comps: ReComp[];
  unitPerM2?: number;
  jeonse?: { median: number; n: number };
};

export type ReRegion = { lawdCd: string; sido: string; name: string; tier: "all" | "full" };

export type ReComplex = {
  complexKey: string;
  complexName: string;
  kind: PropertyType;
  lawdCd: string;
  umdNm: string;
  jibun: string;
  buildYear?: number;
  trades: number;
  lastDeal: string;
  areas: string;
};

export type IndexPoint = { date: string; value: number };
export type MarketSeries = { key: string; label: string; unit: string; cycle: "WK" | "MM" | "QY"; points: IndexPoint[] };
export type RegionMonth = { ym: string; count: number; avgPerM2: number; rentCount: number };
export type RegionMarket = {
  lawdCd: string;
  sido: string;
  name: string;
  indices: MarketSeries[];
  months?: RegionMonth[];
  source: string;
};

export function fetchRegions(): Promise<ReRegion[]> {
  return apiRequest<ReRegion[]>("/api/v1/realestate/regions");
}

export function fetchComplexes(lawdCd: string, kind: PropertyType, q: string): Promise<ReComplex[]> {
  const p = new URLSearchParams({ kind });
  if (lawdCd) p.set("lawdCd", lawdCd);
  if (q.trim()) p.set("q", q.trim());
  return apiRequest<ReComplex[]>(`/api/v1/realestate/complexes?${p}`);
}

export type ComplexData = {
  score?: ScoreView;
  note?: AINote;
  sales: ReTrade[];
  rents: ReTrade[];
  areas: { area: number; count: number }[];
  estimate?: ReEstimate;
  jeonse?: { median: number; n: number };
};

export function fetchComplex(key: string, lawdCd: string, area?: number): Promise<ComplexData> {
  const p = new URLSearchParams({ key, lawdCd });
  if (area) p.set("area", String(area));
  return apiRequest<ComplexData>(`/api/v1/realestate/complex?${p}`);
}

/** lawdCd(시군구) 또는 sido(시도 2자리) — 둘 다 비면 전국. */
export function fetchRegionMarket(lawdCd: string, sido = ""): Promise<RegionMarket> {
  const q = lawdCd ? `?lawdCd=${lawdCd}` : sido ? `?sido=${sido}` : "";
  return apiRequest<RegionMarket>(`/api/v1/realestate/market${q}`);
}

export type AssetDetailData = {
  asset: AssetView;
  now?: AssetValuation;
  explain?: ReEstimate | Record<string, unknown>;
  error?: string;
  history: AssetValuation[];
  market?: RegionMarket;
  sales?: ReTrade[];
  rents?: ReTrade[];
};

export function fetchAssetDetail(id: number): Promise<AssetDetailData> {
  return apiRequest<AssetDetailData>(`/api/v1/wealth/assets/${id}`);
}

// ── 금리 ────────────────────────────────────────────────────────────────────

export type RateItem = { key: string; label: string; cycle: "D" | "M"; date: string; value: number; prev?: number; prevDate?: string };
export type RatesData = {
  note?: AINote;
  /** 금리 브리핑(홈과 같은 글, 2026-10-03 F4). */
  brief?: AssetBrief;
  items: RateItem[];
  curve: { years: number; value: number }[];
  trend: Record<string, IndexPoint[]>;
  source: string;
};

export function fetchRates(): Promise<RatesData> {
  return apiRequest<RatesData>("/api/v1/macro/rates");
}

// ── 공매 ────────────────────────────────────────────────────────────────────

export type OnbidItem = {
  cltrMngNo: string;
  pbctCdtnNo: number;
  pbancMngNo: string;
  onbidCltrNo?: number;
  name: string;
  address: string;
  sido: string;
  usageL: string;
  usageM: string;
  usageS: string;
  prptDiv: string;
  dspsMethod: string;
  orgName: string;
  appraisal?: number;
  minBid?: number;
  minBidRatio?: number;
  failCount: number;
  bidStart?: string;
  bidEnd?: string;
  result: string;
  winningBid?: number;
  validBidders?: number;
};

export type OnbidQuery = {
  status: "open" | "closed";
  sido: string;
  usage: string;
  minFails: number;
  maxRatio: number;
  method: "" | "매각" | "임대";
  sort: "bid" | "score" | "ratio" | "fails" | "appraisal";
  page: number;
};

export type OnbidList = { items: OnbidView[]; total: number; page: number; pageSize: number; source: string };

export function fetchOnbid(q: OnbidQuery): Promise<OnbidList> {
  // 120건 — 같은 건물 호실을 묶은 뒤에도 한 쪽에 카드가 여럿 남게(2026-10-03 R4).
  const p = new URLSearchParams({ status: q.status, page: String(q.page), sort: q.sort, pageSize: "120" });
  if (q.sido) p.set("sido", q.sido);
  if (q.usage) p.set("usage", q.usage);
  if (q.minFails) p.set("minFails", String(q.minFails));
  if (q.maxRatio) p.set("maxRatio", String(q.maxRatio));
  if (q.method) p.set("method", q.method);
  return apiRequest<OnbidList>(`/api/v1/auctions/onbid?${p}`);
}

export type OnbidStat = { sido: string; usageM: string; sold: number; failed: number; avgRatio: number };

export function fetchOnbidStats(): Promise<OnbidStat[]> {
  return apiRequest<OnbidStat[]>("/api/v1/auctions/onbid/stats");
}

/** 온비드 물건 화면 — 물건별 고정 주소를 API가 주지 않아 검색 첫 화면으로 보낸다. */
export const ONBID_URL = "https://www.onbid.co.kr";

// ── 코인 ────────────────────────────────────────────────────────────────────

export type CryptoTicker = { market: string; koreanName: string; price: number; changeRate: number };

export function fetchCryptoMarkets(): Promise<CryptoTicker[]> {
  return apiRequest<CryptoTicker[]>("/api/v1/crypto/markets");
}

// ── 계산 ────────────────────────────────────────────────────────────────────

/** 순자산 화면의 자산군(주식·ETF는 보유종목에서 온다). */
export type WealthClass = "stock" | "etf" | "cash" | AssetType;

export const WEALTH_CLASS_LABEL: Record<WealthClass, string> = {
  stock: "주식",
  etf: "ETF",
  cash: "현금",
  ...ASSET_TYPE_LABEL,
};

/** 자산군 색 — 차트·범례가 같은 순서·같은 색을 쓴다. */
export const WEALTH_CLASS_COLOR: Record<WealthClass, string> = {
  stock: "#3b6ff5",
  etf: "#7a5af5",
  real_estate: "#e07b39",
  cash: "#8a94a6",
  deposit: "#2bb3a3",
  saving: "#5cc8b8",
  bond: "#4b8bbe",
  crypto: "#f2b134",
  gold: "#c9a227",
  vehicle: "#9b6b4f",
  art: "#c25b8a",
  other: "#6b7280",
};

export type AllocationRow = { cls: WealthClass; value: number; pct: number };

/** 자산군별 합계와 비중(값이 0인 군은 뺀다, 큰 순). */
export function allocation(parts: Partial<Record<WealthClass, number>>): AllocationRow[] {
  const rows = (Object.entries(parts) as [WealthClass, number][]).filter(([, v]) => v > 0);
  const total = rows.reduce((s, [, v]) => s + v, 0);
  return rows
    .map(([cls, value]) => ({ cls, value, pct: total > 0 ? (value / total) * 100 : 0 }))
    .sort((a, b) => b.value - a.value);
}

/** 내 몫의 부채 잔액 — 사용자가 넣은 잔액을 쓴다(약정 잔액은 참고). */
export function liabilityBalance(l: UserLiability): number {
  return l.balance > 0 ? l.balance : l.scheduledBalance ?? l.principal;
}

/** 담보 대출 비율(LTV) — 담보 자산의 내 평가액 대비. */
export function ltv(liabs: UserLiability[], asset: AssetView): number | null {
  const v = asset.myValue ?? 0;
  if (v <= 0) return null;
  const debt = liabs.filter((l) => l.assetId === asset.id).reduce((s, l) => s + liabilityBalance(l), 0);
  return debt > 0 ? (debt / v) * 100 : null;
}

/** 금액 짧게 — 억·만. 음수는 −를 붙인다. */
export function money(v: number | null | undefined): string {
  if (v == null || !Number.isFinite(v)) return "-";
  const sign = v < 0 ? "−" : "";
  const a = Math.abs(v);
  if (a >= 1e8) {
    const eok = a / 1e8;
    return `${sign}${eok >= 100 ? Math.round(eok).toLocaleString("ko-KR") : eok.toFixed(2).replace(/\.?0+$/, "")}억`;
  }
  if (a >= 1e4) return `${sign}${Math.round(a / 1e4).toLocaleString("ko-KR")}만`;
  return `${sign}${Math.round(a).toLocaleString("ko-KR")}원`;
}

export function signedPct(v: number | null | undefined, digits = 1): string {
  if (v == null || !Number.isFinite(v)) return "-";
  return `${v > 0 ? "+" : ""}${v.toFixed(digits)}%`;
}

// ── 점수·AI 해설 (2026-10-01) ────────────────────────────────────────────────

export type ScoreView = {
  score: number;
  grade: "A" | "B" | "C" | "D" | "E";
  parts: Record<string, number>;
  facts: Record<string, unknown>;
  asOf: string;
};

export type AINote = { kind: string; key: string; body: { note?: string; tags?: string[]; points?: string[] } & Record<string, unknown>; source: "llm" | "rule"; model?: string; updatedAt: string };

export type RegionOverviewRow = { lawdCd: string; name: string; score: ScoreView; note?: AINote };

/** 시도·전국 한 덩어리 — 시도는 점수(시도끼리 백분위), 전국은 점수 없이 사실만. */
export type AreaIntel = { key: string; name: string; score?: ScoreView; facts: Record<string, unknown>; note?: AINote; asOf: string };
export type SidoOverviewRow = { sido: string; name: string; score: ScoreView; note?: AINote };
export type RealEstateOverview = { regions: RegionOverviewRow[]; area?: AreaIntel | null; sidos?: SidoOverviewRow[]; method: string };

export function fetchRealEstateOverview(sido: string): Promise<RealEstateOverview> {
  return apiRequest(`/api/v1/realestate/overview${sido ? `?sido=${sido}` : ""}`);
}

export type RankingSort = "score" | "momentumPct" | "trades6m" | "jeonseRatio" | "pyeongPrice" | "peakPct" | "peakDrop";
export type ComplexRankRow = { complexKey: string; name: string; lawdCd: string; score: ScoreView; note?: AINote };

export function fetchComplexRankings(lawdCd: string, sido: string, sort: RankingSort, limit = 30): Promise<{ items: ComplexRankRow[]; method: string }> {
  const p = new URLSearchParams({ sort, limit: String(limit) });
  if (lawdCd) p.set("lawdCd", lawdCd);
  if (sido) p.set("sido", sido);
  return apiRequest(`/api/v1/realestate/rankings?${p}`);
}

export type OnbidView = OnbidItem & { score?: ScoreView; flags: string[] };

export function fetchOnbidRounds(cltr: string): Promise<OnbidView[]> {
  return apiRequest(`/api/v1/auctions/onbid/rounds?cltr=${encodeURIComponent(cltr)}`);
}

export type HomeAssets = {
  rates?: { items: { key: string; label: string; value: number; prev?: number; date: string }[]; note?: AINote };
  realestate?: {
    nation?: { value: number; date: string; w1: number; w4: number; spark: IndexPoint[] };
    seoul?: { value: number; date: string; w1: number; w4: number; spark: IndexPoint[] };
    risers?: { lawdCd: string; name: string; facts: Record<string, number>; score: number }[];
  };
  /** 금리 TOP(2026-10-09 요청 7) — 예금·적금은 기본금리 높은 순, 주담대는 낮은 순 5개 + 한은 평균. */
  products?: Partial<Record<HomeProductGroup, HomeProductBlock>>;
  /** 홈 「부동산·금리 브리핑」(2026-10-03). 첫 회차 전에는 서버가 기존 해설로 같은 모양을 채운다(headline 빈칸). */
  briefing?: { re?: AssetBrief; rates?: AssetBrief };
};

export type HomeProductGroup = "deposit" | "saving" | "mortgage";

export type HomeProductBlock = {
  items: {
    id: string;
    name: string;
    /** finGroup: 금감원 권역 코드(020000 은행 · 030300 저축은행 …), joinDeny 1 = 누구나 가입. */
    facts: { base: number; max: number; min?: number; term: number; company: string; finGroup?: string; joinDeny?: number };
    score: number;
  }[];
  avg?: { label: string; value: number; date: string };
};

export type AssetBrief = {
  /** 상승 · 보합 · 하락 — 규칙(전국 4주 지수 · 국고채 3년 3개월 변화). 폴백은 빈칸일 수 있다. */
  mood: string;
  headline: string;
  body: string[];
  watch: string[];
  tags?: string[];
  source: "llm" | "rule";
  updatedAt: string;
};

export function fetchHomeAssets(): Promise<HomeAssets> {
  return apiRequest<HomeAssets>("/api/v1/home/assets");
}

export const GRADE_CLASS: Record<string, string> = {
  A: "bg-emerald-500/15 text-emerald-600 dark:text-emerald-400",
  B: "bg-sky-500/15 text-sky-600 dark:text-sky-400",
  C: "bg-zinc-500/15 text-zinc-600 dark:text-zinc-300",
  D: "bg-amber-500/15 text-amber-600 dark:text-amber-400",
  E: "bg-rose-500/15 text-rose-600 dark:text-rose-400",
};

// ── 관심 부동산 (2026-10-03 R3) ──────────────────────────────────────────────

export type ReWatchItem = {
  kind: "complex" | "region";
  key: string;
  lawdCd: string;
  name: string;
  createdAt: string;
  score?: ScoreView;
  note?: AINote;
  /** 단지만 — 최근 30일 거래 수 · 마지막 거래 · 같은 평형 3년 최고가를 넘었나. */
  trades?: { recent30: number; last?: ReTrade; newHigh: boolean };
};

export function fetchReWatch(): Promise<{ items: ReWatchItem[]; max: number }> {
  return apiRequest("/api/v1/realestate/watch");
}

export function setReWatch(kind: ReWatchItem["kind"], key: string, on: boolean): Promise<unknown> {
  return on
    ? apiRequest("/api/v1/realestate/watch", { method: "PUT", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ kind, key }) })
    : apiRequest(`/api/v1/realestate/watch?${new URLSearchParams({ kind, key })}`, { method: "DELETE" });
}

/**
 * 자산 하나의 미리보기(2026-10-09 요청 5) — 단지·지역·금융상품·금리. 글 속 자산 태그와 같은 모양이라
 * 같은 모달(`AssetQuickLook`)이 그린다. 열 때 한 번만 부른다.
 */
export async function fetchAssetPeek(kind: PreviewAssetKind, key: string): Promise<{ asset: AssetMention; fields: ScreenField[] }> {
  return apiRequest(`/api/v1/assets/peek?${new URLSearchParams({ kind, key })}`, undefined, "자산 정보를 불러오지 못했습니다");
}
