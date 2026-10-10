import { apiRequest } from "@/lib/api-client";
import { cachedGet } from "@/lib/fetch-cache";
import type { ScoreAxisId, ScoreBreakdown } from "@/lib/stock-score-api";

/**
 * 점수 설명 — `be/internal/model/score_explain.go`의 거울.
 *
 * # 입구가 둘인 이유
 *
 *   · `fetchScoreExplainClient` — 한 종목을 **펼쳤을 때** 한 번. 15KB.
 *   · `fetchScoreChangesClient` — 목록이 **행마다** 필요할 때. 10종목 2.6KB.
 *
 * 큰 쪽을 목록에서 부르면 20종목에 300KB와 스무 번의 조회가 된다. 둘은 같은 답을
 * 하지만 크기가 다른 자료이고, 화면이 그 구분을 잊지 않도록 타입도 따로 둔다.
 */

export type ScoreChangeWindow = "1d" | "1w";
export type ScoreChangeDriverKind = "axis" | "availability" | "shrink" | "weight";
export type ScoreDataBasis = "actual" | "partial";

export type ScoreMetricExplain = {
  key: string;
  label: string;
  axis: ScoreAxisId;
  /** "%" | "x" | "score" — 13.07을 13.07%로, 2.33을 2.33배로 적기 위한 값. */
  unit: string;
  /**
   * **원값**의 방향이다(백분위가 아니라). 부채비율은 낮을수록 좋아서 29.9%가
   * 백분위 87.5로 뒤집혀 들어간다 — 화면이 그 뒤집힘을 말하지 않으면 「낮은데 왜
   * 점수가 높지」가 생긴다.
   */
  higherIsBetter: boolean;
  raw?: number;
  /** 0~100, 높을수록 좋음. 화면에는 `상위 N%`(=100−백분위)로 적는다. */
  percentile?: number;
  /** 백분위를 실제로 잰 비교 집단. 놀라운 순위는 거의 항상 이 칸이 설명한다. */
  rankedIn?: string;
};

export type ScoreAxisExplain = {
  axis: ScoreAxisId;
  label: string;
  /** null은 「데이터 없음」이다 — 0(최하위)과 뜻이 정반대다. */
  score: number | null;
  weight: number;
  /** 데이터가 있는 축들 사이에서 다시 나눈 비중(%). 실제로 작동한 값이다. */
  weightShare: number;
  /** 최종 점수에 보탠 점수. 축 기여도의 합 + 축소 보정 = 최종 점수. */
  contribution?: number;
  available: boolean;
  /** "no_data" | "not_applicable" | "weight_zero" */
  missingReason?: string;
  metrics?: ScoreMetricExplain[];
};

export type ScoreMetricDelta = {
  key: string;
  label: string;
  axis?: ScoreAxisId;
  unit: string;
  higherIsBetter: boolean;
  fromRaw?: number;
  toRaw?: number;
  fromPercentile?: number;
  toPercentile?: number;
  percentileDelta?: number;
};

export type ScoreChangeDriver = {
  kind: ScoreChangeDriverKind;
  axis?: ScoreAxisId;
  label: string;
  /** 숫자로 지은 한 문장. LLM이 쓴 것이 아니다. */
  detail: string;
  /** 최종 점수 단위(점). 드라이버 합 = `exactDelta`. */
  effect: number;
  from?: number;
  to?: number;
  metrics?: ScoreMetricDelta[];
};

export type ScoreChange = {
  window: ScoreChangeWindow;
  label: string;
  fromDate?: string;
  toDate: string;
  available: boolean;
  /** "no_prior_row" | "score_withheld" */
  reason?: string;
  fromScore?: number;
  toScore?: number;
  delta?: number;
  /** 반올림 전 실제 변화. 드라이버는 이 값에 맞아떨어진다. */
  exactDelta: number;
  drivers: ScoreChangeDriver[];
  /** 표시 정수와 실제 변화의 차. 유일한 미설명분이고, 숨기지 않는다. */
  roundingResidual: number;
};

export type ScoreComposite = {
  raw: number;
  final: number;
  shrinkFactor: number;
  usedWeight: number;
  totalWeight: number;
  coverageFloor: number;
  shrinkFull: number;
  withheld: boolean;
};

export type ScoreRejectedMetric = {
  key: string;
  label: string;
  axis: ScoreAxisId;
  unit: string;
  value: number;
  domainLo: number;
  domainHi: number;
  reason: string;
};

