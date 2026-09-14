import type { PriceTarget } from "@/lib/price-targets";
import type { QuoteSummary } from "@/lib/quotes-types";
import type { ScoreBreakdown, Stamp, StampHorizonKind, StampVerdict } from "@/lib/stamp-types";

/**
 * 「목표가 예측」 — 목표가 관리와 스탬프를 하나의 레코드로 본 모양.
 *
 * # 왜 합치는가
 *
 * 두 기능은 같은 물건의 두 반쪽이었다. 목표가는 「얼마까지」만 있고 **언제까지**가
 * 없어 채점이 불가능했고, 스탬프는 「언제까지 얼마」는 있지만 **손절가·2차 목표**가
 * 없어 매매 계획이 되지 못했다. 실제로 두 테이블은 이미 절반쯤 같은 코드였다 —
 * 도달 판정(일봉 고·저 훑기), 진행률, peak/trough가 두 벌로 구현돼 있다.
 *
 * # 이 파일이 하는 일
 *
 * **BE는 아직 두 테이블 그대로다.** 여기서 두 응답을 하나의 `Prediction`으로 접어
 * 화면이 한 종류만 다루게 한다. 기한 있는 예측은 스탬프에서, 기한 없는 예측
 * (`horizonKind: "none"`)은 목표가에서 온다 — 나중에 BE를 합칠 때
 * `fromStamp`/`fromTarget` 두 함수만 지우면 나머지 화면 코드는 그대로 산다.
 *
 * 두 출처가 채우지 못하는 칸은 **`undefined`로 남긴다**. 목표가에는 등록 당시
 * 스냅샷이 없으므로 5축 대조·알파·적중 판정이 없고, 화면은 그 자리에 "—"를 그린다.
 * 0으로 채우면 「알파 0%」가 되어 없는 사실을 지어내게 된다.
 */

/** 기한 종류. `none`이 기존 목표가(무기한)가 들어오는 자리다. */
export type PredictionHorizon = StampHorizonKind | "none";

export type PredictionSource = "stamp" | "target";

/**
 * 상태 일곱 가지.
 *
 * **「도달」과 「적중」은 다른 사건이다.** 도달은 가격이 목표를 찍은 것(장중 고·저),
 * 적중은 기한이 끝났을 때 방향이 맞은 것이다. 무기한 예측은 채점 시점이 없으므로
 * 도달까지만 있고 적중은 영원히 없다 — 그래서 색을 아예 갈라 놓는다.
 */
export type PredictionStatus =
  | "live" // 진행 중
  | "near" // 목표·손절 5% 이내
  | "reached" // 목표 도달 (가격 사건)
  | "stop_hit" // 손절선 통과
  | "due" // 기한이 지났고 아직 안 열어 봄
  | "hit" // 채점 결과 적중
  | "miss"; // 채점 결과 빗나감

export const PREDICTION_STATUS_LABEL: Record<PredictionStatus, string> = {
  live: "진행 중",
  near: "임박",
  reached: "목표 도달",
  stop_hit: "손절 통과",
  due: "개봉 대기",
  hit: "적중",
  miss: "빗나감",
};

/** 목표·손절까지 이 % 이내면 「임박」으로 본다. 목표가 화면의 기존 기준과 같은 값. */
export const NEAR_PCT = 5;

/** 등록 후 종합점수가 이만큼 움직이면 「전제가 흔들렸다」고 본다. */
export const SCORE_SHIFT_ALERT = 5;

