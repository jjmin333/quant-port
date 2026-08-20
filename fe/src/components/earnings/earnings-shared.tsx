"use client";

import type {
  EarningsDateStatus,
  EarningsEvent,
  EarningsReaction,
  EarningsSession,
  EarningsSurprise,
} from "@/lib/earnings-types";
import { formatFinancialAmount } from "@/lib/format";
import { formatShortDate } from "@/lib/earnings-range";
import { cn } from "@/lib/utils";

/**
 * 실적 화면의 공통 표기 규칙.
 *
 * 여기 모아 둔 이유는 **세 상태(확정·예정·추정)를 한 곳에서만 결정해야** 하기
 * 때문이다. 캘린더 본문·레일·종목 탭이 각자 배지를 그리면, 우리가 역산한 추정
 * 날짜가 어떤 화면에서는 회사가 공시한 날짜와 똑같이 보이게 된다 — 이 기능에서
 * 유일하게 하면 안 되는 일이다.
 */

/** 발표일 상태별 표기. `estimated`는 반드시 다른 모양이어야 한다. */
export const DATE_STATUS_META: Record<
  EarningsDateStatus,
  { label: string; className: string; title: string }
> = {
  confirmed: {
    label: "발표",
    className: "bg-muted text-foreground",
    title: "이미 발표된 실적입니다",
  },
  scheduled: {
    label: "예정",
    className: "border border-dashed border-border bg-transparent text-muted-foreground",
    title: "데이터 제공처가 알려 준 발표 예정일입니다",
  },
  // 점선 + 보라 계열. 빨강·파랑은 이 앱에서 상승·하락이라 쓸 수 없고, 회색 점선은
  // `scheduled`가 이미 쓰고 있어 둘이 구별되지 않는다.
  estimated: {
    label: "추정",
    className:
      "border border-dashed border-[var(--sample)]/60 bg-[var(--sample)]/10 text-[var(--sample)]",
    title:
      "작년 같은 분기의 발표일로 역산한 추정일입니다. 국내는 실적발표 예정일을 제공하는 소스가 없습니다.",
  },
};

export function DateStatusBadge({ status }: { status: EarningsDateStatus }) {
  const meta = DATE_STATUS_META[status];
  return (
    <span
      title={meta.title}
      className={cn(
        "shrink-0 rounded-full px-1.5 py-0.5 text-[10px] font-medium leading-none",
        meta.className,
      )}
    >
      {meta.label}
    </span>
  );
}

const SESSION_LABEL: Record<EarningsSession, string> = {
  bmo: "장전",
  amc: "장후",
  dmh: "장중",
};

/** 장전·장후 표기. 미국만 오고, 미공개(빈 값)면 아무것도 그리지 않는다. */
export function SessionBadge({ session }: { session?: EarningsSession }) {
  if (!session) return null;
  return (
    <span className="shrink-0 rounded bg-muted px-1 py-0.5 text-[10px] leading-none text-muted-foreground">
      {SESSION_LABEL[session]}
    </span>
  );
}

/** "2026 2Q · 연결" — 한 발표가 어느 기간의 것인지. */
export function periodLabel(event: EarningsEvent): string {
  const period = event.fiscalPeriod === "FY" ? "연간" : event.fiscalPeriod;
  return event.basis ? `${event.fiscalYear} ${period} · ${event.basis}` : `${event.fiscalYear} ${period}`;
}

export const SURPRISE_METRIC_LABEL: Record<EarningsSurprise["metric"], string> = {
  opIncome: "영업이익",
  eps: "EPS",
  revenue: "매출",
};

/**
 * 서프라이즈 한 조각: `+5.5%` 또는 `흑자전환`.
 *
 * **`pct`가 없을 때 빈칸으로 두지 않는다.** 기준치가 0 이하라 퍼센트를 만들 수
 * 없는 경우인데, `—`로 그리면 데이터가 없는 것처럼 보인다. 서버가 준 라벨이
 * 그 자리에서 더 정확한 답이다.
 */
export function SurpriseValue({
  surprise,
  className,
}: {
  surprise?: EarningsSurprise;
  className?: string;
}) {
  if (!surprise) {
    return <span className={cn("text-muted-foreground", className)}>—</span>;
  }
  if (surprise.pct == null) {
    return (
      <span
        className={cn("font-medium text-[var(--text-secondary)]", className)}
        title={`${SURPRISE_METRIC_LABEL[surprise.metric]} 기준치가 0 이하라 증감률을 낼 수 없습니다`}
      >
        {surprise.label || "—"}
      </span>
    );
  }
  const up = surprise.pct >= 0;
  return (
    <span
      // 상승=빨강. 이 앱 전체 규칙이라 여기서 뒤집으면 같은 색이 화면마다 다른
      // 뜻을 갖게 된다.
      className={cn(
        "font-mono font-semibold tabular-nums",
        up ? "text-[var(--profit)]" : "text-[var(--loss)]",
        className,
      )}
      title={`${SURPRISE_METRIC_LABEL[surprise.metric]} 컨센서스 대비`}
    >
      {up ? "+" : ""}
      {surprise.pct.toFixed(1)}%
      {surprise.label ? <span className="ml-1 text-[10px] font-normal">{surprise.label}</span> : null}
    </span>
  );
}

/**
 * 「기대 84.8조 → 89.4조」 한 줄.
 *
 * EPS는 통화 축약(조·억)이 말이 안 되므로 소수점으로 적는다 — `formatFinancialAmount`에
 * EPS를 넣으면 `$3`이 된다.
 */