export type ScoreHistoryPoint = {
  tradeDate: string;
  score: number | null;
  coverage: number;
  breakdown: ScoreBreakdown;
};

export type ScoreProvenance = {
  asOf: string;
  computedAt: string;
  basis: ScoreDataBasis;
  sources: string[];
  note?: string;
};

export type ScoreExplainData = {
  symbol: string;
  name?: string;
  tradeDate: string;
  score: number | null;
  grade?: string;
  scoreKind: string;
  coverage: number;
  peerGroup?: string;
  peerPct?: number;
  close?: number;
  marketCap?: number;
  barCount?: number;
  computedAt: string;
  weights: Record<ScoreAxisId, number>;
  weightSource: string;
  composite: ScoreComposite;
  axes: ScoreAxisExplain[];
  rejected?: ScoreRejectedMetric[];
  changes: ScoreChange[];
  history: ScoreHistoryPoint[];
  provenance: ScoreProvenance;
};

/** 목록 행에 실리는 한 줄짜리. `topDriver`가 없으면 「변화 없음」이다. */
export type ScoreChangeSummary = {
  symbol: string;
  tradeDate: string;
  score: number | null;
  available: boolean;
  reason?: string;
  fromDate?: string;
  fromScore?: number;
  delta?: number;
  topDriver?: ScoreChangeDriver;
};

/**
 * 한 종목의 설명 전체.
 *
 * 점수는 하루 한 번 도는 배치의 산물이라 같은 방문 안에서 다시 물어도 값이 같다 —
 * 탭을 오가며 다시 마운트되는 자리라 캐시가 필요하다(`fetchStockScoreClient`와 같은 60초).
 */
export function fetchScoreExplainClient(symbol: string): Promise<ScoreExplainData | null> {
  const path = `/api/v1/stocks/${encodeURIComponent(symbol)}/score/explain`;
  return cachedGet(
    path,
    () => apiRequest<ScoreExplainData | null>(path, undefined, "점수 근거 조회 실패"),
    60_000,
  );
}

/**
 * 여러 종목의 변화 한 줄씩. **목록은 이쪽만 부른다.**
 *
 * 서버 상한과 같은 200으로 자른다 — 넘겨 보내면 서버가 조용히 잘라 뒤쪽 종목만
 * 설명이 빈 채로 그려진다.
 */
export async function fetchScoreChangesClient(symbols: string[]): Promise<ScoreChangeSummary[]> {
  const list = [...new Set(symbols.map((s) => s.trim().toUpperCase()).filter(Boolean))].slice(
    0,
    200,
  );
  if (list.length === 0) return [];
  const data = await apiRequest<ScoreChangeSummary[] | null>(
    `/api/v1/stocks/scores/changes?symbols=${encodeURIComponent(list.join(","))}`,
    undefined,
    "점수 변화 조회 실패",
  );
  return data ?? [];
}

/* ------------------------------------------------------------------------- */
/* 표시 규칙                                                                  */
/* ------------------------------------------------------------------------- */

/**
 * 백분위(높을수록 좋음)를 사람이 쓰는 「상위 N%」로 뒤집는다.
 *
 * 1 아래로는 내려가지 않는다 — 한 그룹의 1등은 100번째 백분위라 0%가 되는데,
 * 「동종 상위 0%」는 아무도 하지 않는 말이다(BE `ReadPeerPct`와 같은 규칙).
 */
export function topPercentLabel(percentile: number | undefined): string | null {
  if (typeof percentile !== "number") return null;
  return `상위 ${Math.max(1, Math.min(100, Math.round(100 - percentile)))}%`;
}

/** 지표 원값을 단위에 맞춰 적는다. */
export function formatMetricValue(value: number | undefined, unit: string): string {
  if (typeof value !== "number") return "—";
  switch (unit) {
    case "%":
      return `${value.toFixed(2)}%`;
    case "x":
      return `${value.toFixed(2)}배`;
    case "price":
      return value.toLocaleString("ko-KR", { maximumFractionDigits: 2 });
    case "score":
      return value.toFixed(1);
    default:
      return value.toLocaleString("ko-KR", { maximumFractionDigits: 2 });
  }
}

/** +2 / −2 / 0 — 부호를 항상 붙인다. 0은 부호 없이 「0」이다. */
export function formatDelta(delta: number, digits = 0): string {
  const v = Number(delta.toFixed(digits));
  if (v === 0) return "0";
  return `${v > 0 ? "+" : "−"}${Math.abs(v).toFixed(digits)}`;
}