export type Prediction = {
  /** 두 출처가 각자 1번 id를 갖고 있어 목록 key로는 쓸 수 없다. */
  key: string;
  source: PredictionSource;
  id: number;
  symbol: string;
  symbolName: string;
  currency: "KRW" | "USD";

  /** 등록 시각(스탬프는 stampedAt, 목표가는 createdAt). */
  createdAt: string;
  /** 등록 시점 주가 — 서버가 잰 값이라 목표를 고쳐도 그대로다. */
  basePrice?: number;
  baseDate?: string;
  currentPrice?: number;

  targetPrice: number;
  targetPrice2?: number;
  stopPrice?: number;
  /** 목표가 등록가보다 위인가. 등록가가 없으면 현재가로, 둘 다 없으면 상방으로 본다. */
  upside: boolean;

  horizonKind: PredictionHorizon;
  /** 기한이 있는 예측만 갖는다. 서버가 시장 캘린더로 확정한 값. */
  dueAt?: string;
  conviction?: number;
  note: string;

  // ── 파생 (조회 시 계산, 저장하지 않음) ──
  /** 목표가가 함의하는 등락률 (%). */
  expectedReturn?: number;
  /** 등록 후 실제 등락률 (%). */
  sinceBasePct?: number;
  /** 현재가에서 목표까지 남은 % (부호 있음). */
  toTargetPct?: number;
  toTarget2Pct?: number;
  toStopPct?: number;
  /**
   * 등록가 → 목표 구간의 진척률(%). 100이면 도달, **음수면 반대로 갔다**.
   * 스탬프의 `targetProgress`와 목표가의 `progress`를 한 척도로 맞춘 값이다.
   */
  progress?: number;
  /** 손익비 = 목표까지 거리 ÷ 손절까지 거리. 손절이 없으면 없다. */
  riskReward?: number;
  /** 목표에 처음 닿은 날. 종가가 아니라 일봉 고·저 기준. */
  reachedDate?: string;
  /** 등록 이후 최고·최저 등락률 (%). */
  peakReturn?: number;
  troughReturn?: number;
  status: PredictionStatus;
  /** 만기까지 남은 일수. 음수면 지났다. 기한 없으면 없다. */
  daysLeft?: number;

  // ── 채점 (기한 있는 예측에만 있다) ──
  verdict?: StampVerdict;
  due?: boolean;
  opened?: boolean;
  /** 만기 구간 수익률 (%). */
  horizonReturn?: number;
  benchmarkId?: string;
  alpha?: number;
  /** |만기가 − 목표가| ÷ 목표가 (%). */
  targetAccuracy?: number;

  // ── 등록 당시 스냅샷 (스탬프 출처만) ──
  scoreThen?: number;
  scoreNow?: number;
  gradeThen?: string;
  gradeNow?: string;
  /** 지금 점수 − 당시 점수. 「전제가 흔들렸다」의 근거. */
  scoreShift?: number;
  breakdownThen?: ScoreBreakdown;
  breakdownNow?: ScoreBreakdown;
  held?: boolean;
  holdingAvgPrice?: number;
  watchlisted?: boolean;

  /** 상세 모달이 원본 레코드를 그대로 쓴다. */
  stamp?: Stamp;
  target?: PriceTarget;
};

function pct(from: number, to: number): number {
  return ((to - from) / from) * 100;
}

function positive(value: number | undefined | null): number | undefined {
  return typeof value === "number" && value > 0 ? value : undefined;
}

/** 만기까지 남은 일수. 0이면 오늘, 음수면 이미 지났다. */
export function daysUntil(iso: string): number {
  const due = new Date(iso);
  if (Number.isNaN(due.getTime())) return 0;
  const startOfDay = (d: Date) => new Date(d.getFullYear(), d.getMonth(), d.getDate()).getTime();
  return Math.round((startOfDay(due) - startOfDay(new Date())) / 86_400_000);
}

/**
 * 상태 판정.
 *
 * 순서가 규칙이다 — **채점이 끝났으면 그것이 최종**이고, 아직 세는 중이면 가격
 * 사건(도달·손절)이 거리(임박)보다 먼저다. 거리를 먼저 보면 이미 목표를 넘어선
 * 예측이 「임박」으로 남는다.
 */
