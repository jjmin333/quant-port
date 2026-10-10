import { apiRequest } from "@/lib/api-client";
import type { EarningsTabData } from "@/lib/earnings-types";

/**
 * 종목 질문 칩 — 규칙 기반 Q&A (2026-09-24 사용자 결정: LLM 없이, 종목 개요의 질문 칩).
 *
 * 답은 **저장된 사실을 문장 틀에 끼운 것**이다. 모델이 지어낼 자리가 없는 대신, 모르는 것은
 * 모른다고 말해야 한다 — 「최근 30일 규칙에 걸린 변화가 없다」는 답이지 실패가 아니다.
 * 답마다 근거(날짜·출처)를 붙인다.
 */

export type StockQAChange = {
  date: string;
  kind: string;
  direction: number;
  title: string;
  explained?: boolean;
  magnitude?: number;
};

export type StockQAPeer = { symbol: string; name: string; score: number; grade?: string };

export type StockQA = {
  symbol: string;
  lastMove?: { date: string; changeRate: number };
  changes: StockQAChange[];
  sectorLabel?: string;
  peers: StockQAPeer[];
  /** 질문 칩 확장(2026-09-27) 재료 — 없으면 구버전 서버. */
  more?: import("@/lib/stock-qa-more").StockQAMore;
};

export type QAAnswer = { lines: string[]; source: string };

export type QuestionId = "why" | "score" | "earnings" | "peers";

export const QUESTIONS: { id: QuestionId; label: string }[] = [
  { id: "why", label: "왜 움직였나" },
  { id: "score", label: "점수가 왜 이런가" },
  { id: "earnings", label: "다음 실적은" },
  { id: "peers", label: "비슷한 종목은" },
];

export function fetchStockQA(symbol: string): Promise<StockQA> {
  return apiRequest<StockQA>(`/api/v1/stocks/${encodeURIComponent(symbol)}/qa`);
}

function md(d: string): string {
  const [, m, day] = d.split("-");
  return `${Number(m)}/${Number(day)}`;
}

function pct(v: number): string {
  return `${v >= 0 ? "+" : ""}${v.toFixed(2)}%`;
}

/** 「왜 움직였나」 — 마지막 등락 + 최근 30일 변화 감지 상위. */
export function answerWhy(qa: StockQA): QAAnswer {
  const lines: string[] = [];
  if (qa.lastMove) lines.push(`마지막 거래일(${md(qa.lastMove.date)}) ${pct(qa.lastMove.changeRate)}.`);
  if (qa.changes.length === 0) {
    lines.push(
      "최근 30일 동안 규칙에 걸린 큰 변화(급등락·거래량·점수·뉴스 사건·공시·실적·내부자)가 없습니다.",
    );
  } else {
    for (const c of qa.changes.slice(0, 3)) lines.push(`${md(c.date)} ${c.title}`);
    const unexplained = qa.changes.find((c) => c.kind === "price_move" && c.explained === false);
    if (unexplained) {
      lines.push(
        `${md(unexplained.date)} 가격 움직임은 같은 날 뉴스·공시에서 뚜렷한 원인을 찾지 못했습니다.`,
      );
    }
  }
  return { lines, source: "근거: 일별 종가 · 변화 감지(매일 00:25)" };
}

// 조사까지 붙여 둔다 — 「영업이익가」를 막는다.
const METRIC_SUBJECT: Record<string, string> = {
  opIncome: "영업이익이",
  revenue: "매출이",
  eps: "EPS가",
  netIncome: "순이익이",
};

const STATUS_LABEL: Record<string, string> = {
  confirmed: "확정",
  scheduled: "예정",
  estimated: "추정",
};

/** 「다음 실적은」 — 다가오는 발표 + 지난 발표의 서프라이즈. */
export function answerEarnings(tab: EarningsTabData): QAAnswer {
  const lines: string[] = [];
  const next = tab.calendar.find((c) => c.kind === "upcoming");
  if (next) {
    const status = next.dateStatus ? STATUS_LABEL[next.dateStatus] ?? "" : "";
    lines.push(
      `다음 발표: ${md(next.date)} ${next.label}${status ? `(${status})` : ""}` +
        (next.estimateBasis ? ` — ${next.estimateBasis}` : ""),
    );
  } else {
    lines.push("다음 발표일은 아직 알려지지 않았습니다.");
  }
  // 「지난」 발표만 — 발표 예정 행(값 없음)이 섞여 들어와 다음 발표와 같은 날짜가 「지난 발표」로 나왔다(AAPL 10/29, 2026-10-10).
  const today = new Date().toLocaleDateString("en-CA", { timeZone: "Asia/Seoul" });
  const last = tab.announcements.filter((a) => a.date <= today).sort((a, b) => b.date.localeCompare(a.date))[0];
  if (last) {
    const head = `지난 발표: ${md(last.date)} ${last.fiscalYear} ${last.fiscalPeriod}`;
    if (last.surprise?.pct != null) {
      const metric = METRIC_SUBJECT[last.surprise.metric] ?? `${last.surprise.metric}이`;
      const word = last.surprise.pct >= 0 ? "상회" : "하회";
      lines.push(`${head} — ${metric} 예상을 ${Math.abs(last.surprise.pct).toFixed(1)}% ${word}`);
    } else {
      lines.push(`${head} — 비교할 예상치가 없었습니다.`);
    }
  }
  return { lines, source: "근거: 실적 일정(FnGuide · Finnhub · SEC 8-K)" };
}

/** 「비슷한 종목은」 — 같은 섹터·같은 시장에서 시가총액이 가까운 다섯. */
export function answerPeers(qa: StockQA): QAAnswer {
  if (!qa.sectorLabel) {
    return { lines: ["섹터 분류가 아직 없어 비교 종목을 고를 수 없습니다."], source: "근거: 섹터 분류" };
  }
  if (qa.peers.length === 0) {
    return {
      lines: [`같은 섹터(${qa.sectorLabel})에 점수가 있는 다른 종목이 없습니다.`],
      source: "근거: 섹터 분류 · AI 점수",
    };
  }
  return {
    lines: [`같은 섹터(${qa.sectorLabel}) · 같은 시장에서 규모(시가총액)가 가까운 종목과 그 AI 점수입니다.`],
    source: "근거: 섹터 분류(DART·SEC) · AI 점수 — 추천이 아니라 비교 목록입니다",
  };
}
