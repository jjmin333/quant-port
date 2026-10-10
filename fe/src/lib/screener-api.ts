import { apiRequest } from "@/lib/api-client";

/**
 * 종목 스크리너 — `be/internal/model/screener.go`의 거울 (V2 요구 4).
 *
 * # 이 화면이 다른 스크리너와 갈라지는 자리
 *
 * 보통의 스크리너는 결과 수만 말한다. 그래서 0이 나오면 사용자는 **시장을 탓한다.**
 * 여기서는 `ScreenFunnel`이 어디서 몇이 빠졌는지를 말한다 — 특히 「잴 수 있음」과
 * 「조건 통과」 사이가 진짜 탈락이고, 그 앞은 **재보지 못한 것**이다 (요구 8).
 *
 * # 단위는 화면 쪽이 기준이다
 *
 * 시가총액은 **조(兆)** 로 주고받는다. 서버가 실행 지점에서만 원 단위로 환산하므로,
 * 저장된 조건도 사람이 읽을 수 있는 숫자로 남는다.
 */

export type ScreenFieldGroup = "score" | "financial" | "valuation" | "price" | "signal";

export const SCREEN_GROUP_LABEL: Record<ScreenFieldGroup, string> = {
  score: "AI 점수",
  financial: "재무",
  valuation: "가치",
  price: "가격·기술",
  signal: "일정·신호",
};

export const SCREEN_GROUP_ORDER: ScreenFieldGroup[] = [
  "score",
  "financial",
  "valuation",
  "price",
  "signal",
];

export type ScreenField = {
  id: string;
  label: string;
  group: ScreenFieldGroup;
  /** "%" | "배" | "점" | "원" | "조원" | "" */
  unit: string;
  /** 원값의 방향. 부채비율·PER은 false라 기본 연산자가 「이하」가 됩니다. */
  higherIsBetter: boolean;
  decimals: number;
  /** ⓘ 설명 — 뜻과 좋은 방향(2026-09-28). */
  help?: string;
  /** 글 태그 `@종목:지표`에서 부르는 이름들. */
  aliases?: string[];
};

export type ScreenCatalogField = ScreenField & {
  /** 이 시장에서 이 값이 측정된 종목 수. */
  count: number;
  /** 분모 — 랭킹 자격을 갖춘 종목 수. */
  inMarket: number;
  /** 자격 종목의 25·50·75% 값 — 「보통 p25~p75」 가이드. 표본이 적으면 없다. */
  quartiles?: [number, number, number];
};

/** 한 종목의 스크리너 값 전부 — 「기준 종목」과 태그 편집기. */
export type ScreenSymbolValues = {
  symbol: string;
  name: string;
  assetType: string;
  market: "domestic" | "overseas";
  /** 공통 11섹터 키 — 없으면 빈 값. */
  sector?: string;
  tradeDate: string;
  values: Record<string, number>;
};

export async function fetchScreenSymbolValuesClient(symbol: string): Promise<ScreenSymbolValues | null> {
  return apiRequest<ScreenSymbolValues | null>(
    `/api/v1/stocks/${encodeURIComponent(symbol)}/screen-values`,
    undefined,
    "이 종목의 지표를 불러오지 못했습니다",
  );
}

export type ScreenCatalog = {
  tradeDate: string;
  market: string;
  inMarket: number;
  fields: ScreenCatalogField[];
};

export type ScreenRule = {
  field: string;
  min?: number;
  max?: number;
};

export type ScreenFunnel = {
  total: number;
  inMarket: number;
  /** 랭킹과 같은 자격 — 축 3개 이상에서 점수가 계산된 종목. */
  eligible: number;
  /** 건 조건을 **전부 잴 수 있는** 종목. */
  measurable: number;
  passed: number;
};

export type ScreenCell = { field: string; value: number | null };

export type ScreenRow = {
  symbol: string;
  name?: string;
  assetType?: string;
  score?: number;
  grade?: string;
  close?: number;
  changeRate?: number;
  currency?: string;
  /**
   * 아래 둘은 **AI 추천 랭킹 행과 같은 뜻·같은 이름**입니다 (2026-09-23 QA P0-01).
   *
   * 예전에는 이 표에 그 둘이 없어 실시간 시세를 얹을 수도, 얹었다고 말할 수도
   * 없었습니다. 그래서 장이 열리면 같은 종목이 랭킹에서는 흐르는 값으로, 여기서는
   * 어제 종가로 보였습니다.
   */
  /** `close`가 어느 날 것인가(YYYY-MM-DD). 실시간으로 덮였으면 `"live"`. */
  priceAsOf?: string;
  /** 등락률을 다시 낼 때 쓰는 기준가. 없으면 등락률을 **그리지 않습니다.** */
  basePrice?: number;
  cells?: ScreenCell[];
};