function deriveStatus(input: {
  source: PredictionSource;
  verdict?: StampVerdict;
  due?: boolean;
  opened?: boolean;
  reachedDate?: string;
  upside: boolean;
  current?: number;
  targetPrice: number;
  stopPrice?: number;
  toTargetPct?: number;
  toStopPct?: number;
}): PredictionStatus {
  const { verdict, due, opened, reachedDate, upside, current, targetPrice, stopPrice } = input;

  // 아직 안 열어 본 만기는 판정보다 먼저다 — 결과를 먼저 보여주면 개봉이 사건이 아니게 된다.
  if (due && !opened) return "due";
  if (verdict === "hit") return "hit";
  if (verdict === "miss") return "miss";

  if (reachedDate) return "reached";
  if (current !== undefined) {
    if (upside ? current >= targetPrice : current <= targetPrice) return "reached";
    // 손절은 목표의 반대쪽이다 — 상방 예측이면 아래로, 하방 예측이면 위로 뚫린다.
    if (stopPrice !== undefined && (upside ? current <= stopPrice : current >= stopPrice)) {
      return "stop_hit";
    }
  }

  const { toTargetPct, toStopPct } = input;
  if (toStopPct !== undefined && Math.abs(toStopPct) <= NEAR_PCT) return "near";
  if (toTargetPct !== undefined && Math.abs(toTargetPct) <= NEAR_PCT) return "near";
  return "live";
}

/** 등록가 → 목표 구간의 진척률(%). 음수는 반대 방향. */
function deriveProgress(base?: number, current?: number, target?: number): number | undefined {
  if (base === undefined || current === undefined || target === undefined) return undefined;
  const span = target - base;
  if (span === 0) return undefined;
  return ((current - base) / span) * 100;
}

/** 기한 있는 예측 — 스탬프 응답에서. */
export function fromStamp(stamp: Stamp): Prediction {
  const base = positive(stamp.price);
  const current = positive(stamp.current?.price);
  const target = stamp.targetPrice;
  // 스탬프의 관점(stance)은 서버가 등록가 기준으로 굳혀 둔 값이라 그대로 따른다.
  const upside = stamp.stance !== "bearish";

  const toTargetPct = current !== undefined && target > 0 ? pct(current, target) : undefined;
  const scoreThen = stamp.score;
  const scoreNow = stamp.current?.score;

  return {
    key: `s-${stamp.id}`,
    source: "stamp",
    id: stamp.id,
    symbol: stamp.symbol,
    symbolName: stamp.symbolName || stamp.symbol,
    currency: stamp.currency === "USD" ? "USD" : "KRW",
    createdAt: stamp.stampedAt,
    basePrice: base,
    baseDate: stamp.tradeDate,
    currentPrice: current,
    targetPrice: target,
    upside,
    horizonKind: stamp.horizonKind,
    dueAt: stamp.dueAt,
    conviction: stamp.conviction,
    note: stamp.note,

    expectedReturn: stamp.expectedReturn,
    sinceBasePct: stamp.current?.changeRate,
    toTargetPct,
    // 스탬프 진행률은 서버가 이미 같은 정의로 계산해 준다 — 다시 세지 않는다.
    progress: stamp.targetProgress ?? deriveProgress(base, current, target),
    reachedDate: stamp.targetHit?.date,
    peakReturn: stamp.peakReturn,
    troughReturn: stamp.troughReturn,
    daysLeft: stamp.dueAt ? daysUntil(stamp.dueAt) : undefined,

    verdict: stamp.verdict,
    due: stamp.due,
    opened: stamp.opened,
    horizonReturn: stamp.horizon?.return,
    benchmarkId: stamp.horizon?.benchmarkId ?? stamp.benchmarkId,
    alpha: stamp.horizon?.alpha,
    targetAccuracy: stamp.targetAccuracy,

    scoreThen,
    scoreNow,
    gradeThen: stamp.grade,
    gradeNow: stamp.current?.grade,
    scoreShift:
      scoreThen !== undefined && scoreNow !== undefined ? scoreNow - scoreThen : undefined,
    breakdownThen: stamp.snapshot?.breakdown,
    breakdownNow: stamp.current?.breakdown,
    held: stamp.snapshot?.held,
    holdingAvgPrice: stamp.snapshot?.holdingAvgPrice,
    watchlisted: stamp.snapshot?.watchlisted,

    status: deriveStatus({
      source: "stamp",
      verdict: stamp.verdict,
      due: stamp.due,
      opened: stamp.opened,
      reachedDate: stamp.targetHit?.date,
      upside,
      current,
      targetPrice: target,
      toTargetPct,
    }),
    stamp,
  };
}

