import type { QAAnswer } from "@/lib/stock-qa";

/**
 * 질문 칩 4 → 10 (2026-09-27 사용자 결정 C ①). 새 여섯 질문의 답 — **저장된 사실을 문장 틀에
 * 끼운 것**이다(LLM 0). 재료는 `/stocks/{symbol}/qa`의 `more`(BE `stock_qa_more.go`),
 * 「내 보유 기준으로」만 로그인 세션의 보유·목표가로 화면에서 계산한다(공개 응답에 개인 데이터 없음).
 *
 * 재료가 없으면 칩을 숨긴다(`availableMoreQuestions`) — 해외 종목에 「외국인·기관은」을 내놓고
 * 「자료가 없습니다」라고 답하는 버튼은 함정이다.
 */

export type StockQAFlows = {
  asOf: string;
  foreign5: number;
  foreign20: number;
  institution5: number;
  institution20: number;
  foreignStreak: number;
  institutionStreak: number;
  foreignHoldRate?: number;
  foreignHoldRatePrev?: number;
  days: number;
};

export type StockQAEstimate = {
  fiscalYear: number;
  fiscalPeriod: string;
  now: number;
  nowDate: string;
  before: number;
  beforeDate: string;
};

export type StockQAValuation = {
  per?: number;
  pbr?: number;
  sectorPer?: number;
  sectorPbr?: number;
  sectorCount: number;
};

export type StockQADisclosure = { date: string; title: string };

export type StockQAMore = {
  flows?: StockQAFlows;
  estimate?: StockQAEstimate;
  valuation?: StockQAValuation;
  risks: {
    warnings: string[];
    insider?: { buys: number; sells: number; days: number };
    riskDisclosures: StockQADisclosure[];
  };
  disclosures: StockQADisclosure[];
};

export type MoreQuestionId = "analyst" | "flows" | "valuation" | "risks" | "disclosures" | "mine";

export const MORE_QUESTIONS: { id: MoreQuestionId; label: string }[] = [
  { id: "analyst", label: "애널리스트 전망은" },
  { id: "flows", label: "외국인·기관은" },
  { id: "valuation", label: "싼가 비싼가" },
  { id: "risks", label: "위험 신호는" },
  { id: "disclosures", label: "최근 공시는" },
  { id: "mine", label: "내 보유 기준으로" },
];

export function availableMoreQuestions(more: StockQAMore | undefined, loggedIn: boolean): MoreQuestionId[] {
  const out: MoreQuestionId[] = [];
  if (more?.estimate) out.push("analyst");
  if (more?.flows) out.push("flows");
  if (more?.valuation && (more.valuation.per != null || more.valuation.pbr != null)) out.push("valuation");
  if (more) out.push("risks");
  if (more && more.disclosures.length > 0) out.push("disclosures");
  if (loggedIn) out.push("mine");
  return out;
}

function md(d: string): string {
  const [, m, day] = d.split("-");
  return `${Number(m)}/${Number(day)}`;
}

function signed(v: number, digits = 1): string {
  return `${v >= 0 ? "+" : "−"}${Math.abs(v).toFixed(digits)}`;
}

/** 주식 수 — 1만 주 넘으면 만 주, 1억 주 넘으면 억 주. */
export function formatShares(n: number): string {
  const sign = n > 0 ? "+" : n < 0 ? "−" : "";
  const a = Math.abs(n);
  if (a >= 1e8) return `${sign}${(a / 1e8).toFixed(1)}억 주`;
  if (a >= 1e4) return `${sign}${(a / 1e4).toFixed(1)}만 주`;
  return `${sign}${a.toLocaleString("ko-KR")}주`;
}

function streakText(n: number): string {
  if (n >= 2) return `${n}일 연속 순매수`;
  if (n <= -2) return `${-n}일 연속 순매도`;
  return "";
}