export type ScreenResult = {
  tradeDate: string;
  market: string;
  funnel: ScreenFunnel;
  rows: ScreenRow[];
  truncated: boolean;
  coverage?: { field: string; count: number }[];
};

export type UserScreen = {
  id: number;
  name: string;
  market: string;
  rules: ScreenRule[];
  sortBy?: string;
  sortAsc?: boolean;
  /** 지금은 쓸 수 없어 빠진 조건 수 — 지표가 사라지면 생깁니다. */
  droppedRules?: number;
  updatedAt?: string;
};

export type ScreenQuery = {
  market: string;
  /** 공통 11섹터 키 — 문장 검색이 「반도체」 등을 옮긴 것(2026-09-27). */
  sector?: string;
  rules: ScreenRule[];
  sortBy?: string;
  sortAsc?: boolean;
  limit?: number;
  /** 기준 종목 — 있으면 서버가 「닮은 순」(같은 업종 먼저)으로 정렬한다(2026-10-01 결정 B). */
  like?: string;
};

/* ------------------------------------------------------------------------- */
/* 첫 화면의 조건                                                              */
/* ------------------------------------------------------------------------- */

/**
 * 아무것도 고르지 않은 사람이 처음 보는 조건 — 「국내 · 종합 점수 70 이상」.
 *
 * **여기 한 곳에만 둡니다.** 예전에는 `screener-client.tsx`의 `useState` 초기값에
 * 같은 리터럴이 두 번 적혀 있었습니다(`rules`와 `ranQuery`). 서버가 같은 조건을
 * 미리 조회해 심게 되면서 그 리터럴이 **세 곳**이 되는데, 하나가 어긋나면 서버가
 * 심은 결과와 화면의 조건 칸이 다른 말을 하고 그 차이는 화면에 안 보입니다 —
 * 사용자는 「score ≥ 70」이라고 적힌 칸 옆에서 다른 조건의 결과를 봅니다.
 *
 * 함수인 이유는 **배열을 돌려주기 때문**입니다. 상수 배열을 내보내면 `setRules`가
 * 그 배열을 그대로 상태에 넣고, 다음 사용자가 앞 사람이 고친 기본값을 받습니다.
 */
export const SCREENER_DEFAULT_MARKET = "domestic";
export const SCREENER_DEFAULT_LIMIT = 100;

export function defaultScreenRules(): ScreenRule[] {
  return [{ field: "score", min: 70 }];
}

/** 첫 조회 — 기본 조건 + 기본 보기 탭(개요)의 열. 서버 선반입과 화면이 같은 질의여야 심은 결과를 그대로 쓴다. */
export function defaultScreenQuery(): ScreenQuery {
  return {
    market: SCREENER_DEFAULT_MARKET,
    rules: rulesWithView(defaultScreenRules(), SCREEN_DEFAULT_VIEW),
    limit: SCREENER_DEFAULT_LIMIT,
  };
}

export const SCREEN_DEFAULT_VIEW = "overview";

/* ------------------------------------------------------------------------- */
/* 요청                                                                       */
/* ------------------------------------------------------------------------- */

export async function fetchScreenCatalogClient(market: string): Promise<ScreenCatalog | null> {
  return apiRequest<ScreenCatalog | null>(
    `/api/v1/stocks/screen/fields?market=${encodeURIComponent(market)}`,
    undefined,
    "조건 목록을 불러오지 못했습니다",
  );
}

/**
 * 조회가 POST인 이유는 서버 쪽 주석과 같습니다 — 조건 12개를 URL에 담으면 프록시마다
 * 다른 상한에 걸리고, 사용자마다 다른 조건이라 URL을 키로 잡는 캐시는 적중률이
 * 0에 가깝습니다.
 */
export async function runScreenClient(query: ScreenQuery): Promise<ScreenResult | null> {
  return apiRequest<ScreenResult | null>(
    "/api/v1/stocks/screen",
    { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(query) },
    "종목을 찾지 못했습니다",
  );
}

export async function fetchUserScreensClient(): Promise<UserScreen[]> {
  const data = await apiRequest<UserScreen[] | null>(
    "/api/v1/user/screens",
    undefined,
    "저장한 조건을 불러오지 못했습니다",
  );
  return data ?? [];
}