/** 기한 없는 예측 — 목표가 응답에서. 시세는 별도 배치로 받아 붙인다. */
export function fromTarget(target: PriceTarget, quote?: QuoteSummary): Prediction {
  const base = positive(target.basePrice);
  const current = positive(quote?.currentPrice);
  const reference = base ?? current;
  // 등록가가 없는 옛 행은 현재가로 방향을 판정한다. 둘 다 없으면 상방으로 본다.
  const upside = reference === undefined ? true : target.targetPrice >= reference;

  const toTargetPct = current !== undefined ? pct(current, target.targetPrice) : undefined;
  const toTarget2Pct =
    current !== undefined && target.targetPrice2 ? pct(current, target.targetPrice2) : undefined;
  const toStopPct =
    current !== undefined && target.stopPrice ? pct(current, target.stopPrice) : undefined;

  let riskReward: number | undefined;
  if (current !== undefined && target.stopPrice && target.stopPrice > 0) {
    const risk = Math.abs(current - target.stopPrice);
    if (risk > 0) riskReward = Math.abs(target.targetPrice - current) / risk;
  }

  return {
    key: `t-${target.id}`,
    source: "target",
    id: Number(target.id),
    symbol: target.symbol,
    symbolName: quote?.name || target.symbol,
    currency: target.currency,
    createdAt: target.createdAt,
    basePrice: base,
    baseDate: target.baseDate,
    currentPrice: current,
    targetPrice: target.targetPrice,
    targetPrice2: target.targetPrice2,
    stopPrice: target.stopPrice,
    upside,
    horizonKind: "none",
    note: target.note,

    expectedReturn: base !== undefined ? pct(base, target.targetPrice) : undefined,
    sinceBasePct: base !== undefined && current !== undefined ? pct(base, current) : undefined,
    toTargetPct,
    toTarget2Pct,
    toStopPct,
    progress: deriveProgress(base, current, target.targetPrice),
    riskReward,
    reachedDate: target.reachedDate,
    // 목표가는 최고·최저를 **가격**으로 준다 — 등록가가 있어야 %로 옮길 수 있다.
    peakReturn:
      base !== undefined && target.peakPrice ? pct(base, target.peakPrice) : undefined,
    troughReturn:
      base !== undefined && target.troughPrice ? pct(base, target.troughPrice) : undefined,

    status: deriveStatus({
      source: "target",
      reachedDate: target.reachedDate,
      upside,
      current,
      targetPrice: target.targetPrice,
      stopPrice: target.stopPrice,
      toTargetPct,
      toStopPct,
    }),
    target,
  };
}

// ─────────────────────────── 집계 ───────────────────────────

export type PredictionSummary = {
  total: number;
  symbols: number;
  dated: number;
  undated: number;
  dueCount: number;
  /** 채점이 끝난 건수 (기한 있는 것만). */
  judged: number;
  hits: number;
  hitRate?: number;
  avgAlpha?: number;
  avgTargetError?: number;
  /** 무기한 예측 중 목표에 닿은 것. 적중률과 **섞지 않는다**. */
  undatedReached: number;
  undatedTracking: number;
  stopHits: number;
  nearCount: number;
};

function mean(values: number[]): number | undefined {
  if (values.length === 0) return undefined;
  return values.reduce((a, b) => a + b, 0) / values.length;
}

