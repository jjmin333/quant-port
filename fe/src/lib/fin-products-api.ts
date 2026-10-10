import { joinTargets, joinWays, myRate, type JoinTarget, type JoinWay } from "@/lib/fin-conds";
import { apiRequest } from "@/lib/api-client";
import type { ScoreView } from "@/lib/wealth-api";
import {
  depositInterest,
  loanRepayment,
  savingInterest,
  type InterestResult,
  type InterestType,
  type RepayMethod,
  type RepayResult,
  type TaxKind,
} from "@/lib/fin-calc";

/**
 * 금융상품 비교 — `be/internal/handler/fin_products.go`의 거울 (2026-09-29).
 *
 * 서버는 금감원 「금융상품 한눈에」 공시를 **원문 그대로**(`base`·`options`) 준다. 필드 이름은
 * API 명세 그대로다(snake_case). 순위와 계산은 전부 여기서 한다 — 사용자가 금액·기간을
 * 바꿀 때마다 서버를 부르지 않게.
 */

export type FinKind = "deposit" | "saving" | "mortgage" | "rent" | "credit";
type Raw = Record<string, unknown>;

export type FinProduct = {
  id: number;
  kind: FinKind;
  finCoNo: string;
  productCode: string;
  group: string;
  companyName: string;
  productName: string;
  dclsMonth: string;
  base: Raw;
  options: Raw[];
  homepageUrl?: string;
  phone?: string;
  fetchedAt: string;
  /** 서버 규칙 점수(asset_scores) — 같은 종류 안 백분위. */
  score?: ScoreView;
  /** 우대조건 AI 정리(asset_ai, 자산 전용 LLM 키). 없으면 원문만. */
  ai?: FinConds;
  /** 지난 공시월의 대표 옵션 금리 — 「지난달보다」. */
  prevBase?: number;
  prevMax?: number;
};

export type FinConds = { conds?: { l: string; b: number }[]; easy?: "쉬움" | "보통" | "어려움" | string; note?: string };

export type FinProductList = {
  kind: FinKind;
  fetchedAt: string | null;
  dclsMonth: string;
  items: FinProduct[];
  /** 화면 맨 위 AI 시장 분석(2026-10-09) — 예·적금 화면은 fin_savings, 대출 화면은 fin_loans. 배치 전이면 없다. */
  brief?: FinBrief;
};

export type FinBrief = {
  body: { headline?: string; body?: string[]; watch?: string[] };
  source: string;
  updatedAt: string;
};

export const FIN_KIND_LABEL: Record<FinKind, string> = {
  deposit: "정기예금",
  saving: "적금",
  mortgage: "주택담보대출",
  rent: "전세자금대출",
  credit: "개인신용대출",
};

export const FIN_GROUP_LABEL: Record<string, string> = {
  "020000": "은행",
  "030200": "여신전문",
  "030300": "저축은행",
  "050000": "보험",
  "060000": "금융투자",
};

/** 금감원 비교공시 — 상품별 고정 주소가 없어 첫 화면으로 보낸다. */
export const FSS_FINLIFE_URL = "https://finlife.fss.or.kr";

export async function fetchFinProducts(kind: FinKind): Promise<FinProductList> {
  const list = await apiRequest<FinProductList>(`/api/v1/fin/products?kind=${kind}`);
  return { ...list, items: (list.items ?? []).map(withShortName) };
}

export async function fetchFinProduct(id: number): Promise<FinProduct> {
  return withShortName(await apiRequest<FinProduct>(`/api/v1/fin/products/${id}`));
}

/**
 * 공시 회사명의 법인 표기를 뗀다 — 「주식회사 케이뱅크」·「농협은행주식회사」·「(주)…」(2026-10-01, 실데이터 확인).
 * 화면 표시용이다. 원문은 DB에 그대로 있다.
 */
export function shortCompanyName(name: string): string {
  return name.replace(/주식회사|\(주\)|㈜/g, "").replace(/\s+/g, " ").trim() || name;
}

function withShortName(p: FinProduct): FinProduct {
  return { ...p, companyName: shortCompanyName(p.companyName) };
}

export function fetchFinWatchlist(): Promise<number[]> {
  return apiRequest<number[]>("/api/v1/fin/watchlist");
}

export function setFinWatch(id: number, on: boolean): Promise<unknown> {
  return apiRequest(`/api/v1/fin/watchlist/${id}`, { method: on ? "PUT" : "DELETE" });
}

/** 공시 숫자 — 숫자·문자열 숫자·null이 섞여 온다. 없으면 null(0으로 읽지 않는다). */
export function num(v: unknown): number | null {
  if (typeof v === "number") return Number.isFinite(v) ? v : null;
  if (typeof v === "string" && v.trim() !== "") {
    const n = Number(v.replace(/,/g, ""));
    return Number.isFinite(n) ? n : null;
  }
  return null;
}