export function EstimateActualPair({
  surprise,
  currency,
}: {
  surprise: EarningsSurprise;
  currency: string;
}) {
  const render = (v: number) =>
    surprise.metric === "eps"
      ? `${currency === "USD" ? "$" : ""}${v.toFixed(2)}`
      : formatFinancialAmount(v, currency === "USD" ? "USD" : "KRW");
  return (
    <span className="font-mono text-[11px] tabular-nums text-muted-foreground">
      기대 {render(surprise.estimate)} → {render(surprise.actual)}
    </span>
  );
}

/** 종목 로고 옆에 붙는 이름. 이름이 아직 없으면 심볼이 자리를 지킨다. */
export function eventName(event: EarningsEvent): string {
  return event.name || event.symbol;
}

/**
 * 발표 후 첫 세션의 주가 반응 — `↗ +13.4%`.
 *
 * **없을 때 `0%`가 아니라 「집계 중」이다.** 인접 거래일 종가가 없으면 서버가
 * 값을 내지 않는데(최근 세션 종가 커버리지가 얇다), 0%로 그리면 「시장이
 * 반응하지 않았다」는 전혀 다른 말이 된다.
 */
export function ReactionValue({
  reaction,
  status,
  className,
}: {
  reaction?: EarningsReaction;
  status: EarningsDateStatus;
  className?: string;
}) {
  // 예정·추정 발표에는 반응이 있을 수 없다. 「집계 중」이라고 적으면 곧 나온다는
  // 뜻이 되므로 그냥 비운다.
  if (status !== "confirmed") {
    return <span className={cn("text-muted-foreground", className)}>·</span>;
  }
  if (!reaction) {
    return (
      <span
        className={cn("text-[10px] text-muted-foreground", className)}
        title="발표 직후 거래일의 종가가 아직 없습니다"
      >
        집계 중
      </span>
    );
  }
  const up = reaction.pct >= 0;
  return (
    <span
      className={cn(
        "font-mono font-semibold tabular-nums",
        up ? "text-[var(--profit)]" : "text-[var(--loss)]",
        className,
      )}
      title={reactionTitle(reaction)}
    >
      {up ? "↗" : "↘"} {up ? "+" : ""}
      {reaction.pct.toFixed(1)}%
    </span>
  );
}

function reactionTitle(reaction: EarningsReaction): string {
  const when = reaction.basis === "sameDay" ? "발표 당일(장전 발표)" : "다음 거래일";
  return `${when} 종가 기준 · ${formatShortDate(reaction.baseDate)} → ${formatShortDate(reaction.date)}`;
}

/**
 * 서프라이즈와 주가가 **엇갈렸는지**.
 *
 * 이 화면에서 가장 값어치 있는 신호다 — 실측: 한국금융지주는 컨센서스를 +27.8%
 * 넘겼는데 다음 거래일 −7.3%였고, 엘앤에프는 −58.2% 하회했는데 +7.3%였다.
 * 숫자와 시장이 다르게 말한 경우이고, 두 열을 나란히 두지 않으면 알 수 없다.
 */
export function isDivergent(event: EarningsEvent): boolean {
  const pct = event.surprise?.pct;
  const reaction = event.reaction?.pct;
  if (pct == null || reaction == null) return false;
  // 문턱을 두는 이유: ±1% 서프라이즈에 ∓0.3% 반응은 엇갈린 게 아니라 그냥 잡음이다.
  if (Math.abs(pct) < 5 || Math.abs(reaction) < 2) return false;
  return pct > 0 !== reaction > 0;
}

export function DivergenceFlag({ event }: { event: EarningsEvent }) {
  if (!isDivergent(event)) return null;
  const beat = (event.surprise?.pct ?? 0) > 0;
  return (
    <span
      /*
       * 중립 솔리드 칩. 빨강·파랑은 상승·하락이고 보라는 예시 데이터라, 남은
       * 자리가 없다 — 새 색 토큰을 만들지 않고 **반전 대비**로 눈에 띄게 한다.
       * 이건 시세 신호가 아니라 주석이므로 색보다 대비가 맞다.
       */
      className="shrink-0 rounded bg-foreground px-1 py-0.5 text-[10px] font-medium leading-none text-background"
      title={
        beat
          ? "컨센서스를 넘겼는데 주가는 내렸습니다 — 시장이 다른 것을 봤다는 뜻입니다"
          : "컨센서스를 밑돌았는데 주가는 올랐습니다"
      }
    >
      {beat ? "상회·하락" : "하회·상승"}
    </span>
  );
}

/**
 * React list key — API가 같은 종목·분기·날짜를 두 줄 줄 수 있다.
 *
 * FnGuide 적재 중 연결/개별이 같은 `basis`로 겹치거나, 같은 분기가 중복 행으로
 * 내려오면 `symbol-fiscalPeriod`만으로는 key가 충돌한다. 수치 앵커를 붙여 구분한다.
 */
export function earningsEventKey(event: EarningsEvent): string {
  const anchor =
    event.surprise?.actual ??
    event.opIncome.actual ??
    event.opIncome.estimate ??
    event.revenue.actual ??
    event.revenue.estimate ??
    event.eps.actual ??
    event.eps.estimate ??
    0;
  return [
    event.symbol,
    event.date,
    event.fiscalYear,
    event.fiscalPeriod,
    event.dateStatus,
    event.basis ?? "",
    event.market,
    anchor,
    event.revisedAt ?? "",
  ].join("|");
}