/**
 * 요약.
 *
 * **적중률과 도달률을 합치지 않는다.** 무기한 예측은 기다리면 언젠가 닿을 수 있어
 * 같은 분모에 넣으면 숫자가 부풀어 오른다. 기한 있는 것은 「적중률」, 기한 없는
 * 것은 「도달률」로 나란히 둔다.
 */
export function summarizePredictions(items: Prediction[]): PredictionSummary {
  const dated = items.filter((p) => p.horizonKind !== "none");
  const undated = items.filter((p) => p.horizonKind === "none");
  const judged = dated.filter((p) => p.verdict === "hit" || p.verdict === "miss");
  const hits = judged.filter((p) => p.verdict === "hit");

  return {
    total: items.length,
    symbols: new Set(items.map((p) => p.symbol)).size,
    dated: dated.length,
    undated: undated.length,
    dueCount: items.filter((p) => p.status === "due").length,
    judged: judged.length,
    hits: hits.length,
    hitRate: judged.length === 0 ? undefined : (hits.length / judged.length) * 100,
    avgAlpha: mean(
      judged.map((p) => p.alpha).filter((v): v is number => typeof v === "number"),
    ),
    avgTargetError: mean(
      judged.map((p) => p.targetAccuracy).filter((v): v is number => typeof v === "number"),
    ),
    undatedReached: undated.filter((p) => p.status === "reached").length,
    undatedTracking: undated.filter((p) => p.status !== "reached").length,
    stopHits: items.filter((p) => p.status === "stop_hit").length,
    nearCount: items.filter((p) => p.status === "near").length,
  };
}

// ─────────────────────────── 필터·정렬 ───────────────────────────

export type PredictionStatusFilter = "all" | "due" | "open" | "reached" | "hit" | "miss";
export type PredictionHorizonFilter = "all" | "dated" | "undated";
export type PredictionSort = "urgent" | "recent" | "due" | "change";

export const STATUS_FILTERS: { id: PredictionStatusFilter; label: string }[] = [
  { id: "all", label: "전체" },
  { id: "due", label: "개봉 대기" },
  { id: "open", label: "진행 중" },
  { id: "reached", label: "도달·손절" },
  { id: "hit", label: "적중" },
  { id: "miss", label: "빗나감" },
];

export const HORIZON_FILTERS: { id: PredictionHorizonFilter; label: string }[] = [
  { id: "all", label: "전체 기한" },
  { id: "dated", label: "기한 있음" },
  { id: "undated", label: "무기한" },
];

export const SORTS: { id: PredictionSort; label: string }[] = [
  { id: "urgent", label: "급한 순" },
  { id: "due", label: "만기 순" },
  { id: "recent", label: "최신 순" },
  { id: "change", label: "등락 순" },
];

export function matchesStatusFilter(p: Prediction, filter: PredictionStatusFilter): boolean {
  switch (filter) {
    case "all":
      return true;
    case "due":
      return p.status === "due";
    case "open":
      return p.status === "live" || p.status === "near";
    case "reached":
      return p.status === "reached" || p.status === "stop_hit";
    case "hit":
      return p.verdict === "hit";
    case "miss":
      return p.verdict === "miss";
  }
}

/**
 * 「급한 순」의 정의.
 *
 * 사용자가 이 화면에서 묻는 것은 「오늘 뭘 봐야 하나」다. 그 답은 ① 열어야 할 것,
 * ② 손절이 뚫린 것, ③ 목표에 닿은 것, ④ 곧 갈릴 것, ⑤ 나머지 순이다. 같은 등급
 * 안에서는 만기가 가까운 쪽이 먼저다.
 */
const URGENCY: Record<PredictionStatus, number> = {
  due: 0,
  stop_hit: 1,
  reached: 2,
  near: 3,
  live: 4,
  hit: 5,
  miss: 5,
};

