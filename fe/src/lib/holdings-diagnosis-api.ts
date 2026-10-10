import { apiRequest } from "@/lib/api-client";

/**
 * 보유 이유 점검 — `be/internal/model/holdings_diagnosis.go`의 거울 (V2 요구 6).
 *
 * # 답이 넷인 이유
 *
 * 「판단 보류」가 있어야 나머지 셋이 정직해진다. 근거가 하나뿐인데 「유지」라고 적으면
 * 그것은 판단이 아니라 기본값인데, 화면에서는 둘이 똑같이 생겼다.
 *
 * # 수익률은 판정에 쓰이지 않는다
 *
 * 「올랐으니 이유가 강화됐다」는 결과를 근거로 되돌리는 말이다. 수익률은 **대조용**으로
 * 만 나가므로 화면에서도 판정 옆이 아니라 줄 끝에 둔다 — 실측에서 삼성전자는 +37.6%에
 * 「약화」, 엔비디아는 +21.8%에 「유지」다.
 */

export type DiagnosisVerdict = "stronger" | "steady" | "weaker" | "unknown";

export type DiagnosisSignalId = "score_trend" | "earnings" | "insider";

export type DiagnosisSignal = {
  id: DiagnosisSignalId;
  label: string;
  /** false면 `direction`은 뜻이 없다 — 재지 못한 것이지 방향이 없는 것이 아니다. */
  available: boolean;
  /** "no_data" | "no_estimate" | "out_of_retention" | "not_collected_yet" */
  reason?: string;
  /** -1 악화 · 0 중립 · +1 개선. */
  direction: number;
  detail?: string;
  /** 이 신호 하나만으로 방향을 말할 만큼 컸는가(문턱의 두 배). */
  strong?: boolean;
};

export type DiagnosisScore = {
  current: number | null;
  grade?: string;
  asOf?: string;
  /** 매수일 이후 첫 회차. 매수가 보관 창보다 오래됐으면 창의 첫날이다. */
  fromDate?: string;
  fromValue?: number;
  delta?: number;
  driver?: string;
};

export type DiagnosisEarnings = {
  count: number;
  period?: string;
  date?: string;
  /** 없으면 「예상대로였다」가 아니라 「비교할 예상이 없었다」. */
  surprise?: number;
};

export type DiagnosisInsider = {
  count: number;
  buyShares: number;
  sellShares: number;
  netShares: number;
};

export type HoldingDiagnosis = {
  symbol: string;
  name?: string;
  quantity: number;
  avgPrice?: number;
  currency?: string;
  lastClose?: number;
  lastCloseDate?: string;
  returnPct?: number;
  firstBought?: string;
  holdingDays?: number;
  verdict: DiagnosisVerdict;
  signals: DiagnosisSignal[];
  /** 실제로 쓴 근거 수. 「근거 1」을 적을 수 있어야 사용자가 무게를 스스로 정한다. */
  signalCount: number;
  score?: DiagnosisScore;
  earnings?: DiagnosisEarnings;
  insider?: DiagnosisInsider;
  scoreAtPurchase?: number;
  scoreAtPurchaseReason?: string;
};

export type HoldingsDiagnosisSummary = {
  stronger: number;
  steady: number;
  weaker: number;
  unknown: number;
};

export type HoldingsDiagnosisData = {
  items: HoldingDiagnosis[];
  summary: HoldingsDiagnosisSummary;
  scoreRetentionFrom?: string;
  note?: string;
};

export async function fetchHoldingsDiagnosisClient(): Promise<HoldingsDiagnosisData | null> {
  return apiRequest<HoldingsDiagnosisData | null>(
    "/api/v1/user/holdings/diagnosis",
    undefined,
    "보유 진단을 불러오지 못했습니다",
  );
}

/* ------------------------------------------------------------------------- */
/* 표시 규칙                                                                  */
/* ------------------------------------------------------------------------- */

export const VERDICT_LABEL: Record<DiagnosisVerdict, string> = {
  stronger: "강화",
  steady: "유지",
  weaker: "약화",
  unknown: "판단 보류",
};

/**
 * 판정 색 — **빨강·파랑을 쓰지 않는다.**
 *
 * 이 앱에서 빨강은 상승, 파랑은 하락이다. 판정에 그 둘을 쓰면 한 줄 안에서 색 하나가
 * 두 가지를 말하게 되고, 「약화인데 왜 빨강(상승)이지」가 생긴다. 그래서 점수 등급과
 * 같은 초록/주황 계열을 쓰고, 「유지」는 아예 색을 뺀다.
 */
export const VERDICT_BADGE: Record<DiagnosisVerdict, string> = {
  stronger: "bg-[var(--score-high)]/12 text-[var(--score-high)]",
  steady: "bg-muted text-[var(--text-muted)]",
  weaker: "bg-[var(--score-low)]/12 text-[var(--score-low)]",
  unknown: "border border-dashed border-[var(--border)] text-[var(--text-muted)]",
};

/** 약화가 먼저다 — 확인이 필요한 것이 위에 있어야 한다. BE 정렬과 같은 순서. */
export const VERDICT_ORDER: DiagnosisVerdict[] = ["weaker", "unknown", "steady", "stronger"];

/**
 * 신호 방향 기호 — 셋을 구분한다.
 *
 *   ▲ ▼  방향이 있음
 *   =    재 봤으나 방향이 없음
 *   −    잴 수 없었음
 *
 * 가운데 둘을 같은 기호로 적으면 「내부자가 거래를 안 했다」와 「내부자 거래를 아직
 * 안 모았다」가 화면에서 같아진다 (요구 8).
 */
export function signalMark(signal: DiagnosisSignal): { mark: string; tone: string } {
  if (!signal.available) return { mark: "−", tone: "text-[var(--text-muted)]" };
  if (signal.direction > 0) return { mark: "▲", tone: "text-[var(--score-high)]" };
  if (signal.direction < 0) return { mark: "▼", tone: "text-[var(--score-low)]" };
  return { mark: "=", tone: "text-[var(--text-muted)]" };
}

/** 왜 못 썼는지. 빈칸으로 두면 화면이 「문제 없음」으로 읽힌다. */
export function signalReasonLabel(signal: DiagnosisSignal): string {
  if (signal.detail) return signal.detail;
  switch (signal.reason) {
    case "no_estimate":
      return "비교할 컨센서스가 없습니다";
    case "out_of_retention":
      return "보관 기간 밖입니다";
    case "not_collected_yet":
      return "아직 수집 전입니다";
    case "no_data":
      return "보유 기간 중 기록이 없습니다";
    default:
      return "-";
  }
}

/** 부호를 붙인 점수 변화. 0은 `+0`이 아니라 `0`이다. */
export function formatScoreDelta(delta: number | undefined): string {
  if (typeof delta !== "number") return "-";
  if (delta === 0) return "0";
  return `${delta > 0 ? "+" : "−"}${Math.abs(delta)}`;
}

/** 수익률. 0은 `−0.00%`가 되지 않는다. */
export function formatReturnPct(pct: number | undefined): string {
  if (typeof pct !== "number") return "-";
  if (pct === 0) return "0.00%";
  return `${pct > 0 ? "+" : "−"}${Math.abs(pct).toFixed(2)}%`;
}

/** `2026-08-11` → `08-11`. 연도는 카드 머리에 한 번만 나온다. */
export function trimDate(date: string | undefined): string {
  if (!date) return "";
  return date.length >= 10 ? date.slice(5) : date;
}