/** 「애널리스트 전망은」 — 다음 분기 EPS 추정치의 최근 한 달 변화(해외). */
export function answerAnalyst(e: StockQAEstimate): QAAnswer {
  const change = e.before !== 0 ? ((e.now - e.before) / Math.abs(e.before)) * 100 : 0;
  const dir = Math.abs(change) < 0.5 ? "거의 그대로" : change > 0 ? "상향" : "하향";
  const lines = [
    `${e.fiscalYear} ${e.fiscalPeriod} EPS 추정치: ${md(e.beforeDate)} ${e.before.toFixed(2)} → ${md(e.nowDate)} ${e.now.toFixed(2)} (${signed(change)}%, ${dir}).`,
  ];
  if (Math.abs(change) >= 5) {
    lines.push(
      change > 0
        ? "한 달 새 5% 넘게 올랐습니다 — 실적 기대가 커지는 쪽입니다."
        : "한 달 새 5% 넘게 내렸습니다 — 실적 눈높이가 낮아지는 쪽입니다.",
    );
  }
  return { lines, source: "근거: 컨센서스 추정치 이력(매일 06:00 적재) — 추천이 아니라 시장 기대의 변화입니다" };
}

/**
 * 지분율은 원천이 **비율(0~1)** 로 준다(실측 삼성전자 0.4664 = 46.64%). 1을 넘으면 이미 %로 본다.
 */
export function holdPct(v: number): number {
  return v <= 1 ? v * 100 : v;
}

/** 「외국인·기관은」 — 5·20거래일 순매수와 연속 일수(국내). */
export function answerFlows(f: StockQAFlows): QAAnswer {
  const fs = streakText(f.foreignStreak);
  const is = streakText(f.institutionStreak);
  const lines = [
    `외국인: 5일 ${formatShares(f.foreign5)} · ${f.days}일 ${formatShares(f.foreign20)}${fs ? ` — ${fs}` : ""}.`,
    `기관: 5일 ${formatShares(f.institution5)} · ${f.days}일 ${formatShares(f.institution20)}${is ? ` — ${is}` : ""}.`,
  ];
  if (f.foreignHoldRate != null) {
    const now = holdPct(f.foreignHoldRate);
    lines.push(
      f.foreignHoldRatePrev != null
        ? `외국인 지분율 ${now.toFixed(2)}% (${f.days}거래일 전 ${holdPct(f.foreignHoldRatePrev).toFixed(2)}%).`
        : `외국인 지분율 ${now.toFixed(2)}%.`,
    );
  }
  return { lines, source: `근거: 투자자별 매매동향 (${md(f.asOf)} 기준, 주식 수)` };
}

/** 「싼가 비싼가」 — PER·PBR을 같은 업종·시장 중앙값과. */
export function answerValuation(v: StockQAValuation): QAAnswer {
  const lines: string[] = [];
  const cmp = (label: string, mine: number | undefined, sector: number | undefined, unit: string) => {
    if (mine == null) return;
    if (mine <= 0) {
      lines.push(`${label} 값이 0 이하라(적자·자본잠식) 업종과 비교할 수 없습니다.`);
      return;
    }
    if (sector == null) {
      lines.push(`${label} ${mine.toFixed(1)}${unit} — 같은 업종 비교 대상이 부족합니다.`);
      return;
    }
    const gap = ((mine - sector) / sector) * 100;
    const word = Math.abs(gap) < 10 ? "비슷합니다" : gap < 0 ? `${Math.abs(gap).toFixed(0)}% 낮습니다(싼 편)` : `${gap.toFixed(0)}% 높습니다(비싼 편)`;
    lines.push(`${label} ${mine.toFixed(1)}${unit} — 같은 업종 중앙값 ${sector.toFixed(1)}${unit}보다 ${word}.`);
  };
  cmp("PER", v.per, v.sectorPer, "배");
  cmp("PBR", v.pbr, v.sectorPbr, "배");
  if (lines.length === 0) lines.push("비교할 PER·PBR이 없습니다.");
  return {
    lines,
    source: `근거: 최신 점수 산출에 쓴 PER·PBR · 같은 업종·시장 ${v.sectorCount}종목 중앙값(적자 제외) — 싸다고 오른다는 뜻은 아닙니다`,
  };
}