/**
 * 축이 빈 이유를 사용자 문장으로.
 *
 * 「아직 없다」와 「원래 없다」는 다른 답이다 — ETF에는 재무제표가 없어서 수익성
 * 축이 채워질 일이 없는데, 둘을 같은 말로 적으면 언젠가 생길 것처럼 읽힌다.
 */
export function missingReasonLabel(reason: string | undefined): string {
  switch (reason) {
    case "not_applicable":
      return "해당 없음";
    case "weight_zero":
      return "가중치 0";
    case "no_data":
      return "데이터 없음";
    default:
      return "데이터 없음";
  }
}

/** 비교할 것이 없는 창의 사유 문장. */
export function changeUnavailableLabel(reason: string | undefined): string {
  switch (reason) {
    case "score_withheld":
      return "한쪽 회차의 점수가 보류되어 비교할 수 없습니다.";
    case "no_prior_row":
      return "비교할 이전 회차가 없습니다.";
    default:
      return "비교할 수 없습니다.";
  }
}

/**
 * 목록 한 줄에 들어갈 사유 — **한 조각만**.
 *
 * 드라이버의 `detail`은 「추세 19.2 → 26.9」처럼 숫자를 그대로 적은 문장이라
 * 설명 패널에서는 맞지만 목록 행에서는 폭을 다 먹는다. 여기서는 축 이름이나
 * 「가치 축 결측」처럼 **무엇 때문인지**만 남긴다.
 */
export function shortDriverLabel(driver: ScoreChangeDriver | undefined): string | null {
  if (!driver) return null;
  switch (driver.kind) {
    case "axis":
      return `${driver.label} 변화`;
    case "availability":
      // BE가 「가치·심리·수급 축이 빠졌습니다」로 준다 — 목록에서는 「축 결측」까지만.
      return driver.detail.includes("빠졌")
        ? `${axesFromDetail(driver.detail)} 결측`
        : "평가 축 변화";
    case "shrink":
      return "데이터 충실도 변화";
    case "weight":
      return `${driver.label} 조정`;
    default:
      return driver.label;
  }
}

/** 「가치·심리·수급 축이 빠졌습니다」 → 「가치·심리·수급 축」 */
function axesFromDetail(detail: string): string {
  const cut = detail.indexOf(" 축이");
  return cut > 0 ? `${detail.slice(0, cut)} 축` : "평가 축";
}

/**
 * 피어그룹 키를 화면 문장으로. `domestic:KOSPI:cap3` → 「국내 KOSPI 대형주」.
 *
 * 저장된 값은 배치가 쓰는 내부 키다(`service/stock_score.go assignPeerGroups`).
 * 「어느 집단에서 잰 순위인가」는 이 표에서 가장 자주 읽히는 칸인데, 그 자리에
 * 콜론으로 이어 붙인 영문 키가 서 있으면 읽는 사람이 거기서 멈춘다.
 *
 * 모르는 조각은 **그대로 남긴다** — 배치가 새 층을 만들면 여기서 조용히 사라지는
 * 것보다 낯선 채로 보이는 편이 낫다.
 */
/** 점수 배치의 섹터 키(`model.Sector`) — 그룹 이름을 읽을 때만 쓴다. */
const SECTOR_PART_LABELS: Record<string, string> = {
  energy: "에너지",
  materials: "소재",
  industrials: "산업재",
  consumer_discretionary: "경기소비재",
  consumer_staples: "필수소비재",
  health_care: "헬스케어",
  financials: "금융",
  information_technology: "IT",
  communication: "커뮤니케이션",
  utilities: "유틸리티",
  real_estate: "부동산",
};

/** 공통 11섹터 키 → 화면 이름(스크리너 업종 칩·결과 표도 이것을 쓴다). */
export function sectorLabel(sector: string | null | undefined): string {
  return (sector && SECTOR_PART_LABELS[sector]) ?? sector ?? "";
}

export function peerGroupLabel(peerGroup: string | undefined): string | null {
  if (!peerGroup) return null;
  if (peerGroup === "*") return "전체 시장";
  return peerGroup
    .split(":")
    .map((part) => {
      switch (part) {
        case "domestic":
          return "국내";
        case "us":
          return "미국";
        case "etf":
          return "ETF";
        case "etn":
          return "ETN";
        case "cap1":
          return "소형주";
        case "cap2":
          return "중형주";
        case "cap3":
          return "대형주";
        default:
          // 재무 지표의 섹터 그룹(`domestic:information_technology`, 2026-09-23).
          return SECTOR_PART_LABELS[part] ?? part;
      }
    })
    .join(" ");
}

