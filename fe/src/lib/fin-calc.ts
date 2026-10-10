/**
 * 예·적금 이자와 대출 상환 계산 (2026-09-29, 금융상품 비교).
 *
 * 식은 금감원 「금융상품 한눈에」 계산기와 같은 월 단위 식이다. 은행의 실제 계산은
 * 일할·원 단위 절사·우대조건 적용 시점 때문에 몇 원~몇백 원 다를 수 있고, 화면은 그 사실을
 * 함께 적는다. 금리는 연 %(3.5 = 연 3.5%), 기간은 개월, 금액은 원.
 */

export type InterestType = "S" | "M"; // 단리 · 월복리 (공시 intr_rate_type)

/** 이자소득세 — 일반 15.4%(소득세 14% + 지방소득세 1.4%), 세금우대 9.5%, 비과세 0. */
export const TAX_RATES = { normal: 0.154, preferential: 0.095, exempt: 0 } as const;
export type TaxKind = keyof typeof TAX_RATES;
export const TAX_LABEL: Record<TaxKind, string> = {
  normal: "일반과세 15.4%",
  preferential: "세금우대 9.5%",
  exempt: "비과세",
};

export type InterestResult = {
  principal: number;
  interest: number;
  tax: number;
  afterTaxInterest: number;
  maturity: number;
};

function finish(principal: number, rawInterest: number, tax: TaxKind): InterestResult {
  const interest = Math.round(rawInterest);
  const t = Math.round(interest * TAX_RATES[tax]);
  return {
    principal,
    interest,
    tax: t,
    afterTaxInterest: interest - t,
    maturity: principal + interest - t,
  };
}

/** 정기예금 — 목돈 `amount`를 `months` 동안. */
export function depositInterest(
  amount: number,
  annualRatePct: number,
  months: number,
  type: InterestType,
  tax: TaxKind = "normal",
): InterestResult {
  const r = annualRatePct / 100;
  const raw = type === "M" ? amount * ((1 + r / 12) ** months - 1) : (amount * r * months) / 12;
  return finish(amount, raw, tax);
}

/**
 * 적금 — 매월 초 `monthly`씩 `months`회. 첫 달 납입분은 `months`개월, 마지막은 1개월 이자가 붙는다.
 * 자유적립식도 **매달 같은 금액을 넣는다고 가정**한다.
 */
export function savingInterest(
  monthly: number,
  annualRatePct: number,
  months: number,
  type: InterestType,
  tax: TaxKind = "normal",
): InterestResult {
  const r = annualRatePct / 100;
  const i = r / 12;
  let raw: number;
  if (type === "M" && i > 0) {
    raw = monthly * (1 + i) * (((1 + i) ** months - 1) / i) - monthly * months;
  } else {
    raw = (monthly * i * months * (months + 1)) / 2;
  }
  return finish(monthly * months, raw, tax);
}

export type RepayMethod = "equal_payment" | "equal_principal" | "bullet";
export const REPAY_LABEL: Record<RepayMethod, string> = {
  equal_payment: "원리금균등",
  equal_principal: "원금균등",
  bullet: "만기일시",
};

export type RepayRow = { month: number; principal: number; interest: number; payment: number; balance: number };
export type RepayResult = {
  firstPayment: number;
  lastPayment: number;
  totalInterest: number;
  totalPayment: number;
  rows: RepayRow[];
};

/** 대출 상환 — 고정금리 가정, 매월 말 상환. 행마다 원 단위 반올림, 마지막 달이 잔액을 정리한다. */
export function loanRepayment(
  amount: number,
  annualRatePct: number,
  months: number,
  method: RepayMethod,
): RepayResult {
  const i = annualRatePct / 100 / 12;
  const rows: RepayRow[] = [];
  let balance = amount;
  const level = i > 0 ? (amount * i) / (1 - (1 + i) ** -months) : amount / months;
  for (let m = 1; m <= months; m++) {
    const interest = Math.round(balance * i);
    let principal: number;
    if (method === "bullet") principal = m === months ? balance : 0;
    else if (m === months) principal = balance;
    else if (method === "equal_principal") principal = Math.round(amount / months);
    else principal = Math.round(level) - interest;
    balance -= principal;
    rows.push({ month: m, principal, interest, payment: principal + interest, balance });
  }
  const totalInterest = rows.reduce((s, r) => s + r.interest, 0);
  return {
    firstPayment: rows[0]?.payment ?? 0,
    lastPayment: rows[rows.length - 1]?.payment ?? 0,
    totalInterest,
    totalPayment: amount + totalInterest,
    rows,
  };
}

