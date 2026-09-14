import { daysUntil, type UpcomingEvent } from "@/hooks/use-upcoming-events";
import type { PriceTarget } from "@/lib/price-targets";
import { formatPercent } from "@/lib/format";
import type { WatchlistRow } from "@/lib/watchlist-calc";

/**
 * 관심종목 한 줄이 **오늘 말할 것이 있는가** — 그 판정을 한 곳에 모은다
 * (2026-09-07 관심종목 전면 개편 시안).
 *
 * # 왜 이게 필요한가
 *
 * 지금 화면은 종목마다 전일·전주·전월 세 칸을 인쇄한다. 열두 종목이면 숫자 서른여섯
 * 개인데, 그중 사용자가 실제로 **무언가 하게 되는** 것은 보통 한둘이다. 나머지는 읽고
 * 넘기는 비용만 남긴다 — 화면이 "다 보여줬으니 네가 골라라"라고 말하는 셈이다.
 *
 * 그래서 값을 더 인쇄하는 대신 **판정을 화면이 대신한다.** 이미 화면에 있는 값(현재가·
 * 52주·담은 뒤·점수)과 이미 부르고 있는 것(예정 일정), 그리고 사용자가 직접 남긴 것
 * (목표가)에서 "지금 말할 거리"만 뽑아 칩으로 세운다. **새 API는 하나도 없다.**
 *
 * # 규칙
 *
 * - 근거 없는 신호를 만들지 않는다. 값이 없으면 그 신호는 그냥 없다 — "데이터 없음"
 *   칩을 세우면 조용한 종목이 제일 시끄러워진다.
 * - 가중치(`weight`)는 **정렬과 분류에만** 쓴다. 화면에 숫자로 찍지 않는다. 이건
 *   점수가 아니라 "먼저 보여줄 순서"다.
 * - 매매 신호가 아니다. 「목표가 도달」은 사용자가 **자기가 적어 둔 값**에 닿았다는
 *   사실 보고이고, 팔라는 말이 아니다.
 */

/**
 * 칩의 색이 곧 뜻이다. 이 앱에서 **빨강은 상승**이라(한국식) 위험을 빨강으로 칠하면
 * 손절가 이탈이 급등처럼 읽힌다. 그래서 색을 세 갈래로 나눈다 —
 *
 *  - `up`·`down`: **가격의 방향**을 말하는 칩만. 등락률 칸과 같은 색 규칙을 쓴다.
 *  - `mine`·`stop`: **내가 그어 둔 선**(목표가·손절가). 시장이 아니라 내 판단이라
 *    브랜드색과 상태색을 쓴다 — 방향색을 빌려 오지 않는다.
 *  - `date`·`note`: 날짜가 잡힌 일과 그 밖의 배경 설명. 색으로 다투지 않는다.
 */
export type SignalTone = "up" | "down" | "mine" | "stop" | "date" | "note";

export type WatchSignal = {
  id: string;
  tone: SignalTone;
  /** 칩에 찍히는 짧은 말. */
  label: string;
  /** 왜 떴는지 한 줄 — 칩의 `title`. 칩이 짧아서 근거는 여기 담는다. */
  detail: string;
  /** 정렬·분류용 무게. 화면에 찍지 않는다. */
  weight: number;
};

/** 목표가·손절가에 「근접」이라 부를 거리(%). */
const NEAR_PCT = 5;
/** 일정을 알리는 창(일). 두 달 뒤 실적은 오늘 이 화면에서 할 일이 없다. */
const EVENT_DAYS = 14;
/** 「오늘 크게 움직였다」의 문턱(%). */
const MOVE_PCT = 3;
/** 52주 레인지의 끝자리로 치는 폭(%p). */
const EDGE_PCT = 5;
/** 담은 뒤 이만큼 벌어지면 한 번 들여다볼 값이 된다(%). */
const DRIFT_PCT = 10;
/** 이만큼 오래 담아 두고도 제자리면 「정리할지 정할 때」다. */
const STALE_DAYS = 120;
const STALE_BAND = 3;