export async function saveUserScreenClient(input: {
  name: string;
  market: string;
  rules: ScreenRule[];
}): Promise<UserScreen | null> {
  return apiRequest<UserScreen | null>(
    "/api/v1/user/screens",
    { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(input) },
    "조건을 저장하지 못했습니다",
  );
}

export async function deleteUserScreenClient(id: number): Promise<void> {
  await apiRequest<unknown>(
    `/api/v1/user/screens/${id}`,
    { method: "DELETE" },
    "조건을 지우지 못했습니다",
  );
}

/* ------------------------------------------------------------------------- */
/* 표시 규칙                                                                  */
/* ------------------------------------------------------------------------- */

/**
 * 커버리지 색 — **초록/주황/빨강**입니다.
 *
 * 이 앱에서 빨강은 상승, 파랑은 하락이라 손익의 두 색을 쓰면 같은 화면에서 색 하나가
 * 두 가지를 말하게 됩니다. 점수 등급과 같은 계열을 씁니다.
 *
 * 경계는 실측에 맞췄습니다(2026-09-17 국내 자격 2,653종목 기준): 대부분의 조건이
 * 90%를 넘고, 낮은 쪽은 PER 60% · 심리·수급 12%입니다.
 */
export function coverageTone(count: number, total: number): string {
  if (total <= 0) return "text-[var(--text-muted)]";
  const ratio = count / total;
  if (ratio >= 0.85) return "text-[var(--score-high)]";
  if (ratio >= 0.4) return "text-[var(--score-mid)]";
  return "text-[var(--score-low)]";
}

/** 90% 아래는 화면에 적습니다 — 그 위는 「거의 전부」라 적을 값이 없습니다. */
export function coverageLabel(count: number, total: number): string | null {
  if (total <= 0) return null;
  const pct = (count / total) * 100;
  if (pct >= 90) return null;
  return `${pct < 1 ? pct.toFixed(1) : Math.round(pct)}%만 측정`;
}

/** 값과 단위를 사람이 읽는 한 덩어리로. 값이 없으면 「-」입니다. */
export function formatFieldValue(value: number | null | undefined, field?: ScreenField): string {
  if (value == null || !Number.isFinite(value)) return "-";
  const decimals = field?.decimals ?? 0;
  const unit = field?.unit ?? "";
  const body = value.toLocaleString("ko-KR", {
    minimumFractionDigits: decimals,
    maximumFractionDigits: decimals,
  });
  if (unit === "원") return `${body}원`;
  if (unit === "조원") return `${body}조`;
  if (unit === "%") return `${body}%`;
  if (unit === "배") return `${body}배`;
  if (unit === "점") return body;
  if (unit === "회") return `${body}회`;
  return body;
}

/** 조건 한 줄을 「15% 이상」·「10~20배」 같은 한 덩어리로. */
export function describeRule(rule: ScreenRule, field?: ScreenField): string {
  const unit = field?.unit === "조원" ? "조" : field?.unit ?? "";
  const num = (v: number) => `${v.toLocaleString("ko-KR")}${unit}`;
  if (rule.min != null && rule.max != null) return `${num(rule.min)} ~ ${num(rule.max)}`;
  if (rule.min != null) return `${num(rule.min)} 이상`;
  if (rule.max != null) return `${num(rule.max)} 이하`;
  return "조건 없음";
}

/** 등락률. 0은 `−0.00%`가 되지 않습니다. */
export function formatChangeRate(rate: number | null | undefined): string {
  if (rate == null || !Number.isFinite(rate)) return "-";
  if (rate === 0) return "0.00%";
  return `${rate > 0 ? "+" : "−"}${Math.abs(rate).toFixed(2)}%`;
}

export function formatCount(n: number): string {
  return n.toLocaleString("ko-KR");
}

/**
 * 깔때기를 화면이 그릴 단계 목록으로.
 *
 * `drop`이 true인 칸이 **재보지 못해서** 빠진 자리입니다 — 조건을 틀려서 빠진 것과
 * 색이 달라야 합니다. `compact`는 모바일용으로 앞 두 칸을 접습니다.
 */
export type FunnelStep = { key: string; label: string; value: number; drop?: boolean; final?: boolean };

export function funnelSteps(funnel: ScreenFunnel, market: string, compact = false): FunnelStep[] {
  const marketLabel = market === "overseas" ? "해외" : "국내";
  const all: FunnelStep[] = [
    { key: "total", label: "전체", value: funnel.total },
    { key: "inMarket", label: marketLabel, value: funnel.inMarket },
    { key: "eligible", label: "점수 자격", value: funnel.eligible, drop: true },
    { key: "measurable", label: "잴 수 있음", value: funnel.measurable, drop: true },
    { key: "passed", label: "조건 통과", value: funnel.passed, final: true },
  ];
  return compact ? all.slice(2) : all;
}