/**
 * 레일 카드처럼 **아주 좁은 칸**을 위한 사유 — 두세 글자.
 *
 * `shortDriverLabel`은 점수 카드의 한 줄(폭 300px+)을 위한 것이라 「가치·심리·수급
 * 축 결측」처럼 길다. 레일 카드의 사유 칸은 88px이고, 거기서 그 문장은 「가치·심리·
 * 수급 축…」으로 잘려 **정작 중요한 「결측」이 사라진다.**
 *
 * 그래서 이 자리에서는 **무엇이 아니라 어떤 종류인지**만 말한다. 어느 축이었는지는
 * `title`에 남고, 종목을 누르면 열리는 화면이 전부 말한다. 「축이 빠져서 오른 것」과
 * 「추세가 좋아져서 오른 것」의 구분이 이 칸이 지켜야 할 최소한이다.
 */
export function compactDriverLabel(driver: ScoreChangeDriver | undefined): string | null {
  if (!driver) return null;
  switch (driver.kind) {
    case "axis":
      return driver.label;
    case "availability":
      return driver.detail.includes("빠졌") ? "축 결측" : "축 추가";
    case "shrink":
      return "충실도";
    case "weight":
      return "가중치";
    default:
      return driver.label;
  }
}

/**
 * 설명 패널 맨 위의 **한 문장** (2026-09-24 사용자 결정 — 템플릿, LLM 0).
 *
 * 숫자는 전부 응답에 있는 값이다 — 이 함수는 고르고 이어 붙이기만 한다. 두 마디다:
 *  1. 무엇이 점수를 만들었나 — 기여가 가장 큰 축과 가장 낮은 축.
 *  2. 어제와 왜 다른가 — 전일 변화의 상위 원인 둘. 원인이 데이터(축 결측·충실도)면 그렇다고 말한다.
 */
/**
 * 주격 조사 — 받침이 있으면 「이」, 없으면 「가」. 괄호 속 숫자가 아니라 **이름**에 붙인다
 * (「추세(99점)가」). 한글이 아니면(영문 약어) 「가」로 둔다.
 */
export function subjectParticle(word: string): "이" | "가" {
  const ch = word.trim().at(-1);
  if (!ch) return "가";
  const code = ch.charCodeAt(0) - 0xac00;
  if (code < 0 || code > 11171) return "가";
  return code % 28 === 0 ? "가" : "이";
}

export function explainSummarySentence(data: ScoreExplainData): string | null {
  if (data.score == null) return null;
  const present = data.axes.filter((a) => a.available && a.score != null);
  if (present.length === 0) return null;
  const top = [...present].sort((a, b) => (b.contribution ?? 0) - (a.contribution ?? 0))[0]!;
  const low = [...present].sort((a, b) => (a.score ?? 0) - (b.score ?? 0))[0]!;

  const first =
    low.axis === top.axis
      ? `종합 ${data.score}점 — ${top.label}(${Math.round(top.score ?? 0)}점)${subjectParticle(top.label)} 점수를 만들었습니다.`
      : `종합 ${data.score}점 — ${top.label}(${Math.round(top.score ?? 0)}점)${subjectParticle(top.label)} 가장 크게 받쳤고, ${low.label}(${Math.round(low.score ?? 0)}점)${subjectParticle(low.label)} 가장 낮습니다.`;

  const day = data.changes.find((c) => c.window === "1d");
  if (!day || !day.available || !day.delta) return first;
  const drivers = [...day.drivers]
    .filter((d) => Math.abs(d.effect) >= 0.05)
    .sort((a, b) => Math.abs(b.effect) - Math.abs(a.effect))
    .slice(0, 2);
  if (drivers.length === 0) return first;
  const parts = drivers.map(
    (d) => `${shortDriverLabel(d)} ${d.effect > 0 ? "+" : ""}${d.effect.toFixed(1)}`,
  );
  const dataDriven = drivers[0]!.kind === "availability" || drivers[0]!.kind === "shrink";
  const sign = day.delta > 0 ? "+" : "";
  return `${first} 어제보다 ${sign}${day.delta}점(${parts.join(" · ")})${dataDriven ? " — 회사가 아니라 데이터가 바뀐 몫이 큽니다" : ""}.`;
}