/** 「위험 신호는」 — 경고 지정 · 내부자 매도 우세 · 증자/소송류 공시(최근 90일). */
export function answerRisks(r: StockQAMore["risks"]): QAAnswer {
  const lines: string[] = [];
  for (const w of r.warnings) lines.push(`시장 경고: ${w}`);
  if (r.insider && r.insider.sells > r.insider.buys && r.insider.sells >= 2) {
    lines.push(`최근 ${r.insider.days}일 내부자 매도 ${r.insider.sells}건 · 매수 ${r.insider.buys}건 — 매도 우세.`);
  }
  for (const d of r.riskDisclosures) lines.push(`${md(d.date)} 공시: ${d.title}`);
  if (lines.length === 0) {
    lines.push("최근 90일 동안 규칙에 걸린 위험 신호(시장 경고·내부자 매도 우세·증자/전환사채/소송류 공시)가 없습니다.");
  }
  return { lines, source: "근거: 거래소 경고 · 내부자 거래(DART·SEC) · 공시 제목(DART)" };
}

/** 「최근 공시는」 — 최근 30일 공시 제목(국내). */
export function answerDisclosures(list: StockQADisclosure[]): QAAnswer {
  return {
    lines: list.map((d) => `${md(d.date)} ${d.title}`),
    source: "근거: DART 공시(매일 적재) — 제목만, 내용은 공시 원문에서",
  };
}

export type MineInput = {
  quantity?: number;
  avgPrice?: number;
  price?: number;
  currency?: "KRW" | "USD";
  targetPrice?: number;
  stopPrice?: number;
};

function money(v: number, currency: "KRW" | "USD"): string {
  return currency === "USD"
    ? `$${v.toLocaleString("en-US", { maximumFractionDigits: 2 })}`
    : `${Math.round(v).toLocaleString("ko-KR")}원`;
}

/** 「내 보유 기준으로」 — 평단 대비 · 평가손익 · 목표가/손절가까지 남은 거리. */
export function answerMine(m: MineInput): QAAnswer {
  const cur = m.currency ?? "KRW";
  const lines: string[] = [];
  const held = (m.quantity ?? 0) > 0;
  if (!held) {
    lines.push("내 포트폴리오에 없는 종목입니다.");
  } else if (m.avgPrice && m.price) {
    const rate = ((m.price - m.avgPrice) / m.avgPrice) * 100;
    const pnl = (m.price - m.avgPrice) * (m.quantity ?? 0);
    lines.push(
      `평단 ${money(m.avgPrice, cur)} · ${m.quantity}주 → 지금 ${money(m.price, cur)} (${signed(rate)}%), 평가손익 ${pnl >= 0 ? "+" : "−"}${money(Math.abs(pnl), cur)}.`,
    );
  } else {
    lines.push(`${m.quantity}주 보유 — 평단이 없어 손익은 계산하지 않습니다.`);
  }
  if (m.price && m.targetPrice) {
    const to = ((m.targetPrice - m.price) / m.price) * 100;
    lines.push(
      Math.abs(to) < 0.05
        ? `목표가 ${money(m.targetPrice, cur)}에 닿아 있습니다.`
        : `목표가 ${money(m.targetPrice, cur)}까지 ${signed(to)}%.`,
    );
  }
  if (m.price && m.stopPrice) {
    const to = ((m.stopPrice - m.price) / m.price) * 100;
    lines.push(`손절가 ${money(m.stopPrice, cur)}까지 ${signed(to)}%.`);
  }
  if (!held && !m.targetPrice) lines.push("보유를 등록하거나 목표가를 적어 두면 여기서 거리를 알려 드립니다.");
  return { lines, source: "근거: 내 포트폴리오 · 목표가 예측(내 계정에만 보입니다)" };
}
