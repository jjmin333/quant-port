import { apiRequest } from "@/lib/api-client";

/**
 * 목표가 시나리오 — `be/internal/model/price_scenario.go`의 거울 (V2 요구 5).
 *
 * # 하나의 숫자를 찍지 않는 이유
 *
 * 실측에서 90일 뒤 실제 가격은 현재가에서 **평균 24%** 떨어져 있었습니다. 그 오차를
 * 아는 채로 한 숫자를 내놓으면 그 정밀도는 거짓입니다.
 *
 * # 적중률은 **방법의 성적**입니다
 *
 * 종목별 표본은 6건뿐이라 적중률이라 부를 수 없습니다. 화면이 그 구분을 잃으면
 * 「이 종목은 70% 맞는다」로 읽히는데, 그것은 측정한 적 없는 문장입니다.
 */

export type ScenarioKind = "bear" | "base" | "bull";

export type ScenarioPoint = {
  kind: ScenarioKind;
  label: string;
  price: number;
  /** 현재가 대비 %. 중립은 언제나 0입니다. */
  changePct: number;
  /** 이 가격이면 받게 되는 배수. PER이 없는 종목에서는 없습니다. */
  impliedPer?: number;
};

export type ScenarioAccuracy = {
  hitRate: number;
  baseErrorPct: number;
  missBelowPct: number;
  missAbovePct: number;
  samples: number;
  symbols: number;
  dates: number;
  measuredAt?: string;
  market?: string;
};

export type PriceScenario = {
  symbol: string;
  name?: string;
  /** 종가 기준일입니다 — 실시간 시세가 아닙니다. */
  asOf: string;
  close: number;
  currency?: string;
  horizonDays: number;
  /** 이 종목의 90일 변동성(%). 밴드 폭이 곧 이 값입니다. */
  volatilityPct: number;
  points?: ScenarioPoint[];
  currentPer?: number;
  accuracy?: ScenarioAccuracy;
  /** "not_enough_history" | "no_close" */
  unavailable?: string;
};

export async function fetchPriceScenarioClient(symbol: string): Promise<PriceScenario | null> {
  return apiRequest<PriceScenario | null>(
    `/api/v1/stocks/${encodeURIComponent(symbol)}/scenario`,
    undefined,
    "시나리오를 불러오지 못했습니다",
  );
}

/* ------------------------------------------------------------------------- */
/* 표시 규칙                                                                  */
/* ------------------------------------------------------------------------- */

/**
 * 시나리오 색 — **초록/주황이 아니라 손익색**을 씁니다.
 *
 * 다른 카드와 반대 선택입니다. 여기서 약세·강세는 **가격의 방향**이라, 이 앱에서
 * 그 뜻을 이미 가진 빨강(상승)·파랑(하락)이 맞습니다. 판정(강화/약화)은 방향이
 * 아니라 평가라서 점수색을 썼습니다.
 */
export const SCENARIO_TONE: Record<ScenarioKind, string> = {
  bear: "text-[var(--loss)]",
  base: "text-[var(--text-primary)]",
  bull: "text-[var(--profit)]",
};

export function scenarioUnavailableLabel(reason: string | undefined): string {
  switch (reason) {
    case "not_enough_history":
      return "시나리오를 만들 만큼의 가격 이력이 아직 없습니다.";
    case "no_close":
      return "종가가 아직 없습니다.";
    default:
      return "시나리오를 만들 수 없습니다.";
  }
}

/** 가격. 통화에 따라 자릿수를 바꿉니다 — 달러에 원화 자릿수를 쓰면 1원이 1센트가 됩니다. */
export function formatScenarioPrice(price: number, currency?: string): string {
  const usd = (currency ?? "").toUpperCase() === "USD";
  return price.toLocaleString("ko-KR", {
    minimumFractionDigits: usd ? 2 : 0,
    maximumFractionDigits: usd ? 2 : 0,
  });
}

/** 등락률. 0은 `−0.0%`가 되지 않습니다. */
export function formatScenarioPct(pct: number): string {
  if (pct === 0) return "0.0%";
  return `${pct > 0 ? "+" : "−"}${Math.abs(pct).toFixed(1)}%`;
}

/**
 * 세 점을 **가격 축** 위의 위치(0~100%)로.
 *
 * 세 칸을 같은 간격으로 그리면 「강세가 약세보다 멀다」는 사실이 사라집니다. 로그
 * 밴드라 위쪽 거리가 언제나 더 먼데, 그 비대칭이 이 카드가 말하려는 것의 절반입니다.
 */
export function scenarioAxisPositions(points: ScenarioPoint[]): Map<ScenarioKind, number> {
  const out = new Map<ScenarioKind, number>();
  if (points.length === 0) return out;
  const prices = points.map((p) => p.price);
  const lo = Math.min(...prices);
  const hi = Math.max(...prices);
  const span = hi - lo;
  /*
    양끝을 4%~96%로 들입니다. **비율은 그대로입니다** — 일차 변환이라 세 점의
    간격 비가 보존되고, 대신 첫·마지막 눈금이 막대 테두리에 걸려 잘리지 않습니다.
  */
  const INSET = 4;
  for (const p of points) {
    if (span <= 0) {
      // 폭이 0이면(변동성 0) 셋이 한자리라 가운데로 모읍니다.
      out.set(p.kind, 50);
      continue;
    }
    out.set(p.kind, INSET + ((p.price - lo) / span) * (100 - INSET * 2));
  }
  return out;
}