function pctAway(from: number, to: number): number {
  return ((to - from) / from) * 100;
}

export function buildWatchSignals(input: {
  row: WatchlistRow;
  /** 이 종목의 예정 일정. 목록 화면이 배치로 한 번 받아 나눠 준다. */
  events?: UpcomingEvent[];
  /** 사용자가 이 종목에 적어 둔 목표가. */
  target?: PriceTarget | null;
}): WatchSignal[] {
  const { row, events = [], target } = input;
  const quote = row.quote;
  const price = quote?.currentPrice ?? 0;
  const out: WatchSignal[] = [];

  // ── 사용자가 스스로 그은 선이 먼저다. 시장이 만든 신호보다 무겁게 둔다 —
  //    본인이 "여기 오면 보겠다"고 적어 둔 값이기 때문이다.
  if (price > 0 && target) {
    if (target.stopPrice && target.stopPrice > 0) {
      const away = pctAway(target.stopPrice, price);
      if (away <= 0) {
        out.push({
          id: "stop-hit",
          tone: "stop",
          label: "손절가 이탈",
          detail: `내가 적어 둔 손절가 아래입니다 (${formatPercent(away)})`,
          weight: 46,
        });
      } else if (away <= NEAR_PCT) {
        out.push({
          id: "stop-near",
          tone: "stop",
          label: `손절가 ${away.toFixed(1)}% 위`,
          detail: "내가 적어 둔 손절가에 근접했습니다",
          weight: 30,
        });
      }
    }
    if (target.targetPrice > 0) {
      const away = pctAway(price, target.targetPrice);
      if (away <= 0) {
        out.push({
          id: "target-reached",
          tone: "mine",
          label: "목표가 도달",
          detail: "내가 적어 둔 목표가에 닿았습니다 — 계획을 다시 볼 때",
          weight: 44,
        });
      } else if (away <= NEAR_PCT) {
        out.push({
          id: "target-near",
          tone: "mine",
          label: `목표가 ${away.toFixed(1)}% 남음`,
          detail: "내가 적어 둔 목표가에 근접했습니다",
          weight: 28,
        });
      }
    }
  }

  // ── 날짜가 박힌 일. 가까울수록 무겁다.
  const next = events.find((e) => daysUntil(e.date) != null);
  const days = next ? daysUntil(next.date) : null;
  if (next && days != null && days <= EVENT_DAYS) {
    out.push({
      id: "event",
      tone: "date",
      label: `${next.label?.trim() || "실적"} D-${days}`,
      detail: `${next.date} · 발표 전에 볼 것을 정해 둘 때`,
      weight: 34 - days,
    });
  }

  // ── 오늘의 움직임. 등락률 칸은 어차피 화면에 있으므로 **문턱을 넘은 것만** 말한다.
  if (quote && price > 0 && Math.abs(quote.changeRate) >= MOVE_PCT) {
    const up = quote.changeRate > 0;
    out.push({
      id: "move",
      tone: up ? "up" : "down",
      label: `오늘 ${formatPercent(quote.changeRate)}`,
      detail: up ? "오늘 크게 올랐습니다" : "오늘 크게 내렸습니다",
      weight: Math.min(26, Math.round(Math.abs(quote.changeRate) * 4)),
    });
  }

  // ── 52주 레인지의 끝. 막대는 이미 행에 있지만, 끝에 닿은 종목은 **골라 낼 값**이다.
  if (row.rangePct != null) {
    if (row.rangePct >= 100 - EDGE_PCT) {
      out.push({
        id: "high52",
        tone: "up",
        label: "52주 고가권",
        detail: `52주 레인지 상단 ${Math.round(row.rangePct)}%`,
        weight: 20,
      });
    } else if (row.rangePct <= EDGE_PCT) {
      out.push({
        id: "low52",
        tone: "down",
        label: "52주 저가권",
        detail: `52주 레인지 상단 ${Math.round(row.rangePct)}%`,
        weight: 20,
      });
    }
  }

  // ── 담은 뒤. 이 화면의 존재 이유라, 크게 벌어졌으면 말한다.
  if (row.sinceAddedRate != null && Math.abs(row.sinceAddedRate) >= DRIFT_PCT) {
    const up = row.sinceAddedRate > 0;
    out.push({
      id: "since",
      tone: up ? "up" : "down",
      label: `담은 뒤 ${formatPercent(row.sinceAddedRate)}`,
      detail: up
        ? "담았을 때보다 많이 올랐습니다 — 지금도 같은 이유로 볼 종목인지"
        : "담았을 때보다 많이 내렸습니다 — 담은 이유가 아직 유효한지",
      weight: 18,
    });
  }

  // ── 점수는 하루 한 번 바뀐다. 양 끝만 말한다.
  if (row.score != null) {
    if (row.score >= 80) {
      out.push({
        id: "score-high",
        tone: "note",
        label: `점수 ${Math.round(row.score)}`,
        detail: "5축 종합 점수 상위권입니다 — 매매 신호가 아닙니다",
        weight: 10,
      });
    } else if (row.score <= 35) {
      out.push({
        id: "score-low",
        tone: "note",
        label: `점수 ${Math.round(row.score)}`,
        detail: "5축 종합 점수 하위권입니다 — 매매 신호가 아닙니다",
        weight: 12,
      });
    }
  }

  // ── 오래 담아 두고 아무 일도 없던 종목. 목록이 길어지는 진짜 원인이고,
  //    다른 어떤 신호도 없을 때만 말한다.
  if (out.length === 0 && row.addedAt && row.sinceAddedRate != null) {
    const held = holdDays(row.addedAt);
    if (held != null && held >= STALE_DAYS && Math.abs(row.sinceAddedRate) < STALE_BAND) {
      out.push({
        id: "stale",
        tone: "note",
        label: `${held}일째 제자리`,
        detail: "담은 뒤 거의 움직이지 않았습니다 — 계속 볼 종목인지 정할 때",
        weight: 8,
      });
    }
  }

  return out.sort((a, b) => b.weight - a.weight);
}