export function sortPredictions(items: Prediction[], sort: PredictionSort): Prediction[] {
  const out = [...items];
  const time = (iso?: string) => (iso ? new Date(iso).getTime() : Number.NaN);

  switch (sort) {
    case "recent":
      out.sort((a, b) => time(b.createdAt) - time(a.createdAt));
      break;
    case "due":
      // 기한 없는 예측은 만기가 없다 — 맨 뒤로. 앞에 두면 "가장 급하다"고 말하는 셈이다.
      out.sort((a, b) => {
        const da = a.dueAt ? time(a.dueAt) : Number.POSITIVE_INFINITY;
        const db = b.dueAt ? time(b.dueAt) : Number.POSITIVE_INFINITY;
        return da - db;
      });
      break;
    case "change":
      out.sort((a, b) => (b.sinceBasePct ?? -Infinity) - (a.sinceBasePct ?? -Infinity));
      break;
    default:
      out.sort((a, b) => {
        const ua = URGENCY[a.status];
        const ub = URGENCY[b.status];
        if (ua !== ub) return ua - ub;
        const da = a.dueAt ? time(a.dueAt) : Number.POSITIVE_INFINITY;
        const db = b.dueAt ? time(b.dueAt) : Number.POSITIVE_INFINITY;
        if (da !== db) return da - db;
        return time(b.createdAt) - time(a.createdAt);
      });
  }
  return out;
}

// ─────────────────────────── 습관 통계 ───────────────────────────

export type HabitBucket = { label: string; judged: number; hits: number };

/**
 * 확신도·기한·방향별 적중률.
 *
 * **표본이 적으면 적다고 말한다.** 화면 쪽이 3건 미만 칸을 점선으로 그리고 결론
 * 문장을 쓰지 않도록, 여기서는 판정 건수를 그대로 넘긴다 — 비율만 넘기면 1/1이
 * 100%로 보여 「확신 5는 항상 맞는다」가 된다.
 */
export function habitStats(items: Prediction[]): {
  conviction: HabitBucket[];
  horizon: HabitBucket[];
  direction: HabitBucket[];
  /** 상방 예측이 예상한 폭과 실제 폭의 차이(%p). 양수면 목표를 높게 잡는 버릇. */
  targetBiasPp?: number;
  biasSamples: number;
} {
  const judged = items.filter((p) => p.verdict === "hit" || p.verdict === "miss");

  const bucket = (label: string, pool: Prediction[]): HabitBucket => ({
    label,
    judged: pool.length,
    hits: pool.filter((p) => p.verdict === "hit").length,
  });

  const conviction = [5, 4, 3, 2, 1].map((v) =>
    bucket(`확신 ${v}`, judged.filter((p) => p.conviction === v)),
  );

  const horizonLabels: { id: PredictionHorizon; label: string }[] = [
    { id: "session_close", label: "당일" },
    { id: "next_session_close", label: "다음 장" },
    { id: "1w", label: "1주" },
    { id: "1m", label: "1개월" },
    { id: "3m", label: "3개월" },
    { id: "custom", label: "직접" },
  ];
  const horizon = horizonLabels
    .map(({ id, label }) => bucket(label, judged.filter((p) => p.horizonKind === id)))
    .filter((b) => b.judged > 0);

  const direction = [
    bucket("상방", judged.filter((p) => p.upside)),
    bucket("하방", judged.filter((p) => !p.upside)),
  ];

  // 목표가 버릇 — 상방 예측만 본다. 상·하방을 섞으면 부호가 상쇄돼 편향이 사라진다.
  const biasPool = judged.filter(
    (p) =>
      p.upside &&
      typeof p.expectedReturn === "number" &&
      typeof p.horizonReturn === "number",
  );
  const targetBiasPp = mean(
    biasPool.map((p) => (p.expectedReturn as number) - (p.horizonReturn as number)),
  );

  return { conviction, horizon, direction, targetBiasPp, biasSamples: biasPool.length };
}