/* ------------------------------------------------------------------------- */
/* 기준 종목 (2026-09-28 사용자 요청 5)                                        */
/* ------------------------------------------------------------------------- */

/** 「이 종목 이상으로 채우기」가 거는 묶음 — 점수 6개 + PER·ROE·부채비율 (사용자 결정). */
export const REFERENCE_FILL_FIELDS = [
  "score",
  "axis.technical",
  "axis.quality",
  "axis.growth",
  "axis.valuation",
  "axis.sentiment",
  "per",
  "roe",
  "debtRatio",
] as const;

/**
 * 기준 종목의 값으로 조건을 만든다 — **좋은 방향으로만**(점수·ROE는 이상, PER·부채비율은 이하).
 * 경계는 표시 자릿수에서 기준 종목이 **통과하는 쪽**으로 반올림한다: 이상이면 내림, 이하면 올림.
 * 값이 없는 지표(적자 PER 등)는 건너뛴다 — 걸면 기준 종목부터 탈락한다.
 */
export function referenceRules(
  values: Record<string, number>,
  fields: Map<string, Pick<ScreenField, "higherIsBetter" | "decimals">>,
  ids: readonly string[] = REFERENCE_FILL_FIELDS,
): ScreenRule[] {
  const out: ScreenRule[] = [];
  for (const id of ids) {
    const v = values[id];
    const f = fields.get(id);
    if (v == null || !Number.isFinite(v) || !f) continue;
    /*
      「닮은꼴 · 나은 종목」(2026-10-01 결정 B): 9개를 전부 기준 이상으로 걸면 MU 2 · NVDA 1종목뿐이었다.
      종합 점수만 기준 이상, 축 다섯은 REFERENCE_AXIS_SLACK만큼 모자라도 통과, 재무 셋은 **열로만** —
      순서는 서버가 「닮은 순」(`like`)으로 정한다.
    */
    if (id.startsWith("axis.")) {
      out.push({ field: id, min: Math.max(0, Math.floor(v) - REFERENCE_AXIS_SLACK) });
      continue;
    }
    if (id !== "score") {
      out.push({ field: id });
      continue;
    }
    const scale = 10 ** (f.decimals ?? 0);
    if (f.higherIsBetter) out.push({ field: id, min: Math.floor(v * scale) / scale });
    else out.push({ field: id, max: Math.ceil(v * scale) / scale });
  }
  return out;
}

/** 축 허용폭 — 실측 −15에서 MU 28 · 삼성전자 112 · NVDA 40종목(−10은 16 · 59 · 20). */
export const REFERENCE_AXIS_SLACK = 15;

/**
 * 결과 행이 기준 종목보다 **어디서 나은가/모자란가** — 축 다섯만, 5점 이상 차이만, 큰 것부터 `max`개.
 * 「닮은꼴」 결과의 칩(파랑 +, 회색 −)이 읽는다.
 */
export function likeDiffs(
  cells: ScreenCell[] | undefined,
  ref: Record<string, number>,
  fields: Map<string, Pick<ScreenField, "label">>,
  max = 3,
): { field: string; label: string; diff: number }[] {
  const out: { field: string; label: string; diff: number }[] = [];
  for (const c of cells ?? []) {
    const r = ref[c.field];
    if (!c.field.startsWith("axis.") || c.value == null || r == null) continue;
    const diff = Math.round(c.value - r);
    if (Math.abs(diff) >= 5) out.push({ field: c.field, label: fields.get(c.field)?.label ?? c.field, diff });
  }
  return out.sort((a, b) => Math.abs(b.diff) - Math.abs(a.diff)).slice(0, max);
}

/** 조건 칸 옆 가이드 — 「보통 40~61」. 분포가 없으면 null. */
export function quartileHint(field: ScreenCatalogField): string | null {
  if (!field.quartiles) return null;
  const [lo, , hi] = field.quartiles;
  const d = Math.min(field.decimals, 1);
  // 1 미만은 유효숫자 둘 — 해외 시가총액(조 단위 0.0003~0.005)이 「0~0」으로 뭉개지지 않게.
  const fmt = (v: number) =>
    v !== 0 && Math.abs(v) < 1
      ? v.toLocaleString("ko-KR", { maximumSignificantDigits: 2 })
      : v.toLocaleString("ko-KR", { maximumFractionDigits: d, minimumFractionDigits: 0 });
  return `보통 ${fmt(lo)}~${fmt(hi)}`;
}