export function str(v: unknown): string {
  return typeof v === "string" ? v.trim() : v == null ? "" : String(v);
}

/** YYYYMM → 「2026년 9월」. */
export function formatDclsMonth(m: string): string {
  return /^\d{6}$/.test(m) ? `${m.slice(0, 4)}년 ${Number(m.slice(4))}월` : "-";
}

/** 마지막 수집이 이틀 넘게 지났으면 「갱신 지연」 — 배치는 매일 05:10. */
export function isStale(fetchedAt: string | null, now = Date.now()): boolean {
  if (!fetchedAt) return false;
  return now - new Date(fetchedAt).getTime() > 2 * 24 * 3600 * 1000;
}

// ── 예·적금 순위 ────────────────────────────────────────────────────────────

export type GroupFilter = "all" | "020000" | "030300" | "other";

function groupMatches(g: string, f: GroupFilter) {
  if (f === "all") return true;
  if (f === "other") return g !== "020000" && g !== "030300";
  return g === f;
}

export type SavingsQuery = {
  kind: "deposit" | "saving";
  /** 예금은 목돈, 적금은 월 납입액. */
  amount: number;
  months: number;
  tax: TaxKind;
  /** 순위 기준 — 기본금리(우대 없이) · 최고금리(우대조건 전부 충족) · 내 조건(F1: 고른 우대조건만). */
  basis: "base" | "max" | "mine";
  /** F1 — 내가 채울 수 있는 우대조건 칩(lib/fin-conds COND_CATEGORIES id). basis "mine"일 때 쓴다. */
  myConds?: ReadonlySet<string>;
  /** F2 — 가입 방법·대상. 없거나 "all"이면 거르지 않는다. */
  way?: "all" | JoinWay;
  target?: "all" | JoinTarget;
  interestType: "all" | InterestType;
  rsrvType: "all" | "S" | "F";
  group: GroupFilter;
  /** 가입 제한 없는 상품만(join_deny = 1). */
  openOnly: boolean;
  watchedOnly: boolean;
  watched: Set<number>;
};

export type SavingsRow = {
  product: FinProduct;
  option: Raw;
  baseRate: number;
  maxRate: number;
  interestType: InterestType;
  base: InterestResult;
  max: InterestResult;
  /** F1 — 고른 우대조건만 채웠을 때의 금리·이자(basis "mine"일 때만). */
  myRate?: number;
  mine?: InterestResult;
};

export type SavingsRanking = {
  rows: SavingsRow[];
  /** 그 기간 옵션이 없는 상품 수 · 최고한도를 넘는 상품 수(예금만) — 화면이 「왜 적은가」를 말한다. */
  noTerm: number;
  overLimit: number;
};

export function calcSavings(
  kind: "deposit" | "saving",
  amount: number,
  rate: number,
  months: number,
  type: InterestType,
  tax: TaxKind,
): InterestResult {
  return kind === "deposit"
    ? depositInterest(amount, rate, months, type, tax)
    : savingInterest(amount, rate, months, type, tax);
}

/** 한 상품에서 조건에 맞는 옵션 중 기준 금리가 가장 높은 것. */
export function pickSavingsOption(p: FinProduct, q: Pick<SavingsQuery, "months" | "interestType" | "rsrvType" | "basis">) {
  let best: { option: Raw; baseRate: number; maxRate: number; type: InterestType } | null = null;
  for (const o of p.options) {
    if (num(o.save_trm) !== q.months) continue;
    const type = str(o.intr_rate_type) === "M" ? "M" : "S";
    if (q.interestType !== "all" && type !== q.interestType) continue;
    if (q.rsrvType !== "all" && str(o.rsrv_type) !== q.rsrvType) continue;
    const baseRate = num(o.intr_rate);
    if (baseRate == null) continue;
    // 최고금리가 비어 있으면 우대가 없는 것 — 기본금리와 같다.
    const maxRate = Math.max(num(o.intr_rate2) ?? baseRate, baseRate);
    // 「내 조건」은 어느 옵션이 유리할지 조건 가산을 붙여 봐야 알아서 — 최고금리가 큰 옵션으로 고른다.
    const byMax = q.basis !== "base";
    const key = byMax ? maxRate : baseRate;
    const bestKey = best ? (byMax ? best.maxRate : best.baseRate) : -Infinity;
    if (key > bestKey) best = { option: o, baseRate, maxRate, type };
  }
  return best;
}

/**
 * 예·적금 순위 — **세후 이자 기준**이다(최고금리 숫자만으로 줄 세우지 않는다).
 * 기본값은 우대 없이 확실히 받는 기본금리 세후 이자이고, 최고금리는 「우대조건을 모두
 * 채울 때」로 따로 계산해 보여 준다. 동점이면 기본금리가 높은 쪽.
 */
