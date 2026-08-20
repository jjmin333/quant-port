import type { PriceTarget } from "@/lib/price-targets";
import type { QuoteSummary } from "@/lib/quotes-types";

/**
 * 목표가 화면의 파생값 한 곳.
 *
 * 행·타일·필터·레일이 전부 "도달했나 / 임박인가 / 손절이 가까운가"를 물어본다.
 * 네 군데에서 따로 계산하면 타일은 「임박 2」라고 적는데 목록의 임박 배지는 세 개인
 * 일이 생긴다. 집계는 **이미 받아둔 배열에서** 센다 — 서버 재요청 0.
 */

/** 목표까지 이 % 이내면 임박으로 본다. */
export const NEAR_TARGET_PCT = 5;

export type TargetStatus = "reached" | "near" | "stop_near" | "open";

export type TargetView = {
  target: PriceTarget;
  quote?: QuoteSummary;
  /** 시세가 없으면 null. 없는 값을 0으로 그리면 "도달했다"는 거짓이 된다. */
  current: number | null;
  /** 목표가 현재가보다 위인가 — 등록가 기준으로 판정한 방향. */
  upside: boolean;
  /** 현재가에서 목표까지 남은 %. 방향과 무관한 절대 거리. */
  toTargetPct: number | null;
  toTarget2Pct: number | null;
  toStopPct: number | null;
  /** 등록 후 등락 %. 등록가가 없으면 null(예전에 만든 목표). */
  sinceBasePct: number | null;
  /** 등록가 → 목표 구간에서 현재가가 어디쯤인지, 0~1로 자른 값. */
  progress: number | null;
  /** 손익비(목표까지 / 손절까지). 손절이 없으면 null. */
  riskReward: number | null;
  status: TargetStatus;
};

function pct(from: number, to: number): number {
  return ((to - from) / from) * 100;
}

export function buildTargetView(
  target: PriceTarget,
  quote: QuoteSummary | undefined,
): TargetView {
  const current = quote?.currentPrice && quote.currentPrice > 0 ? quote.currentPrice : null;
  const base = target.basePrice && target.basePrice > 0 ? target.basePrice : null;

  // 목표가 등록가보다 위면 상방 목표다. 등록가가 없는 옛 행은 현재가로 대신
  // 판정한다 — 둘 다 없으면 상방으로 본다(대다수가 그렇다).
  const reference = base ?? current;
  const upside = reference === null ? true : target.targetPrice >= reference;

  const toTargetPct = current === null ? null : pct(current, target.targetPrice);
  const toTarget2Pct =
    current === null || !target.targetPrice2 ? null : pct(current, target.targetPrice2);
  const toStopPct = current === null || !target.stopPrice ? null : pct(current, target.stopPrice);
  const sinceBasePct = base === null || current === null ? null : pct(base, current);

  let progress: number | null = null;
  if (base !== null && current !== null && base !== target.targetPrice) {
    const raw = (current - base) / (target.targetPrice - base);
    progress = Math.max(0, Math.min(1, raw));
  }

  let riskReward: number | null = null;
  if (current !== null && target.stopPrice && target.stopPrice > 0) {
    const reward = Math.abs(target.targetPrice - current);
    const risk = Math.abs(current - target.stopPrice);
    if (risk > 0) riskReward = reward / risk;
  }

  const status = deriveStatus({ target, current, upside, toTargetPct, toStopPct });

  return {
    target,
    quote,
    current,
    upside,
    toTargetPct,
    toTarget2Pct,
    toStopPct,
    sinceBasePct,
    progress,
    riskReward,
    status,
  };
}

function deriveStatus({
  target,
  current,
  upside,
  toTargetPct,
  toStopPct,
}: {
  target: PriceTarget;
  current: number | null;
  upside: boolean;
  toTargetPct: number | null;
  toStopPct: number | null;
}): TargetStatus {
  /*
   * 서버가 일봉 고/저로 「닿았다」고 판정했으면 그게 먼저다. 종가만 보면 장중에
   * 닿았다가 되돌아온 목표가 영원히 미달로 남는다 — 스탬프의 조기 적중과 같은 이유.
   */
  if (target.reachedDate) return "reached";
  if (current !== null) {
    if (upside ? current >= target.targetPrice : current <= target.targetPrice) return "reached";
  }
  // 손절이 목표보다 가까우면 그쪽을 먼저 말한다. 「+21.9% 남음」만 적힌 행은
  // 손절선 5% 앞이라는 사실을 감춘다.
  if (toStopPct !== null && Math.abs(toStopPct) <= NEAR_TARGET_PCT) return "stop_near";
  if (toTargetPct !== null && Math.abs(toTargetPct) <= NEAR_TARGET_PCT) return "near";
  return "open";
}

export const TARGET_STATUS_LABEL: Record<TargetStatus, string> = {
  reached: "목표 도달",
  near: "도달 임박",
  stop_near: "손절 근접",
  open: "진행 중",
};

export type TargetSummary = {
  total: number;
  reached: number;
  near: number;
  stopNear: number;
  quoted: number;
  /** 미도달 행의 평균 남은 거리(%). 없으면 null. */
  avgToTarget: number | null;
  /** 등록가가 있는 행의 평균 등록 후 등락(%). */
  avgSinceBase: number | null;
};

export function summarizeTargets(views: TargetView[]): TargetSummary {
  const remaining = views
    .filter((v) => v.status !== "reached" && v.toTargetPct !== null)
    .map((v) => v.toTargetPct as number);
  const since = views
    .filter((v) => v.sinceBasePct !== null)
    .map((v) => v.sinceBasePct as number);

  const avg = (xs: number[]) => (xs.length === 0 ? null : xs.reduce((a, b) => a + b, 0) / xs.length);

  return {
    total: views.length,
    reached: views.filter((v) => v.status === "reached").length,
    near: views.filter((v) => v.status === "near").length,
    stopNear: views.filter((v) => v.status === "stop_near").length,
    quoted: views.filter((v) => v.current !== null).length,
    avgToTarget: avg(remaining),
    avgSinceBase: avg(since),
  };
}

export type TargetSort = "near" | "recent" | "change";

export const TARGET_SORTS: { id: TargetSort; label: string }[] = [
  { id: "near", label: "임박순" },
  { id: "recent", label: "등록순" },
  { id: "change", label: "등락순" },
];

export function sortTargetViews(views: TargetView[], sort: TargetSort): TargetView[] {
  const out = [...views];
  if (sort === "recent") {
    out.sort(
      (a, b) =>
        new Date(b.target.createdAt).getTime() - new Date(a.target.createdAt).getTime(),
    );
    return out;
  }
  if (sort === "change") {
    out.sort((a, b) => (b.sinceBasePct ?? -Infinity) - (a.sinceBasePct ?? -Infinity));
    return out;
  }
  // 임박순: 도달한 것을 위로, 그 다음 목표까지 가까운 순. 시세를 못 받은 행은
  // 거리를 모르므로 맨 뒤 — 위로 올리면 "가장 가깝다"고 말하는 셈이 된다.
  out.sort((a, b) => {
    const rank = (v: TargetView) => (v.status === "reached" ? 0 : 1);
    if (rank(a) !== rank(b)) return rank(a) - rank(b);
    const da = a.toTargetPct === null ? Infinity : Math.abs(a.toTargetPct);
    const db = b.toTargetPct === null ? Infinity : Math.abs(b.toTargetPct);
    return da - db;
  });
  return out;
}