/* ------------------------------------------------------------------------- */
/* 프리셋 · 보기 탭 (2026-10-09 결정 A — docs/mockups/requests-1009-v1.html ②)   */
/* ------------------------------------------------------------------------- */

/** 목적 카드 하나 — 서버가 지금 걸리는 종목 수(count)를 같이 준다. -1은 셀 수 없었음. */
export type ScreenPreset = {
  id: string;
  label: string;
  icon: string;
  desc: string;
  rules: ScreenRule[];
  count: number;
  /** 앞 점수일의 통과 수 — 없으면 변화 표시를 안 한다(2026-10-10). */
  prevCount?: number;
  /** 앞 점수일엔 없다가 새로 걸린 종목, 점수 높은 순 최대 3. */
  newNames?: string[];
};

export async function fetchScreenPresetsClient(market: string): Promise<ScreenPreset[]> {
  return (
    (await apiRequest<ScreenPreset[] | null>(
      `/api/v1/stocks/screen/presets?market=${encodeURIComponent(market)}`,
      undefined,
      "자주 쓰는 조건을 불러오지 못했습니다",
    )) ?? []
  );
}

/**
 * 결과표 보기 탭 — 탭마다 덧붙여 보는 열(거르지 않는 「열만」 조건으로 보낸다). 거는 조건의 열은 늘 먼저 선다.
 * 한 탭 4칸 이하 — 칸이 많을수록 아무 칸도 안 읽힌다(Finviz의 단점).
 */
export const SCREEN_VIEWS = [
  // 개요 = 현재 주가 · 등락률 · AI 점수(고정 열) 다음 시가총액 · 20일 · 60일(2026-10-09 요청 3).
  { id: "overview", label: "개요", fields: ["marketCap", "return20d", "return60d"] },
  { id: "value", label: "가치", fields: ["per", "pbr", "psr", "targetUpside"] },
  { id: "quality", label: "수익성", fields: ["roe", "operatingMargin", "netProfitMargin", "debtRatio"] },
  { id: "growth", label: "성장", fields: ["revenueGrowthYoY", "epsGrowthYoY", "return5d", "return20d"] },
  { id: "signal", label: "수급·신호", fields: ["flowNetPct", "rsi14", "pctFromHigh52Week", "daysToEarnings"] },
] as const;

export type ScreenViewId = (typeof SCREEN_VIEWS)[number]["id"];

/** 거는 조건 + 보기 탭의 열(값 없는 줄). 이미 있는 필드는 다시 넣지 않고, 서버 상한(12)에서 자른다. */
export function rulesWithView(rules: ScreenRule[], view: ScreenViewId, allowed?: Set<string>): ScreenRule[] {
  const out = rules.map((r) => ({ ...r }));
  const have = new Set(out.map((r) => r.field));
  const extra = SCREEN_VIEWS.find((v) => v.id === view)?.fields ?? [];
  for (const f of extra) {
    if (out.length >= 12) break;
    if (have.has(f) || (allowed && !allowed.has(f))) continue;
    out.push({ field: f });
    have.add(f);
  }
  return out;
}

/** 조건 칩 한 개의 글 — 「PER ≤ 12배」, 값이 없으면 「PER (열만)」. */
export function ruleChipLabel(rule: ScreenRule, field?: ScreenField): string {
  const name = field?.label ?? rule.field;
  if (rule.min == null && rule.max == null) return `${name} (열만)`;
  return `${name} ${describeRule(rule, field)}`;
}

/**
 * 범위 고르기 빠른 칩 — 분포(p25·p50·p75)에서 바로 누를 값 셋. 좋은 방향 쪽으로 엄격해진다.
 * ROE(높을수록)는 「≥ p25 · ≥ p50 · ≥ p75」, PER(낮을수록)은 「≤ p75 · ≤ p50 · ≤ p25」.
 */
export function quickBounds(field: ScreenCatalogField): { op: "min" | "max"; value: number }[] {
  if (!field.quartiles) return [];
  const d = Math.min(field.decimals, 1);
  const round = (v: number) => Number(v.toFixed(d));
  const [p25, p50, p75] = field.quartiles.map(round);
  const vals = field.higherIsBetter ? [p25, p50, p75] : [p75, p50, p25];
  const op = field.higherIsBetter ? "min" : "max";
  return [...new Set(vals)].map((value) => ({ op, value }));
}