/** 원 → 「1,234,567원」. */
export function won(v: number | null | undefined): string {
  if (v == null || !Number.isFinite(v)) return "-";
  return `${Math.round(v).toLocaleString("ko-KR")}원`;
}

/** 원 → 「1,234만원」 (만 단위 미만은 원으로). */
export function wonShort(v: number | null | undefined): string {
  if (v == null || !Number.isFinite(v)) return "-";
  if (Math.abs(v) >= 1e8) {
    const eok = Math.floor(v / 1e8);
    const man = Math.round((v % 1e8) / 1e4);
    return man ? `${eok}억 ${man.toLocaleString("ko-KR")}만원` : `${eok}억원`;
  }
  if (Math.abs(v) >= 1e4) return `${Math.round(v / 1e4).toLocaleString("ko-KR")}만원`;
  return won(v);
}

export function pct(v: number | null | undefined, digits = 2): string {
  return v == null || !Number.isFinite(v) ? "-" : `${v.toFixed(digits)}%`;
}

/* ------------------------------------------------------------------------- */
/* 상품 행 → 계산기 (2026-10-01 요청 5 — 「이 금리로 계산」)                    */
/* ------------------------------------------------------------------------- */

export type CalcTab = "deposit" | "saving" | "loan";
export type CalcPrefill = { amount?: number; months?: number; rate?: number; type?: InterestType; method?: RepayMethod };


/** 목록의 금액·기간·방식을 주소 조각으로 — 상세·계산기가 같은 값으로 열린다(`readCalcPrefill`이 읽는다). */
export function prefillQuery(v: CalcPrefill, tab?: CalcTab): string {
  const q = new URLSearchParams(tab ? { tab } : {});
  if (v.amount != null) q.set("amount", String(Math.round(v.amount)));
  if (v.months != null) q.set("months", String(v.months));
  if (v.rate != null) q.set("rate", String(Math.round(v.rate * 100) / 100));
  if (v.type) q.set("type", v.type);
  if (v.method) q.set("method", v.method);
  return q.toString();
}

/** 주소의 값 — 범위를 벗어나거나 숫자가 아니면 버린다(손으로 고친 주소가 계산기를 깨지 않게). */
export function readCalcPrefill(get: (k: string) => string | null): CalcPrefill {
  const n = (k: string, min: number, max: number) => {
    const raw = get(k);
    if (raw == null || raw.trim() === "") return undefined;
    const v = Number(raw);
    return Number.isFinite(v) && v >= min && v <= max ? v : undefined;
  };
  const type = get("type");
  const method = get("method");
  return {
    amount: n("amount", 1, 10_000_000_000),
    months: n("months", 1, 600),
    rate: n("rate", 0, 30),
    type: type === "S" || type === "M" ? type : undefined,
    method: method === "equal_payment" || method === "equal_principal" || method === "bullet" ? method : undefined,
  };
}

/**
 * 갈아타기 월 이자 차이(F3, 2026-10-03) — 잔액 × (내 금리 − 상품 금리) ÷ 12. 양수 = 매달 그만큼 덜 낸다.
 * 첫 달 이자 기준의 어림값이다(상환으로 잔액이 줄면 차이도 준다 · 중도상환수수료 제외).
 */
export function monthlyInterestGap(balance: number, myRate: number, productRate: number): number {
  return Math.round((balance * (myRate - productRate)) / 100 / 12);
}