export function holdDays(addedAt: string): number | null {
  const t = new Date(addedAt).getTime();
  if (!Number.isFinite(t)) return null;
  return Math.max(0, Math.floor((Date.now() - t) / 86_400_000));
}

/** 정렬용 한 숫자. 가장 무거운 신호가 지배하고 나머지는 조금씩 보탠다. */
export function attentionScore(signals: WatchSignal[]): number {
  if (signals.length === 0) return 0;
  const [top, ...rest] = signals;
  return top.weight + rest.reduce((sum, s) => sum + s.weight * 0.25, 0);
}

export type WatchBucket = "action" | "watch" | "quiet";

/**
 * 세 칸으로 나눈다 — 「지금 볼 것」 / 「움직인 것」 / 「조용한 것」.
 *
 * 문턱을 신호 **개수**가 아니라 가장 무거운 신호로 잡는 이유: 약한 신호 셋(점수 낮음 +
 * 52주 저가권 + 담은 뒤 -11%)이 손절가 이탈 하나를 이기면 안 된다. 그건 같은 사실을
 * 세 번 말한 것에 가깝다.
 */
export function watchBucket(signals: WatchSignal[]): WatchBucket {
  const top = signals[0]?.weight ?? 0;
  if (top >= 26) return "action";
  if (top >= 10) return "watch";
  return "quiet";
}

export const BUCKET_LABEL: Record<WatchBucket, string> = {
  action: "지금 볼 것",
  watch: "움직인 것",
  quiet: "조용한 것",
};

export const BUCKET_HINT: Record<WatchBucket, string> = {
  action: "날짜가 잡혔거나, 내가 그어 둔 선에 닿았습니다",
  watch: "오늘 눈에 띄게 움직였거나 레인지 끝에 있습니다",
  quiet: "따로 말할 거리가 없습니다",
};