export function rankSavings(items: FinProduct[], q: SavingsQuery): SavingsRanking {
  const rows: SavingsRow[] = [];
  let noTerm = 0;
  let overLimit = 0;
  for (const p of items) {
    if (!groupMatches(p.group, q.group)) continue;
    if (q.openOnly && num(p.base.join_deny) !== 1) continue;
    if (q.watchedOnly && !q.watched.has(p.id)) continue;
    if (q.way && q.way !== "all" && !joinWays(str(p.base.join_way)).has(q.way)) continue;
    if (q.target && q.target !== "all" && !joinTargets(str(p.base.join_member)).has(q.target)) continue;
    const pick = pickSavingsOption(p, q);
    if (!pick) {
      noTerm++;
      continue;
    }
    // 예금의 max_limit은 가입 한도(원). 적금은 공시마다 뜻이 달라 거르지 않고 보여만 준다.
    const limit = num(p.base.max_limit);
    if (q.kind === "deposit" && limit != null && limit > 0 && q.amount > limit) {
      overLimit++;
      continue;
    }
    const row: SavingsRow = {
      product: p,
      option: pick.option,
      baseRate: pick.baseRate,
      maxRate: pick.maxRate,
      interestType: pick.type,
      base: calcSavings(q.kind, q.amount, pick.baseRate, q.months, pick.type, q.tax),
      max: calcSavings(q.kind, q.amount, pick.maxRate, q.months, pick.type, q.tax),
    };
    if (q.basis === "mine") {
      row.myRate = myRate(pick.baseRate, pick.maxRate, p.ai?.conds, q.myConds ?? new Set());
      row.mine = calcSavings(q.kind, q.amount, row.myRate, q.months, pick.type, q.tax);
    }
    rows.push(row);
  }
  const key = (r: SavingsRow) => (q.basis === "max" ? r.max : q.basis === "mine" ? (r.mine ?? r.base) : r.base).afterTaxInterest;
  rows.sort((a, b) => key(b) - key(a) || b.baseRate - a.baseRate || a.product.productName.localeCompare(b.product.productName));
  return { rows, noTerm, overLimit };
}

// ── 대출 순위 ───────────────────────────────────────────────────────────────

export const CREDIT_BANDS: { id: string; label: string }[] = [
  { id: "crdt_grad_1", label: "900점 초과" },
  { id: "crdt_grad_4", label: "801~900점" },
  { id: "crdt_grad_5", label: "701~800점" },
  { id: "crdt_grad_6", label: "601~700점" },
  { id: "crdt_grad_10", label: "501~600점" },
  { id: "crdt_grad_11", label: "401~500점" },
  { id: "crdt_grad_12", label: "301~400점" },
  { id: "crdt_grad_13", label: "300점 이하" },
];

export type LoanQuery = {
  kind: "mortgage" | "rent" | "credit";
  amount: number;
  months: number;
  /** 분할상환 상품의 계산 방식. 만기일시 상품은 언제나 bullet. */
  method: RepayMethod;
  /**
   * 순위 금리. avg = 전월 실제 취급 평균(주담대·전세) / 내 신용점수 구간 금리(신용),
   * min = 공시 최저금리 — 광고에 쓰이는 숫자이고 대부분 받을 수 없다.
   */
  basis: "avg" | "min";
  /** 주담대·전세: 분할(D) · 만기일시(S). */
  rpayType: "D" | "S";
  rateType: "all" | "F" | "C";
  mrtgType: "all" | "A" | "E";
  creditBand: string;
  creditType: string; // "all" 또는 crdt_prdt_type
  group: GroupFilter;
  watchedOnly: boolean;
  watched: Set<number>;
};

export type LoanRow = {
  product: FinProduct;
  option: Raw;
  /** 순위와 계산에 쓴 금리. */
  rate: number;
  minRate: number | null;
  maxRate: number | null;
  avgRate: number | null;
  method: RepayMethod;
  repay: RepayResult;
};

export type LoanRanking = { rows: LoanRow[]; noRate: number };

/** 한 상품의 조건 맞는 옵션 → 순위 금리(없으면 null). */
function loanRate(p: FinProduct, o: Raw, q: LoanQuery) {
  if (q.kind === "credit") {
    if (str(o.crdt_lend_rate_type) !== "A") return null; // A = 대출금리(기준+가산 뒤 최종)
    const band = num(o[q.creditBand]);
    const avg = num(o.crdt_grad_avg);
    const rate = q.basis === "avg" ? band : avg;
    return rate == null ? null : { rate, minRate: null, maxRate: null, avgRate: avg };
  }
  if (str(o.rpay_type) !== q.rpayType) return null;
  if (q.rateType !== "all" && str(o.lend_rate_type) !== q.rateType) return null;
  if (q.kind === "mortgage" && q.mrtgType !== "all" && str(o.mrtg_type) !== q.mrtgType) return null;
  const minRate = num(o.lend_rate_min);
  const maxRate = num(o.lend_rate_max);
  const avgRate = num(o.lend_rate_avg);
  const rate = q.basis === "avg" ? avgRate : minRate;
  return rate == null ? null : { rate, minRate, maxRate, avgRate };
}

/**
 * 대출 순위 — **총이자 기준**(같은 금액·기간이면 금리 순서와 같다). 기본 금리는 실제 취급 평균
 * (신용은 내 점수 구간)이고, 공시 최저금리 순위는 「광고 금리」로 따로 고른다. 그 금리가
 * 비어 있는 상품(전월 취급 없음 등)은 순위에서 빠지고 개수만 알린다.
 */
export function rankLoans(items: FinProduct[], q: LoanQuery): LoanRanking {
  const rows: LoanRow[] = [];
  let noRate = 0;
  for (const p of items) {
    if (!groupMatches(p.group, q.group)) continue;
    if (q.watchedOnly && !q.watched.has(p.id)) continue;
    if (q.kind === "credit" && q.creditType !== "all" && str(p.base.crdt_prdt_type) !== q.creditType) continue;
    let best: (NonNullable<ReturnType<typeof loanRate>> & { option: Raw }) | null = null;
    for (const o of p.options) {
      const r = loanRate(p, o, q);
      if (r && (!best || r.rate < best.rate)) best = { ...r, option: o };
    }
    if (!best) {
      noRate++;
      continue;
    }
    const method: RepayMethod = q.kind !== "credit" && q.rpayType === "S" ? "bullet" : q.method;
    rows.push({ product: p, ...best, method, repay: loanRepayment(q.amount, best.rate, q.months, method) });
  }
  rows.sort(
    (a, b) =>
      a.repay.totalInterest - b.repay.totalInterest || a.product.productName.localeCompare(b.product.productName),
  );
  return { rows, noRate };
}

/** 신용대출 종류 — 공시에 실제로 있는 값만 고를 수 있게 목록에서 모은다. */
export function creditTypes(items: FinProduct[]): { id: string; label: string }[] {
  const seen = new Map<string, string>();
  for (const p of items) {
    const id = str(p.base.crdt_prdt_type);
    if (id && !seen.has(id)) seen.set(id, str(p.base.crdt_prdt_type_nm) || id);
  }
  return [...seen].map(([id, label]) => ({ id, label }));
}

/* ------------------------------------------------------------------------- */
/* 시장 지표 · 시장 내 위치 (2026-10-09 금융상품 분석)                            */
/* ------------------------------------------------------------------------- */

/**
 * v가 시장에서 상위 몇 %인가(1~100, 작을수록 좋다). 예·적금 금리는 높을수록, 대출 금리는 낮을수록 좋다.
 * 같은 값은 같은 순위 — 「상위 1%」가 동점 여럿에 나뉘어 붙지 않게 v보다 **좋은** 값의 수로 센다.
 */
export function marketTopPct(v: number, all: number[], higherBetter: boolean): number {
  if (all.length === 0) return 100;
  const better = all.filter((x) => (higherBetter ? x > v : x < v)).length;
  return Math.max(1, Math.ceil(((better + 1) / all.length) * 100));
}

const avg = (xs: number[]) => (xs.length ? xs.reduce((a, b) => a + b, 0) / xs.length : null);

/** 예·적금 12개월 옵션 기준 — 은행·저축은행 최고금리 평균, 최고 상품. */
export function savingsMarketStats(items: FinProduct[]) {
  const bank: number[] = [];
  const sb: number[] = [];
  let top: { name: string; rate: number } | null = null;
  let n = 0;
  for (const p of items) {
    let best: number | null = null;
    for (const o of p.options) {
      if (num(o.save_trm) !== 12) continue;
      const r = num(o.intr_rate2) ?? num(o.intr_rate);
      if (r != null && (best == null || r > best)) best = r;
    }
    if (best == null) continue;
    n++;
    if (p.group === "020000") bank.push(best);
    else if (p.group === "030300") sb.push(best);
    if (!top || best > top.rate) top = { name: `${shortCompanyName(p.companyName)} ${p.productName}`, rate: best };
  }
  return { n, bankAvgMax: avg(bank), sbAvgMax: avg(sb), top };
}

/** 대출 — 상품별 대표 평균금리(가장 낮은 옵션)의 평균·최저. */
export function loanMarketStats(rates: number[]) {
  return { n: rates.length, avg: avg(rates), min: rates.length ? Math.min(...rates) : null };
}
